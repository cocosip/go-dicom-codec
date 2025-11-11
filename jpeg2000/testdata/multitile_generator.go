package testdata

import (
	"bytes"
	"encoding/binary"
)

// GenerateMultiTileJ2K generates a JPEG 2000 codestream with multiple tiles
//
// Parameters:
//   - width, height: Image dimensions
//   - tileWidth, tileHeight: Tile dimensions
//   - bitDepth: Bits per sample (8, 12, or 16)
//   - numLevels: Number of wavelet decomposition levels (0-6)
//   - components: Number of components (1=grayscale, 3=RGB)
func GenerateMultiTileJ2K(width, height, tileWidth, tileHeight, bitDepth, numLevels, components int) []byte {
	buf := &bytes.Buffer{}

	// SOC - Start of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ - Image and Tile Size
	writeSIZMultiTile(buf, width, height, tileWidth, tileHeight, bitDepth, components)

	// COD - Coding Style Default
	writeCODMultiTile(buf, numLevels, components)

	// QCD - Quantization Default
	writeQCDMultiTile(buf, bitDepth, numLevels)

	// Calculate number of tiles
	numTilesX := (width + tileWidth - 1) / tileWidth
	numTilesY := (height + tileHeight - 1) / tileHeight
	numTiles := numTilesX * numTilesY

	// Write each tile
	for tileIdx := 0; tileIdx < numTiles; tileIdx++ {
		// SOT - Start of Tile
		tileLength := calculateTileLengthMultiTile(tileWidth, tileHeight, numLevels, components)
		writeSOTMultiTile(buf, tileIdx, tileLength)

		// SOD - Start of Data
		_ = binary.Write(buf, binary.BigEndian, uint16(0xFF93))

		// Tile data (placeholder)
		writeTileDataMultiTile(buf, tileWidth, tileHeight, components)
	}

	// EOC - End of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func writeSIZMultiTile(buf *bytes.Buffer, width, height, tileWidth, tileHeight, bitDepth, components int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF51)) // SIZ marker

	sizData := &bytes.Buffer{}

	_ = binary.Write(sizData, binary.BigEndian, uint16(0))          // Rsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(width))      // Xsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(height))     // Ysiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))          // XOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))          // YOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(tileWidth))  // XTsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(tileHeight)) // YTsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))          // XTOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))          // YTOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint16(components)) // Csiz

	// Component information
	ssiz := uint8(bitDepth - 1)
	for i := 0; i < components; i++ {
		_ = binary.Write(sizData, binary.BigEndian, ssiz)
		_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // XRsiz
		_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // YRsiz
	}

	length := uint16(sizData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(sizData.Bytes())
}

func writeCODMultiTile(buf *bytes.Buffer, numLevels, components int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF52)) // COD marker

	codData := &bytes.Buffer{}

	// Scod - Coding style parameters
	_ = binary.Write(codData, binary.BigEndian, uint8(0))

	// SGcod - Progression order, number of layers, MCT
	_ = binary.Write(codData, binary.BigEndian, uint8(0))  // LRCP progression order
	_ = binary.Write(codData, binary.BigEndian, uint16(1)) // 1 quality layer

	// MCT: 1 for RGB (3 components), 0 for grayscale
	mct := uint8(0)
	if components == 3 {
		mct = 1
	}
	_ = binary.Write(codData, binary.BigEndian, mct)

	// SPcod - Decomposition levels and code-block size
	_ = binary.Write(codData, binary.BigEndian, uint8(numLevels)) // Number of decomposition levels
	_ = binary.Write(codData, binary.BigEndian, uint8(4))         // Code-block width exponent (2^4 = 16)
	_ = binary.Write(codData, binary.BigEndian, uint8(4))         // Code-block height exponent (2^4 = 16)
	_ = binary.Write(codData, binary.BigEndian, uint8(0))         // Code-block style
	_ = binary.Write(codData, binary.BigEndian, uint8(1))         // Transformation: 5-3 reversible

	length := uint16(codData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(codData.Bytes())
}

func writeQCDMultiTile(buf *bytes.Buffer, bitDepth, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF5C)) // QCD marker

	qcdData := &bytes.Buffer{}

	// Sqcd - Quantization style (no quantization for lossless)
	_ = binary.Write(qcdData, binary.BigEndian, uint8(0))

	// SPqcd - Quantization step size for each subband
	numSubbands := 3*numLevels + 1
	for i := 0; i < numSubbands; i++ {
		_ = binary.Write(qcdData, binary.BigEndian, uint8(bitDepth<<3))
	}

	length := uint16(qcdData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(qcdData.Bytes())
}

func writeSOTMultiTile(buf *bytes.Buffer, tileIdx int, tileLength int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF90)) // SOT marker
	_ = binary.Write(buf, binary.BigEndian, uint16(10))     // Lsot (length)

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx))    // Isot
	_ = binary.Write(buf, binary.BigEndian, uint32(tileLength)) // Psot
	_ = binary.Write(buf, binary.BigEndian, uint8(0))           // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1))           // TNsot
}

func calculateTileLengthMultiTile(tileWidth, tileHeight, numLevels, components int) int {
	// Rough estimate
	baseSize := (tileWidth * tileHeight * components * 2) / 8
	return baseSize + 100
}

func writeTileDataMultiTile(buf *bytes.Buffer, tileWidth, tileHeight, components int) {
	// Write minimal placeholder data for each component
	for c := 0; c < components; c++ {
		buf.WriteByte(0x00) // Empty packet marker
	}

	// Add padding
	placeholderData := make([]byte, 50)
	buf.Write(placeholderData)
}

// Generate2x2TileJ2K generates a simple 2x2 tile grid for testing
func Generate2x2TileJ2K() []byte {
	return GenerateMultiTileJ2K(512, 512, 256, 256, 8, 0, 1)
}

// Generate3x2TileJ2K generates a 3x2 tile grid
func Generate3x2TileJ2K() []byte {
	return GenerateMultiTileJ2K(600, 400, 256, 256, 8, 0, 1)
}

// Generate2x2TileRGBJ2K generates a 2x2 tile grid with RGB
func Generate2x2TileRGBJ2K() []byte {
	return GenerateMultiTileJ2K(512, 512, 256, 256, 8, 0, 3)
}
