package htj2k

// MagSgn (Magnitude-Sign) Encoder/Decoder for HTJ2K
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// The MagSgn segment stores magnitude and sign bits for significant samples
// It grows forward in the codeblock

// MagSgnEncoder implements the magnitude-sign encoder for HTJ2K
type MagSgnEncoder struct {
	buffer    []byte  // Output buffer (grows forward)

	// Bit-level output
	bitBuffer byte    // Buffer for incomplete bytes
	bitCount  int     // Number of bits in bitBuffer (0-7)
}

// NewMagSgnEncoder creates a new MagSgn encoder
func NewMagSgnEncoder() *MagSgnEncoder {
	return &MagSgnEncoder{
		buffer:    make([]byte, 0, 2048),
		bitBuffer: 0,
		bitCount:  0,
	}
}

// EncodeMagSgn encodes magnitude and sign bits for a sample
// mag: magnitude value (absolute value of coefficient)
// sign: sign bit (0 = positive, 1 = negative)
// numBits: number of magnitude bits to encode
func (m *MagSgnEncoder) EncodeMagSgn(mag uint32, sign int, numBits int) {
	// Encode magnitude bits
	for i := numBits - 1; i >= 0; i-- {
		bit := (mag >> i) & 1
		m.writeBit(byte(bit))
	}

	// Encode sign bit
	m.writeBit(byte(sign))
}

// EncodeMagnitude encodes only magnitude bits (no sign)
func (m *MagSgnEncoder) EncodeMagnitude(mag uint32, numBits int) {
	for i := numBits - 1; i >= 0; i-- {
		bit := (mag >> i) & 1
		m.writeBit(byte(bit))
	}
}

// EncodeSign encodes only sign bit
func (m *MagSgnEncoder) EncodeSign(sign int) {
	m.writeBit(byte(sign))
}

// writeBit writes a single bit to the buffer
func (m *MagSgnEncoder) writeBit(bit byte) {
	m.bitBuffer = (m.bitBuffer << 1) | (bit & 1)
	m.bitCount++

	if m.bitCount == 8 {
		m.buffer = append(m.buffer, m.bitBuffer)
		m.bitBuffer = 0
		m.bitCount = 0
	}
}

// Flush finalizes encoding and returns the buffer
func (m *MagSgnEncoder) Flush() []byte {
	// Flush remaining bits
	if m.bitCount > 0 {
		m.bitBuffer <<= (8 - m.bitCount)
		m.buffer = append(m.buffer, m.bitBuffer)
		m.bitBuffer = 0
		m.bitCount = 0
	}

	return m.buffer
}

// GetBytes returns the current buffer without flushing
func (m *MagSgnEncoder) GetBytes() []byte {
	return m.buffer
}

// Length returns the current length of encoded data
func (m *MagSgnEncoder) Length() int {
	return len(m.buffer)
}

// MagSgnDecoder implements the magnitude-sign decoder for HTJ2K
type MagSgnDecoder struct {
	data      []byte  // Input data
	pos       int     // Current byte position

	// Bit-level input
	bitBuffer byte    // Current byte being read
	bitPos    int     // Current bit position (0-7)
}

// NewMagSgnDecoder creates a new MagSgn decoder
func NewMagSgnDecoder(data []byte) *MagSgnDecoder {
	return &MagSgnDecoder{
		data:      data,
		pos:       0,
		bitBuffer: 0,
		bitPos:    8, // Will load first byte on first read
	}
}

// DecodeMagSgn decodes magnitude and sign bits
// numBits: number of magnitude bits to decode
// Returns: (magnitude, sign, hasMore)
func (m *MagSgnDecoder) DecodeMagSgn(numBits int) (uint32, int, bool) {
	// Decode magnitude bits
	mag := uint32(0)
	for i := 0; i < numBits; i++ {
		bit, ok := m.readBit()
		if !ok {
			return 0, 0, false
		}
		mag = (mag << 1) | uint32(bit)
	}

	// Decode sign bit
	sign, ok := m.readBit()
	if !ok {
		return mag, 0, false
	}

	return mag, sign, true
}

// DecodeMagnitude decodes only magnitude bits
func (m *MagSgnDecoder) DecodeMagnitude(numBits int) (uint32, bool) {
	mag := uint32(0)
	for i := 0; i < numBits; i++ {
		bit, ok := m.readBit()
		if !ok {
			return 0, false
		}
		mag = (mag << 1) | uint32(bit)
	}
	return mag, true
}

// DecodeSign decodes only sign bit
func (m *MagSgnDecoder) DecodeSign() (int, bool) {
	return m.readBit()
}

// readBit reads a single bit from the input stream
func (m *MagSgnDecoder) readBit() (int, bool) {
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

// HasMore returns true if more data is available
func (m *MagSgnDecoder) HasMore() bool {
	return m.pos < len(m.data) || m.bitPos < 8
}

// Reset resets the decoder to the beginning
func (m *MagSgnDecoder) Reset() {
	m.pos = 0
	m.bitPos = 8
	m.bitBuffer = 0
}
