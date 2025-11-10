package mqc

// MQ Arithmetic Decoder for JPEG 2000
// Reference: ISO/IEC 15444-1:2019 Annex C
// Based on the MQ-coder (multiplication-free, table-driven arithmetic coder)

// MQDecoder implements the MQ arithmetic decoder
type MQDecoder struct {
	// Input data
	data   []byte
	pos    int  // Current byte position
	bitPos int  // Bit position within current byte

	// State variables
	a   uint32 // Probability interval
	c   uint32 // Code register
	ct  int    // Bit counter
	curByte byte   // Current byte being processed

	// Contexts
	contexts []uint8 // Context states (one per context)
}

// NewMQDecoder creates a new MQ decoder
func NewMQDecoder(data []byte, numContexts int) *MQDecoder {
	mqc := &MQDecoder{
		data:     data,
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
	// Fill C register
	mqc.c = 0
	mqc.bytein()
	mqc.c = mqc.c << 8
	mqc.bytein()
	mqc.c = mqc.c << 7
	mqc.ct -= 7
	mqc.a = 0x8000
}

// Decode decodes a single bit using the specified context
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
		// Coded bit is LPS
		d = 1 - mps
		mqc.c -= qe << 16

		// Conditional exchange
		if mqc.a < qe {
			mqc.a = qe
		}

		// Update state
		newState := nlpsTable[state]
		newMPS := mps
		if switchTable[state] == 1 {
			newMPS = 1 - mps
		}
		*cx = newState | (uint8(newMPS) << 7)
		mqc.renormd()
	} else {
		// Coded bit is MPS
		d = mps
		mqc.c -= qe << 16

		if mqc.a >= 0x8000 {
			// No renormalization needed
			return d
		}

		// Update state (MPS path)
		newState := nmpsTable[state]
		*cx = newState | (uint8(mps) << 7)
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
func (mqc *MQDecoder) bytein() {
	if mqc.pos < len(mqc.data) {
		mqc.curByte = mqc.data[mqc.pos]
		mqc.pos++

		// Handle byte stuffing (0xFF followed by more than 0x8F)
		if mqc.curByte == 0xFF {
			if mqc.pos < len(mqc.data) {
				next := mqc.data[mqc.pos]
				if next > 0x8F {
					mqc.c += 0xFF00
					mqc.ct = 8
					return
				}
				mqc.pos++
			}
		}

		mqc.c += uint32(mqc.curByte) << (8 + mqc.ct)
		mqc.ct = 8
	} else {
		// No more data - pad with 0xFF
		mqc.c += 0xFF00
		mqc.ct = 8
	}
}

// ResetContext resets a context to initial state
func (mqc *MQDecoder) ResetContext(contextID int) {
	mqc.contexts[contextID] = 0
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
