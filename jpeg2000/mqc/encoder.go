package mqc

import (
	"bytes"
)

// MQEncoder implements the MQ arithmetic encoder
// Reference: ISO/IEC 15444-1:2019 Annex C
type MQEncoder struct {
	// Output buffer
	output *bytes.Buffer

	// State variables
	a   uint32 // Probability interval
	c   uint32 // Code register
	ct  int    // Bit counter

	// Last byte written
	lastByte byte

	// Track if any bytes have been output
	hasOutput bool

	// Contexts
	contexts []uint8 // Context states (one per context)
}

// NewMQEncoder creates a new MQ encoder
func NewMQEncoder(numContexts int) *MQEncoder {
	mqe := &MQEncoder{
		output:    &bytes.Buffer{},
		a:         0x8000,
		c:         0,
		ct:        12,
		lastByte:  0,
		hasOutput: false,
		contexts:  make([]uint8, numContexts),
	}

	// Initialize all contexts to state 0
	for i := range mqe.contexts {
		mqe.contexts[i] = 0
	}

	return mqe
}

// Encode encodes a single bit using the specified context
func (mqe *MQEncoder) Encode(bit int, contextID int) {
	cx := &mqe.contexts[contextID]
	state := *cx & 0x7F  // Lower 7 bits = state
	mps := int(*cx >> 7) // Bit 7 = MPS (More Probable Symbol)

	// Get Qe value for this state
	qe := qeTable[state]

	if bit == mps {
		// Encoding MPS (Most Probable Symbol)
		mqe.a -= qe
		if (mqe.a & 0x8000) == 0 {
			// Renormalization needed
			// Conditional exchange (from ISO/IEC 15444-1)
			if mqe.a < qe {
				mqe.a = qe
			} else {
				mqe.c += qe
			}
			// Update state (MPS path)
			*cx = nmpsTable[state] | (uint8(mps) << 7)
			mqe.renorme()
		} else {
			// No renormalization
			mqe.c += qe
		}
	} else {
		// Encoding LPS (Least Probable Symbol)
		mqe.a -= qe
		if mqe.a < qe {
			mqe.c += qe
		} else {
			mqe.a = qe
		}
		// Update state (LPS path)
		newState := nlpsTable[state]
		newMPS := mps
		if switchTable[state] == 1 {
			newMPS = 1 - mps
		}
		*cx = newState | (uint8(newMPS) << 7)
		mqe.renorme()
	}
}

// renorme renormalizes the encoder (probability interval doubling)
func (mqe *MQEncoder) renorme() {
	for mqe.a < 0x8000 {
		mqe.a <<= 1
		mqe.c <<= 1
		mqe.ct--
		if mqe.ct == 0 {
			mqe.byteout()
		}
	}
}

// byteout outputs a byte to the stream
func (mqe *MQEncoder) byteout() {
	if mqe.lastByte == 0xFF {
		mqe.output.WriteByte(mqe.lastByte)
		mqe.hasOutput = true
		mqe.lastByte = byte((mqe.c >> 20) & 0xFF)
		mqe.c &= 0xFFFFF
		mqe.ct = 7
	} else {
		if (mqe.c & 0x8000000) == 0 {
			// Write lastByte if hasOutput is already true
			// (meaning lastByte was populated from previous byteout)
			if mqe.hasOutput {
				mqe.output.WriteByte(mqe.lastByte)
			}
			mqe.lastByte = byte((mqe.c >> 19) & 0xFF)
			mqe.c &= 0x7FFFF
			mqe.ct = 8
			// Mark hasOutput=true AFTER extracting new lastByte
			mqe.hasOutput = true
		} else {
			mqe.lastByte++
			if mqe.lastByte == 0xFF {
				mqe.c &= 0x7FFFFFF
				mqe.output.WriteByte(mqe.lastByte)
				mqe.hasOutput = true
				mqe.lastByte = byte((mqe.c >> 20) & 0xFF)
				mqe.c &= 0xFFFFF
				mqe.ct = 7
			} else {
				mqe.output.WriteByte(mqe.lastByte)
				mqe.hasOutput = true
				mqe.lastByte = byte((mqe.c >> 19) & 0xFF)
				mqe.c &= 0x7FFFF
				mqe.ct = 8
			}
		}
	}
}

// Flush finalizes encoding and returns the encoded data
func (mqe *MQEncoder) Flush() []byte {
	// setbits: fill remaining bits with 1's for flushing
	// Mirror OpenJPEG's opj_mqc_setbits() - always fill 0xFFFF
	tempC := mqe.c + mqe.a

	// Fill with 0xFFFF to match OpenJPEG exactly
	mqe.c |= 0xFFFF

	if mqe.c >= tempC {
		mqe.c -= 0x8000
	}

	// Output final bytes
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	// Write last byte if not 0xFF
	if mqe.lastByte != 0xFF {
		mqe.output.WriteByte(mqe.lastByte)
	}

	return mqe.output.Bytes()
}

// GetBuffer returns the current output buffer (for layered encoding)
func (mqe *MQEncoder) GetBuffer() []byte {
	return mqe.output.Bytes()
}

// NumBytes returns the current number of bytes in the output buffer
// This is used for rate tracking in multi-layer encoding (following OpenJPEG)
func (mqe *MQEncoder) NumBytes() int {
	return mqe.output.Len()
}

// FlushToOutput flushes the encoder to the output buffer
// This is used for pass termination in multi-layer encoding
func (mqe *MQEncoder) FlushToOutput() {
	// setbits: fill remaining bits with 1's for flushing
	// Mirror OpenJPEG's opj_mqc_setbits() - always fill 0xFFFF
	tempC := mqe.c + mqe.a

	// Fill with 0xFFFF to match OpenJPEG exactly
	mqe.c |= 0xFFFF

	if mqe.c >= tempC {
		mqe.c -= 0x8000
	}

	// Output final bytes
	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	mqe.c <<= uint(mqe.ct)
	mqe.byteout()

	// Write last byte if not 0xFF
	if mqe.lastByte != 0xFF {
		mqe.output.WriteByte(mqe.lastByte)
	}

	// Reset state for next pass (but don't reset output buffer or contexts)
	mqe.a = 0x8000
	mqe.c = 0
	mqe.ct = 12
	mqe.lastByte = 0
	mqe.hasOutput = false
}

// Reset resets the encoder state
func (mqe *MQEncoder) Reset() {
	mqe.output.Reset()
	mqe.a = 0x8000
	mqe.c = 0
	mqe.ct = 12
	mqe.lastByte = 0
	mqe.hasOutput = false
}

// ResetContext resets a context to initial state
func (mqe *MQEncoder) ResetContext(contextID int) {
	mqe.contexts[contextID] = 0
}

// ResetContexts resets all contexts to initial state
func (mqe *MQEncoder) ResetContexts() {
	for i := range mqe.contexts {
		mqe.contexts[i] = 0
	}
}

// GetContextState returns the current state of a context
func (mqe *MQEncoder) GetContextState(contextID int) uint8 {
	return mqe.contexts[contextID]
}

// SetContextState sets the state of a context
func (mqe *MQEncoder) SetContextState(contextID int, state uint8) {
	mqe.contexts[contextID] = state
}
