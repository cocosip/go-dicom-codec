package mqc

import "fmt"

var mqcCallCount = 0
var mqcDebug = false

// EnableDecoderDebug enables debug logging for MQ decoder
func EnableDecoderDebug() {
	mqcDebug = true
	mqcCallCount = 0
}

// MQ Arithmetic Decoder for JPEG 2000
// Reference: ISO/IEC 15444-1:2019 Annex C
// Based on the MQ-coder (multiplication-free, table-driven arithmetic coder)

// MQDecoder implements the MQ arithmetic decoder
type MQDecoder struct {
	// Input data
	data   []byte
	pos    int  // Current byte position

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

// init initializes the decoder
func (mqc *MQDecoder) init() {
	// Initialize C register (ISO/IEC 15444-1 C.3.4)
	// Load first byte into bits 23-16
	firstByte := byte(0xFF)
	if mqc.pos < len(mqc.data) {
		firstByte = mqc.data[mqc.pos]
		mqc.c = uint32(firstByte) << 16
		mqc.pos++
	} else {
		mqc.c = 0xFF << 16
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
			} else {
				// Bit-stuffed: consume second byte, only 7 bits
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
	mqcCallCount++

	cx := &mqc.contexts[contextID]
	state := *cx & 0x7F  // Lower 7 bits = state
	mps := int(*cx >> 7) // Bit 7 = MPS (More Probable Symbol)

	if mqcDebug && mqcCallCount <= 250 {
		fmt.Printf("[MQC #%03d BEFORE] ctx=%d state=%d mps=%d A=0x%04x C=0x%08x ct=%d\n",
			mqcCallCount, contextID, state, mps, mqc.a, mqc.c, mqc.ct)
	}

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

	if mqcDebug && mqcCallCount <= 250 {
		fmt.Printf("[MQC #%03d AFTER ] bit=%d A=0x%04x C=0x%08x ct=%d newState=%d newMPS=%d\n",
			mqcCallCount, d, mqc.a, mqc.c, mqc.ct, *cx&0x7F, int(*cx>>7))
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
func (mqc *MQDecoder) bytein() {
	if mqcDebug {
		fmt.Printf("[MQC BYTEIN] pos=%d len=%d curByte=0x%02x\n", mqc.pos, len(mqc.data), mqc.data[mqc.pos])
	}

	// Bounds check (should never happen with sentinel, but safety first)
	if mqc.pos >= len(mqc.data) {
		mqc.c += 0xFF00
		mqc.ct = 8
		return
	}

	curByte := mqc.data[mqc.pos]
	mqc.pos++

	// Handle 0xFF byte stuffing
	if curByte == 0xFF {
		// Check if we can read next byte
		if mqc.pos >= len(mqc.data) {
			mqc.c += 0xFF00
			mqc.ct = 8
			return
		}

		nextByte := mqc.data[mqc.pos]
		if nextByte > 0x8F {
			// Marker segment or sentinel - stuffed byte, don't consume next byte
			mqc.c += 0xFF00
			mqc.ct = 8
		} else {
			// Normal 0xFF followed by <=0x8F (bit stuffing)
			mqc.pos++
			mqc.c += uint32(nextByte) << 9
			mqc.ct = 7
		}
	} else {
		// Normal byte
		mqc.c += uint32(curByte) << 8
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
