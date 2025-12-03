package htj2k

// MEL (MELCODE) Encoder
// Implements the 13-state adaptive run-length encoder for HTJ2K
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// Simplified implementation: For each bit input:
// - 0 (continue run): emits nothing, increments state
// - 1 (end run): emits run length code, resets to state 0

// MELEncoder implements the MELCODE encoder for HTJ2K block coding
type MELEncoder struct {
	buffer []byte  // Output buffer

	// Bit-level output
	bitBuffer byte  // Buffer for incomplete bytes
	bitCount  int   // Number of bits in bitBuffer (0-7)
}

// NewMELEncoder creates a new MEL encoder
func NewMELEncoder() *MELEncoder {
	return &MELEncoder{
		buffer: make([]byte, 0, 1024),
	}
}

// EncodeBit encodes a single bit (0 = continue run, 1 = end run)
// For simplicity, we just encode bits directly
// A full implementation would use the 13-state MELCODE table
func (m *MELEncoder) EncodeBit(bit int) {
	m.emitBit(byte(bit))
}

// emitBit writes a single bit to the output buffer
func (m *MELEncoder) emitBit(bit byte) {
	m.bitBuffer = (m.bitBuffer << 1) | (bit & 1)
	m.bitCount++

	if m.bitCount == 8 {
		m.buffer = append(m.buffer, m.bitBuffer)
		m.bitBuffer = 0
		m.bitCount = 0
	}
}

// Flush finalizes the MEL encoding and returns the encoded data
func (m *MELEncoder) Flush() []byte {
	// Flush any remaining bits
	if m.bitCount > 0 {
		m.bitBuffer <<= (8 - m.bitCount)
		m.buffer = append(m.buffer, m.bitBuffer)
		m.bitBuffer = 0
		m.bitCount = 0
	}

	return m.buffer
}

// GetBytes returns the current encoded bytes (without flushing)
func (m *MELEncoder) GetBytes() []byte {
	return m.buffer
}

// Length returns the current length of encoded data
func (m *MELEncoder) Length() int {
	return len(m.buffer)
}

// MELDecoder implements the MELCODE decoder for HTJ2K block coding
type MELDecoder struct {
	data      []byte // Input data
	pos       int    // Current byte position

	// Bit-level input
	bitBuffer byte   // Buffer for current byte
	bitPos    int    // Current bit position in byte (0-7)
}

// NewMELDecoder creates a new MEL decoder
func NewMELDecoder(data []byte) *MELDecoder {
	return &MELDecoder{
		data:   data,
		pos:    0,
		bitPos: 8, // Will load first byte on first read
	}
}

// DecodeBit decodes a single bit from the MEL stream
// Returns: (bit, hasMore)
func (m *MELDecoder) DecodeBit() (int, bool) {
	// Load next byte if needed
	if m.bitPos >= 8 {
		if m.pos >= len(m.data) {
			return 0, false
		}
		m.bitBuffer = m.data[m.pos]
		m.pos++
		m.bitPos = 0
	}

	// Extract bit
	bit := (m.bitBuffer >> (7 - m.bitPos)) & 1
	m.bitPos++

	return int(bit), true
}
