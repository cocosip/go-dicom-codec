package testdata

import (
	"bytes"
	"encoding/binary"
)

// GenerateSimpleJ2K generates a minimal valid JPEG 2000 codestream
// for testing purposes. This creates the simplest possible JPEG 2000:
// - Single tile
// - Grayscale (1 component)
// - No wavelet transform (0 decomposition levels)
// - No entropy coding (all-zero data)
//
// This is mainly for testing the codestream parser and basic structure.
func GenerateSimpleJ2K(width, height, bitDepth int) []byte {
	buf := bytes.Buffer{}

	// SOC - Start of Codestream
	writeMarker(&buf, 0xFF4F) // SOC

	// SIZ - Image and Tile Size
	writeSIZ(&buf, width, height, bitDepth)

	// COD - Coding Style Default
	writeCOD(&buf, 0) // 0 decomposition levels

	// QCD - Quantization Default
	writeQCD(&buf, bitDepth)

	// SOT - Start of Tile
	writeSOT(&buf, 0, calculateTileLength(width, height))

	// SOD - Start of Data
	writeMarker(&buf, 0xFF93) // SOD

	// Tile data (empty for now - will be minimal placeholder)
	// For 0 decomposition levels and no coding passes, we need minimal data
	writeTileData(&buf, width, height)

	// EOC - End of Codestream
	writeMarker(&buf, 0xFFD9) // EOC

	return buf.Bytes()
}

// writeMarker writes a 2-byte marker
func writeMarker(buf *bytes.Buffer, marker uint16) {
	binary.Write(buf, binary.BigEndian, marker)
}

// writeSIZ writes the SIZ (Image and Tile Size) segment
func writeSIZ(buf *bytes.Buffer, width, height, bitDepth int) {
	writeMarker(buf, 0xFF51) // SIZ marker

	sizData := bytes.Buffer{}

	// Rsiz - Capabilities (0 = baseline)
	binary.Write(&sizData, binary.BigEndian, uint16(0))

	// Xsiz, Ysiz - Image width and height
	binary.Write(&sizData, binary.BigEndian, uint32(width))
	binary.Write(&sizData, binary.BigEndian, uint32(height))

	// XOsiz, YOsiz - Image offset
	binary.Write(&sizData, binary.BigEndian, uint32(0))
	binary.Write(&sizData, binary.BigEndian, uint32(0))

	// XTsiz, YTsiz - Tile width and height (same as image = single tile)
	binary.Write(&sizData, binary.BigEndian, uint32(width))
	binary.Write(&sizData, binary.BigEndian, uint32(height))

	// XTOsiz, YTOsiz - Tile offset
	binary.Write(&sizData, binary.BigEndian, uint32(0))
	binary.Write(&sizData, binary.BigEndian, uint32(0))

	// Csiz - Number of components
	binary.Write(&sizData, binary.BigEndian, uint16(1)) // Grayscale

	// Component 0
	ssiz := uint8(bitDepth - 1) // Bit depth - 1
	binary.Write(&sizData, binary.BigEndian, ssiz)
	binary.Write(&sizData, binary.BigEndian, uint8(1)) // XRsiz - horizontal separation
	binary.Write(&sizData, binary.BigEndian, uint8(1)) // YRsiz - vertical separation

	// Write length
	length := uint16(sizData.Len() + 2) // +2 for length field itself
	binary.Write(buf, binary.BigEndian, length)

	// Write data
	buf.Write(sizData.Bytes())
}

// writeCOD writes the COD (Coding Style Default) segment
func writeCOD(buf *bytes.Buffer, numLevels int) {
	writeMarker(buf, 0xFF52) // COD marker

	codData := bytes.Buffer{}

	// Scod - Coding style
	binary.Write(&codData, binary.BigEndian, uint8(0)) // Default

	// SGcod - Progression order, layers, etc
	binary.Write(&codData, binary.BigEndian, uint8(0))    // Progression order (LRCP)
	binary.Write(&codData, binary.BigEndian, uint16(1))   // Number of layers
	binary.Write(&codData, binary.BigEndian, uint8(0))    // Multiple component transform (none)
	binary.Write(&codData, binary.BigEndian, uint8(numLevels)) // Decomposition levels
	binary.Write(&codData, binary.BigEndian, uint8(4))    // Code-block width (2^(4+2) = 64)
	binary.Write(&codData, binary.BigEndian, uint8(4))    // Code-block height (2^(4+2) = 64)
	binary.Write(&codData, binary.BigEndian, uint8(0))    // Code-block style
	binary.Write(&codData, binary.BigEndian, uint8(1))    // Transformation (5-3 reversible)

	// Write length
	length := uint16(codData.Len() + 2)
	binary.Write(buf, binary.BigEndian, length)

	// Write data
	buf.Write(codData.Bytes())
}

// writeQCD writes the QCD (Quantization Default) segment
func writeQCD(buf *bytes.Buffer, bitDepth int) {
	writeMarker(buf, 0xFF5C) // QCD marker

	qcdData := bytes.Buffer{}

	// Sqcd - Quantization style (no quantization for reversible)
	binary.Write(&qcdData, binary.BigEndian, uint8(0)) // No quantization

	// SPqcd - Quantization step size (just exponent for reversible)
	// For reversible, we just specify the number of guard bits
	exponent := uint8(bitDepth + 1) // Simple exponent
	binary.Write(&qcdData, binary.BigEndian, exponent)

	// Write length
	length := uint16(qcdData.Len() + 2)
	binary.Write(buf, binary.BigEndian, length)

	// Write data
	buf.Write(qcdData.Bytes())
}

// writeSOT writes the SOT (Start of Tile) segment
func writeSOT(buf *bytes.Buffer, tileIndex int, tileLength uint32) {
	writeMarker(buf, 0xFF90) // SOT marker

	// Length
	binary.Write(buf, binary.BigEndian, uint16(10)) // SOT is always 10 bytes

	// Isot - Tile index
	binary.Write(buf, binary.BigEndian, uint16(tileIndex))

	// Psot - Tile-part length (including SOT marker but not SOT marker itself)
	binary.Write(buf, binary.BigEndian, tileLength)

	// TPsot - Tile-part index
	binary.Write(buf, binary.BigEndian, uint8(0))

	// TNsot - Number of tile-parts
	binary.Write(buf, binary.BigEndian, uint8(1))
}

// calculateTileLength calculates the tile-part length
func calculateTileLength(width, height int) uint32 {
	// SOT marker segment (12 bytes: marker + length + data)
	// SOD marker (2 bytes)
	// Tile data (minimal - just a few bytes for empty packets)
	// Total: approximately 20 bytes for minimal tile
	return uint32(20)
}

// writeTileData writes minimal tile data
func writeTileData(buf *bytes.Buffer, width, height int) {
	// For the simplest case with 0 decomposition levels:
	// We need to write packet data
	// Each packet has a header and body
	//
	// For an empty packet (all coefficients are zero):
	// - Packet header: 1 bit (header present = 0 for empty)
	// We'll write just a single zero byte to represent an empty packet
	buf.WriteByte(0x00)
}

// GenerateUniformImage generates a JPEG 2000 codestream with uniform pixel values
// This is slightly more complex than GenerateSimpleJ2K but still very minimal
func GenerateUniformImage(width, height, bitDepth int, value int32) []byte {
	// For now, just return simple J2K
	// TODO: Implement actual pixel encoding when encoder is ready
	return GenerateSimpleJ2K(width, height, bitDepth)
}
