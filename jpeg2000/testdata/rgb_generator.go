package testdata

import (
	"bytes"
	"encoding/binary"
)

// GenerateRGBJ2K generates a JPEG 2000 codestream with 3 components (RGB)
// This creates a simple RGB image for testing
//
// Parameters:
//   - width, height: Image dimensions
//   - bitDepth: Bits per sample (8, 12, or 16)
//   - numLevels: Number of wavelet decomposition levels (0-6)
func GenerateRGBJ2K(width, height, bitDepth, numLevels int) []byte {
	buf := &bytes.Buffer{}

	// SOC - Start of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF4F))

	// SIZ - Image and Tile Size (with 3 components)
	writeSIZRGB(buf, width, height, bitDepth)

	// COD - Coding Style Default
	writeCODRGB(buf, numLevels)

	// QCD - Quantization Default
	writeQCDRGB(buf, bitDepth, numLevels)

	// SOT - Start of Tile
	tileLength := calculateTileLengthRGB(width, height, numLevels)
	writeSOTRGB(buf, 0, tileLength)

	// SOD - Start of Data
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF93))

	// Tile data (placeholder for MVP)
	writeTileDataRGB(buf, width, height, bitDepth, numLevels)

	// EOC - End of Codestream
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFFD9))

	return buf.Bytes()
}

func writeSIZRGB(buf *bytes.Buffer, width, height, bitDepth int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF51)) // SIZ marker

	sizData := &bytes.Buffer{}

	_ = binary.Write(sizData, binary.BigEndian, uint16(0))       // Rsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(width))   // Xsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(height))  // Ysiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))       // XOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))       // YOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(width))   // XTsiz (tile width)
	_ = binary.Write(sizData, binary.BigEndian, uint32(height))  // YTsiz (tile height)
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))       // XTOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))       // YTOsiz
	_ = binary.Write(sizData, binary.BigEndian, uint16(3))       // Csiz (3 components: RGB)

	// Component 0 (Red/Y)
	ssiz := uint8(bitDepth - 1)
	_ = binary.Write(sizData, binary.BigEndian, ssiz)
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // XRsiz
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // YRsiz

	// Component 1 (Green/Cb)
	_ = binary.Write(sizData, binary.BigEndian, ssiz)
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // XRsiz
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // YRsiz

	// Component 2 (Blue/Cr)
	_ = binary.Write(sizData, binary.BigEndian, ssiz)
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // XRsiz
	_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // YRsiz

	length := uint16(sizData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(sizData.Bytes())
}

func writeCODRGB(buf *bytes.Buffer, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF52)) // COD marker

	codData := &bytes.Buffer{}

	// Scod - Coding style parameters
	_ = binary.Write(codData, binary.BigEndian, uint8(0))

	// SGcod - Progression order, number of layers, MCT
	_ = binary.Write(codData, binary.BigEndian, uint8(0))  // LRCP progression order
	_ = binary.Write(codData, binary.BigEndian, uint16(1)) // 1 quality layer
	_ = binary.Write(codData, binary.BigEndian, uint8(1))  // MCT enabled (for RGB)

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

func writeQCDRGB(buf *bytes.Buffer, bitDepth, numLevels int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF5C)) // QCD marker

	qcdData := &bytes.Buffer{}

	// Sqcd - Quantization style (no quantization for lossless)
	_ = binary.Write(qcdData, binary.BigEndian, uint8(0))

	// SPqcd - Quantization step size for each subband
	// For reversible (5-3) transform with no quantization
	// We need (3 * numLevels + 1) step values
	numSubbands := 3*numLevels + 1
	for i := 0; i < numSubbands; i++ {
		// Reversible quantization: exponent only (8 bits)
		_ = binary.Write(qcdData, binary.BigEndian, uint8(bitDepth<<3))
	}

	length := uint16(qcdData.Len() + 2)
	_ = binary.Write(buf, binary.BigEndian, length)
	buf.Write(qcdData.Bytes())
}

func writeSOTRGB(buf *bytes.Buffer, tileIdx int, tileLength int) {
	_ = binary.Write(buf, binary.BigEndian, uint16(0xFF90)) // SOT marker
	_ = binary.Write(buf, binary.BigEndian, uint16(10))     // Lsot (length)

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx))    // Isot
	_ = binary.Write(buf, binary.BigEndian, uint32(tileLength)) // Psot
	_ = binary.Write(buf, binary.BigEndian, uint8(0))           // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1))           // TNsot (1 tile)
}

func calculateTileLengthRGB(width, height, numLevels int) int {
	// Rough estimate for 3 components
	baseSize := (width * height * 3 * 2) / 8
	return baseSize + 150
}

func writeTileDataRGB(buf *bytes.Buffer, width, height, bitDepth, numLevels int) {
	// Write minimal placeholder data for 3 components
	// In a complete implementation, this would be actual encoded packets
	// for each component

	// For MVP, write empty packets for each component
	for comp := 0; comp < 3; comp++ {
		// Empty packet marker
		buf.WriteByte(0x00)
	}

	// Add some padding to reach minimum size
	placeholderData := make([]byte, 100)
	buf.Write(placeholderData)
}

// GenerateRGBTestImage generates a simple RGB test pattern
// Returns interleaved RGB data [R0,G0,B0,R1,G1,B1,...]
func GenerateRGBTestImage(width, height int) []int32 {
	data := make([]int32, width*height*3)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 3

			// Create a simple pattern:
			// - Top-left: Red
			// - Top-right: Green
			// - Bottom-left: Blue
			// - Bottom-right: White
			// With gradients in between

			r := int32((x * 255) / (width - 1))
			g := int32((y * 255) / (height - 1))
			b := int32(((width - 1 - x) * 255) / (width - 1))

			data[idx] = r
			data[idx+1] = g
			data[idx+2] = b
		}
	}

	return data
}

// GenerateRGBComponents generates RGB test image as separate components
// Returns three arrays: [R], [G], [B]
func GenerateRGBComponents(width, height int) (r, g, b []int32) {
	r = make([]int32, width*height)
	g = make([]int32, width*height)
	b = make([]int32, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x

			// Red channel: horizontal gradient
			r[idx] = int32((x * 255) / (width - 1))

			// Green channel: vertical gradient
			g[idx] = int32((y * 255) / (height - 1))

			// Blue channel: inverse horizontal gradient
			b[idx] = int32(((width - 1 - x) * 255) / (width - 1))
		}
	}

	return
}

// GenerateSolidColorRGB generates a solid color RGB image
func GenerateSolidColorRGB(width, height int, red, green, blue int32) []int32 {
	data := make([]int32, width*height*3)

	for i := 0; i < width*height; i++ {
		data[i*3] = red
		data[i*3+1] = green
		data[i*3+2] = blue
	}

	return data
}

// GenerateColorBarsRGB generates color bars test pattern
// Standard SMPTE color bars
func GenerateColorBarsRGB(width, height int) []int32 {
	data := make([]int32, width*height*3)

	// 7 color bars: White, Yellow, Cyan, Green, Magenta, Red, Blue
	colors := [][3]int32{
		{255, 255, 255}, // White
		{255, 255, 0},   // Yellow
		{0, 255, 255},   // Cyan
		{0, 255, 0},     // Green
		{255, 0, 255},   // Magenta
		{255, 0, 0},     // Red
		{0, 0, 255},     // Blue
	}

	barWidth := width / 7

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			barIdx := x / barWidth
			if barIdx >= 7 {
				barIdx = 6
			}

			idx := (y*width + x) * 3
			data[idx] = colors[barIdx][0]
			data[idx+1] = colors[barIdx][1]
			data[idx+2] = colors[barIdx][2]
		}
	}

	return data
}
