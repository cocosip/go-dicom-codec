package mqc

// MQ Arithmetic Decoder for JPEG 2000
// Reference: ISO/IEC 15444-1:2019 Annex C
// Based on the MQ-coder (multiplication-free, table-driven arithmetic coder)

// MQDecoder implements the MQ arithmetic decoder
type MQDecoder struct {
	// Input data
	data   []byte
	pos    int  // Current byte position
	lastByte byte // Last byte read (for 0xFF detection)

	// State variables
	a   uint32 // Probability interval
	c   uint32 // Code register
	ct  int    // Bit counter

	// Contexts
	contexts []uint8 // Context states (one per context)
}

// NewMQDecoder creates a new MQ decoder
func NewMQDecoder(data []byte, numContexts int) *MQDecoder {
	// Append 0xFF 0xFF sentinel marker to end of data
	// This acts as an artificial marker to stop bytein routines
	// Reference: OpenJPEG opj_mqc_init_dec_common
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc := &MQDecoder{
		data:     dataWithSentinel,
		pos:      0,
		lastByte: 0,
		a:        0x8000,
		c:        0,
		ct:       0,
		contexts: make([]uint8, numContexts),
	}

	// Initialize all contexts to state 0
	for i := range mqc.contexts {
		mqc.contexts[i] = 0
	}

	// Initialize decoder
	mqc.init()

	return mqc
}

// SetData updates the decoder's data buffer while preserving contexts
// This is used for lossy TERMALL mode where contexts should be maintained across passes
func (mqc *MQDecoder) SetData(data []byte) {
	// Append sentinel marker
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	// Update data and reset position
	mqc.data = dataWithSentinel
	mqc.pos = 0
	mqc.lastByte = 0

	// Reset decoder state registers but keep contexts
	mqc.a = 0x8000
	mqc.c = 0
	mqc.ct = 0

	// Reinitialize decoder with new data
	mqc.init()
}

// NewMQDecoderWithContexts creates a new MQ decoder with inherited contexts from previous decoder
// This is the correct approach for TERMALL mode: each pass gets a fresh decoder initialization
// but contexts can be preserved (lossy) or reset (lossless)
func NewMQDecoderWithContexts(data []byte, prevContexts []uint8) *MQDecoder {
	// Append sentinel marker
	dataWithSentinel := make([]byte, len(data)+2)
	copy(dataWithSentinel, data)
	dataWithSentinel[len(data)] = 0xFF
	dataWithSentinel[len(data)+1] = 0xFF

	mqc := &MQDecoder{
		data:     dataWithSentinel,
		pos:      0,
		lastByte: 0,
		a:        0x8000,
		c:        0,
		ct:       0,
		contexts: make([]uint8, len(prevContexts)),
	}

	// Copy contexts from previous decoder
	copy(mqc.contexts, prevContexts)

	// Initialize decoder
	mqc.init()

	return mqc
}

// GetContexts returns a copy of the current context states
// Used to preserve contexts across TERMALL pass boundaries
func (mqc *MQDecoder) GetContexts() []uint8 {
	contexts := make([]uint8, len(mqc.contexts))
	copy(contexts, mqc.contexts)
	return contexts
}

// init initializes the decoder
func (mqc *MQDecoder) init() {
	// Initialize C register (ISO/IEC 15444-1 C.3.4)
	// Load first byte into bits 23-16
	firstByte := byte(0xFF)
	if mqc.pos < len(mqc.data) {
		firstByte = mqc.data[mqc.pos]
		mqc.c = uint32(firstByte) << 16
		mqc.lastByte = firstByte
		mqc.pos++
	} else {
		mqc.c = 0xFF << 16
		mqc.lastByte = 0xFF
	}

	// Load second byte into bits 15-8
	// If first byte is 0xFF, we need special handling (bit stuffing)
	if firstByte == 0xFF {
		// Bit stuffing: only 7 bits available from second byte
		if mqc.pos < len(mqc.data) {
			secondByte := mqc.data[mqc.pos]
			// Check if this is a marker (> 0x8F) or bit-stuffed byte
			if secondByte > 0x8F {
				// Marker/sentinel: use 0xFF00
				mqc.c += 0xFF00
				mqc.ct = 8
				// Don't update lastByte or pos - marker not consumed
			} else {
				// Bit-stuffed: consume second byte, only 7 bits
				mqc.lastByte = secondByte
				mqc.pos++
				mqc.c += uint32(secondByte) << 9  // Shift by 9 (bit stuffing)
				mqc.ct = 7
			}
		} else {
			mqc.c += 0xFF00
			mqc.ct = 8
		}
	} else {
		// Normal case: use bytein for second byte
		mqc.bytein()
	}

	// Shift left by 7 to position for decoding
	// After this, bits 23-17 contain the first 7 bits of code
	mqc.c <<= 7
	mqc.ct -= 7
	mqc.a = 0x8000
}

// Decode decodes a single bit using the specified context
//
// Performance notes:
// - Hot path function - called millions of times during decoding
// - Table-driven (no branches in probability lookup)
// - Multiplication-free arithmetic (shifts and adds only)
// - Context state maintained in single byte (compact)
// - Typical performance: ~10-20ns per bit on modern CPUs
// - Optimization: inline candidate, branch prediction important
func (mqc *MQDecoder) Decode(contextID int) int {
	cx := &mqc.contexts[contextID]
	state := *cx & 0x7F  // Lower 7 bits = state
	mps := int(*cx >> 7) // Bit 7 = MPS (More Probable Symbol)

	// Get Qe value for this state
	qe := qeTable[state]

	// Calculate sub-interval
	mqc.a -= qe

	var d int // Decoded bit

	// Check if coded bit is LPS or MPS
	if (mqc.c >> 16) < qe {
		// LPS exchange (ISO/IEC 15444-1 C.3.2)
		// Note: C is NOT modified in this path (only in MPS path)

		// Conditional exchange
		if mqc.a < qe {
			// Exchange: MPS becomes the larger interval
			mqc.a = qe
			d = mps
			// Update state (MPS path)
			*cx = nmpsTable[state] | (uint8(mps) << 7)
		} else {
			// No exchange: decode LPS
			mqc.a = qe
			d = 1 - mps
			// Update state (LPS path)
			newState := nlpsTable[state]
			newMPS := mps
			if switchTable[state] == 1 {
				newMPS = 1 - mps
			}
			*cx = newState | (uint8(newMPS) << 7)
		}
		mqc.renormd()
	} else {
		// MPS path
		mqc.c -= qe << 16

		if mqc.a >= 0x8000 {
			// No renormalization needed
			d = mps
			return d
		}

		// MPS exchange (ISO/IEC 15444-1 C.3.2)
		if mqc.a < qe {
			// Exchange: LPS becomes the larger interval
			d = 1 - mps
			// Update state (LPS path)
			newState := nlpsTable[state]
			newMPS := mps
			if switchTable[state] == 1 {
				newMPS = 1 - mps
			}
			*cx = newState | (uint8(newMPS) << 7)
		} else {
			// No exchange: decode MPS
			d = mps
			// Update state (MPS path)
			*cx = nmpsTable[state] | (uint8(mps) << 7)
		}
		mqc.renormd()
	}

	return d
}

// renormd renormalizes the decoder (probability interval doubling)
func (mqc *MQDecoder) renormd() {
	for mqc.a < 0x8000 {
		if mqc.ct == 0 {
			mqc.bytein()
		}
		mqc.a <<= 1
		mqc.c <<= 1
		mqc.ct--
	}
}

// bytein reads the next byte from input stream
// Reference: OpenJPEG opj_mqc_bytein_macro
// Note: Data has 0xFF 0xFF sentinel appended, so we're guaranteed to have data
//
// OpenJPEG semantics: bp points to last read byte, reads *(bp+1) as next value
// Our semantics: pos points to next unread byte, lastByte holds last read byte
// Mapping: OpenJPEG *bp corresponds to our lastByte, *(bp+1) to data[pos]
func (mqc *MQDecoder) bytein() {
	// Bounds check (should never happen with sentinel, but safety first)
	if mqc.pos >= len(mqc.data) {
		mqc.c += 0xFF00
		mqc.ct = 8
		return
	}

	// OpenJPEG: l_c = *(bp + 1), check *bp for 0xFF
	// Our mapping: check lastByte, read data[pos]
	nextByte := mqc.data[mqc.pos]  // Corresponds to *(bp+1) = l_c

	// Handle 0xFF byte stuffing based on lastByte
	if mqc.lastByte == 0xFF {
		if nextByte > 0x8F {
			// Marker/sentinel: c += 0xff00, ct = 8, bp unchanged
			// Don't update lastByte or pos
			mqc.c += 0xFF00
			mqc.ct = 8
		} else {
			// Bit stuffing: c += l_c << 9, ct = 7, bp++
			mqc.lastByte = nextByte
			mqc.pos++
			mqc.c += uint32(nextByte) << 9
			mqc.ct = 7
		}
	} else {
		// Normal: c += l_c << 8, ct = 8, bp++
		mqc.lastByte = nextByte
		mqc.pos++
		mqc.c += uint32(nextByte) << 8
		mqc.ct = 8
	}
}

// ResetContext resets a context to initial state
func (mqc *MQDecoder) ResetContext(contextID int) {
	mqc.contexts[contextID] = 0
}

// ResetContexts resets all contexts to initial state
func (mqc *MQDecoder) ResetContexts() {
	for i := range mqc.contexts {
		mqc.contexts[i] = 0
	}
}

// ReinitAfterTermination reinitializes the decoder state after a terminated pass
// This is used in TERMALL mode where each pass is independently terminated
// Simply resets the MQC state variables - the decoder will naturally continue
// reading from the current position in the bitstream
func (mqc *MQDecoder) ReinitAfterTermination() {
	// Reset MQC state variables to initial values
	// The probability interval and code register are reset
	mqc.a = 0x8000
	mqc.c = 0
	mqc.ct = 0
	// Note: We do NOT reset pos or lastByte - decoder continues from current position
}

// GetContextState returns the current state of a context
func (mqc *MQDecoder) GetContextState(contextID int) uint8 {
	return mqc.contexts[contextID]
}

// SetContextState sets the state of a context
func (mqc *MQDecoder) SetContextState(contextID int, state uint8) {
	mqc.contexts[contextID] = state
}

// MQ-coder state tables
// Reference: ISO/IEC 15444-1:2019 Table C.2

// qeTable - Qe values for each state
var qeTable = [47]uint32{
	0x5601, 0x3401, 0x1801, 0x0AC1, 0x0521, 0x0221, 0x5601, 0x5401,
	0x4801, 0x3801, 0x3001, 0x2401, 0x1C01, 0x1601, 0x5601, 0x5401,
	0x5101, 0x4801, 0x3801, 0x3401, 0x3001, 0x2801, 0x2401, 0x2201,
	0x1C01, 0x1801, 0x1601, 0x1401, 0x1201, 0x1101, 0x0AC1, 0x09C1,
	0x08A1, 0x0521, 0x0441, 0x02A1, 0x0221, 0x0141, 0x0111, 0x0085,
	0x0049, 0x0025, 0x0015, 0x0009, 0x0005, 0x0001, 0x5601,
}

// nmpsTable - Next state for MPS
var nmpsTable = [47]uint8{
	1, 2, 3, 4, 5, 38, 7, 8,
	9, 10, 11, 12, 13, 29, 15, 16,
	17, 18, 19, 20, 21, 22, 23, 24,
	25, 26, 27, 28, 29, 30, 31, 32,
	33, 34, 35, 36, 37, 38, 39, 40,
	41, 42, 43, 44, 45, 45, 46,
}

// nlpsTable - Next state for LPS
var nlpsTable = [47]uint8{
	1, 6, 9, 12, 29, 33, 6, 14,
	14, 14, 17, 18, 20, 21, 14, 14,
	15, 16, 17, 18, 19, 19, 20, 21,
	22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37,
	38, 39, 40, 41, 42, 43, 46,
}

// switchTable - MPS/LPS switch indicator
var switchTable = [47]uint8{
	1, 0, 0, 0, 0, 0, 1, 0,
	0, 0, 0, 0, 0, 0, 1, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0,
}

// GetQeTable returns the Qe probability estimation table for validation
func GetQeTable() [47]uint32 {
	return qeTable
}

// GetNmpsTable returns the NMPS state transition table for validation
func GetNmpsTable() [47]uint8 {
	return nmpsTable
}

// GetNlpsTable returns the NLPS state transition table for validation
func GetNlpsTable() [47]uint8 {
	return nlpsTable
}

// GetSwitchTable returns the MPS/LPS switch table for validation
func GetSwitchTable() [47]uint8 {
	return switchTable
}
