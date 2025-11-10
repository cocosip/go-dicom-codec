package lossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
)

// Encoder represents a JPEG-LS lossless encoder
type Encoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int // Maximum sample value (2^bitDepth - 1)

	contextTable *ContextTable
	runEncoder   *RunModeEncoder
}

// NewEncoder creates a new JPEG-LS encoder
func NewEncoder(width, height, components, bitDepth int) *Encoder {
	maxVal := (1 << uint(bitDepth)) - 1

	return &Encoder{
		width:        width,
		height:       height,
		components:   components,
		bitDepth:     bitDepth,
		maxVal:       maxVal,
		contextTable: NewContextTable(maxVal),
		runEncoder:   NewRunModeEncoder(),
	}
}

// Encode encodes pixel data to JPEG-LS format
// pixelData: raw pixel values (interleaved for multi-component)
// Returns: JPEG-LS compressed data
func Encode(pixelData []byte, width, height, components, bitDepth int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, common.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, common.ErrInvalidComponents
	}

	if bitDepth < 2 || bitDepth > 16 {
		return nil, fmt.Errorf("invalid bit depth: %d (must be 2-16)", bitDepth)
	}

	encoder := NewEncoder(width, height, components, bitDepth)
	return encoder.encode(pixelData)
}

// encode performs the actual encoding
func (enc *Encoder) encode(pixelData []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := common.NewWriter(&buf)

	// Write SOI marker
	if err := writer.WriteMarker(common.MarkerSOI); err != nil {
		return nil, err
	}

	// Write SOF55 marker (JPEG-LS)
	if err := enc.writeSOF55(writer); err != nil {
		return nil, err
	}

	// Write JPEG-LS parameters marker (LSE)
	if err := enc.writeLSE(writer); err != nil {
		return nil, err
	}

	// Write SOS marker
	if err := enc.writeSOS(writer); err != nil {
		return nil, err
	}

	// Encode scan data
	if err := enc.encodeScan(writer, pixelData); err != nil {
		return nil, err
	}

	// Write EOI marker
	if err := writer.WriteMarker(common.MarkerEOI); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeSOF55 writes Start of Frame marker for JPEG-LS
func (enc *Encoder) writeSOF55(writer *common.Writer) error {
	// SOF55 = 0xFFF7
	length := 8 + enc.components*3
	data := make([]byte, length)

	data[0] = byte(enc.bitDepth)           // Precision
	data[1] = byte(enc.height >> 8)        // Height MSB
	data[2] = byte(enc.height & 0xFF)      // Height LSB
	data[3] = byte(enc.width >> 8)         // Width MSB
	data[4] = byte(enc.width & 0xFF)       // Width LSB
	data[5] = byte(enc.components)         // Number of components

	// Component specifications
	for i := 0; i < enc.components; i++ {
		offset := 6 + i*3
		data[offset] = byte(i + 1)  // Component ID
		data[offset+1] = 0x11       // Sampling factors (1x1)
		data[offset+2] = 0          // Quantization table (not used in lossless)
	}

	return writer.WriteSegment(0xFFF7, data)
}

// writeLSE writes JPEG-LS parameters (LSE marker)
func (enc *Encoder) writeLSE(writer *common.Writer) error {
	// LSE = 0xFFF8
	// Write default parameters
	data := make([]byte, 13)
	data[0] = 1 // ID = 1 (preset parameters)

	// MAXVAL (maximum sample value)
	maxVal := uint16(enc.maxVal)
	data[1] = byte(maxVal >> 8)
	data[2] = byte(maxVal & 0xFF)

	// T1, T2, T3 (thresholds) - use defaults
	// T1 = 3, T2 = 7, T3 = 21
	data[3] = 0
	data[4] = 3
	data[5] = 0
	data[6] = 7
	data[7] = 0
	data[8] = 21

	// RESET interval (0 = disabled)
	data[9] = 0
	data[10] = 0
	data[11] = 0
	data[12] = 64

	return writer.WriteSegment(0xFFF8, data)
}

// writeSOS writes Start of Scan marker
func (enc *Encoder) writeSOS(writer *common.Writer) error {
	length := 4 + enc.components*2
	data := make([]byte, length)

	data[0] = byte(enc.components) // Number of components

	// Component selectors
	for i := 0; i < enc.components; i++ {
		offset := 1 + i*2
		data[offset] = byte(i + 1) // Component ID
		data[offset+1] = 0         // No AC/DC tables in JPEG-LS
	}

	// NEAR parameter (0 for lossless)
	data[length-3] = 0

	// ILV (interleave mode): 0 = non-interleaved, 1 = line interleaved, 2 = sample interleaved
	data[length-2] = 0

	// Point transform (0)
	data[length-1] = 0

	return writer.WriteSegment(common.MarkerSOS, data)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *common.Writer, pixelData []byte) error {
	// Create Golomb writer for entropy coding
	var scanBuf bytes.Buffer
	gw := NewGolombWriter(&scanBuf)

	// Convert pixel data to integers
	pixels := enc.pixelsToIntegers(pixelData)

	// Encode each component separately (non-interleaved mode)
	for comp := 0; comp < enc.components; comp++ {
		if err := enc.encodeComponent(gw, pixels, comp); err != nil {
			return err
		}
	}

	// Flush remaining bits
	if err := gw.Flush(); err != nil {
		return err
	}

	// Write scan data directly (it's already byte-stuffed by GolombWriter)
	_, err := writer.Write(scanBuf.Bytes())
	return err
}

// encodeComponent encodes a single component
func (enc *Encoder) encodeComponent(gw *GolombWriter, pixels []int, comp int) error {
	stride := enc.components
	offset := comp

	// Process line by line
	for y := 0; y < enc.height; y++ {
		for x := 0; x < enc.width; x++ {
			idx := (y*enc.width + x) * stride + offset
			if idx >= len(pixels) {
				continue
			}

			sample := pixels[idx]

			// Get neighboring pixels for prediction
			a, b, c, d := enc.getNeighbors(pixels, x, y, comp)

			// Compute prediction
			prediction := Predict(a, b, c)

			// Compute context
			q1, q2, q3 := ComputeContext(a, b, c, d)
			ctx := enc.contextTable.GetContext(q1, q2, q3)

			// Apply bias correction
			bias := ctx.GetBias()
			correctedPred := CorrectPrediction(prediction, bias, enc.maxVal+1)

			// Compute error
			errValue := sample - correctedPred

			// Map error to non-negative
			mappedError := MapErrorValue(errValue, enc.maxVal+1)

			// Get Golomb parameter
			k := ctx.ComputeGolombParameter()

			// Encode error using Golomb-Rice coding
			if err := gw.WriteGolomb(mappedError, k); err != nil {
				return err
			}

			// Update context
			ctx.UpdateContext(errValue)
		}
	}

	return nil
}

// getNeighbors gets the neighboring pixels for prediction
// Returns: a (left), b (top), c (top-left), d (top-right)
func (enc *Encoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := enc.components
	offset := comp

	// Default values for edges
	a, b, c, d := 0, 0, 0, 0

	// a = left pixel (West)
	if x > 0 {
		idx := (y*enc.width + (x - 1)) * stride + offset
		if idx < len(pixels) {
			a = pixels[idx]
		}
	}

	// b = top pixel (North)
	if y > 0 {
		idx := ((y-1)*enc.width + x) * stride + offset
		if idx < len(pixels) {
			b = pixels[idx]
		}
	}

	// c = top-left pixel (North-West)
	if x > 0 && y > 0 {
		idx := ((y-1)*enc.width + (x - 1)) * stride + offset
		if idx < len(pixels) {
			c = pixels[idx]
		}
	}

	// d = top-right pixel (North-East)
	if x < enc.width-1 && y > 0 {
		idx := ((y-1)*enc.width + (x + 1)) * stride + offset
		if idx < len(pixels) {
			d = pixels[idx]
		}
	}

	return a, b, c, d
}

// pixelsToIntegers converts pixel bytes to integer array
func (enc *Encoder) pixelsToIntegers(pixelData []byte) []int {
	if enc.bitDepth <= 8 {
		// 8-bit or less: one byte per sample
		pixels := make([]int, len(pixelData))
		for i, b := range pixelData {
			pixels[i] = int(b)
		}
		return pixels
	}

	// 9-16 bit: two bytes per sample (little-endian)
	numPixels := len(pixelData) / 2
	pixels := make([]int, numPixels)
	for i := 0; i < numPixels; i++ {
		idx := i * 2
		val := int(pixelData[idx]) | (int(pixelData[idx+1]) << 8)
		pixels[i] = val
	}
	return pixels
}
