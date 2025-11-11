package nearlossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
	"github.com/cocosip/go-dicom-codec/jpegls/lossless"
)

// Encoder represents a JPEG-LS near-lossless encoder
type Encoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int  // Maximum sample value (2^bitDepth - 1)
	near       int  // NEAR parameter (maximum error bound)
	qbpp       int  // Quantized bits per pixel
	range_     int  // Quantization range

	contextTable *lossless.ContextTable
	runEncoder   *lossless.RunModeEncoder
}

// NewEncoder creates a new JPEG-LS near-lossless encoder
func NewEncoder(width, height, components, bitDepth, near int) *Encoder {
	maxVal := (1 << uint(bitDepth)) - 1

	// Compute quantization parameters
	qbpp := computeQBPP(maxVal, near)
	range_ := (maxVal + 2*near) / (2*near + 1) + 1

	return &Encoder{
		width:        width,
		height:       height,
		components:   components,
		bitDepth:     bitDepth,
		maxVal:       maxVal,
		near:         near,
		qbpp:         qbpp,
		range_:       range_,
		contextTable: lossless.NewContextTable(maxVal),
		runEncoder:   lossless.NewRunModeEncoder(),
	}
}

// Encode encodes pixel data to JPEG-LS near-lossless format
func Encode(pixelData []byte, width, height, components, bitDepth, near int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, common.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, common.ErrInvalidComponents
	}

	if bitDepth < 2 || bitDepth > 16 {
		return nil, fmt.Errorf("invalid bit depth: %d (must be 2-16)", bitDepth)
	}

	if near < 0 || near > 255 {
		return nil, fmt.Errorf("invalid NEAR parameter: %d (must be 0-255)", near)
	}

	encoder := NewEncoder(width, height, components, bitDepth, near)
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

	// Write JPEG-LS parameters marker (LSE) with NEAR
	if err := enc.writeLSE(writer); err != nil {
		return nil, err
	}

	// Write SOS marker with NEAR parameter
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
	length := 8 + enc.components*3
	data := make([]byte, length)

	data[0] = byte(enc.bitDepth)
	data[1] = byte(enc.height >> 8)
	data[2] = byte(enc.height & 0xFF)
	data[3] = byte(enc.width >> 8)
	data[4] = byte(enc.width & 0xFF)
	data[5] = byte(enc.components)

	for i := 0; i < enc.components; i++ {
		offset := 6 + i*3
		data[offset] = byte(i + 1)
		data[offset+1] = 0x11
		data[offset+2] = 0
	}

	return writer.WriteSegment(0xFFF7, data)
}

// writeLSE writes JPEG-LS parameters (LSE marker) with NEAR
func (enc *Encoder) writeLSE(writer *common.Writer) error {
	data := make([]byte, 13)
	data[0] = 1 // ID = 1 (preset parameters)

	// MAXVAL
	maxVal := uint16(enc.maxVal)
	data[1] = byte(maxVal >> 8)
	data[2] = byte(maxVal & 0xFF)

	// T1, T2, T3 (thresholds) - use defaults for near-lossless
	// Since we pass quantized errors to context, default thresholds work well
	data[3] = 0
	data[4] = 3
	data[5] = 0
	data[6] = 7
	data[7] = 0
	data[8] = 21

	// RESET interval
	data[9] = 0
	data[10] = 0
	data[11] = 0
	data[12] = 64

	return writer.WriteSegment(0xFFF8, data)
}

// writeSOS writes Start of Scan marker with NEAR parameter
func (enc *Encoder) writeSOS(writer *common.Writer) error {
	length := 4 + enc.components*2
	data := make([]byte, length)

	data[0] = byte(enc.components)

	for i := 0; i < enc.components; i++ {
		offset := 1 + i*2
		data[offset] = byte(i + 1)
		data[offset+1] = 0
	}

	// NEAR parameter (key difference from lossless!)
	data[length-3] = byte(enc.near)

	// ILV (interleave mode)
	data[length-2] = 0

	// Point transform
	data[length-1] = 0

	return writer.WriteSegment(common.MarkerSOS, data)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *common.Writer, pixelData []byte) error {
	var scanBuf bytes.Buffer
	gw := lossless.NewGolombWriter(&scanBuf)

	// Convert pixel data to integers
	pixels := enc.pixelsToIntegers(pixelData)

	// Make a copy for encoding (encoder modifies pixels array during near-lossless encoding)
	encodingPixels := make([]int, len(pixels))
	copy(encodingPixels, pixels)

	// Encode each component
	for comp := 0; comp < enc.components; comp++ {
		if err := enc.encodeComponent(gw, encodingPixels, comp); err != nil {
			return err
		}
	}

	if err := gw.Flush(); err != nil {
		return err
	}

	_, err := writer.Write(scanBuf.Bytes())
	return err
}

// encodeComponent encodes a single component
func (enc *Encoder) encodeComponent(gw *lossless.GolombWriter, pixels []int, comp int) error {
	stride := enc.components
	offset := comp

	for y := 0; y < enc.height; y++ {
		for x := 0; x < enc.width; x++ {
			idx := (y*enc.width + x) * stride + offset
			if idx >= len(pixels) {
				continue
			}

			sample := pixels[idx]

			// Get neighbors
			a, b, c, d := enc.getNeighbors(pixels, x, y, comp)

			// Compute context on ORIGINAL values (before quantization)
			// This ensures thresholds work correctly
			q1, q2, q3 := lossless.ComputeContext(a, b, c, d)
			ctx := enc.contextTable.GetContext(q1, q2, q3)

			// Quantize neighbors for prediction
			qa := enc.quantize(a)
			qb := enc.quantize(b)
			qc := enc.quantize(c)

			// Compute prediction on quantized values
			prediction := lossless.Predict(qa, qb, qc)

			// Apply bias correction (only for lossless mode)
			// In near-lossless mode, bias correction causes encoder/decoder desync
			// because quantized errors don't match the gradient-based context selection
			bias := 0
			if enc.near == 0 {
				bias = ctx.GetBias()
			}
			correctedPred := enc.correctPrediction(prediction, bias)

			// Reconstruct prediction to original range
			reconstructedPred := enc.dequantize(correctedPred)

			// Compute error (on original values)
			errValue := sample - reconstructedPred

			// Quantize error for near-lossless
			quantizedError := enc.quantizeError(errValue)

			// Dequantize error for reconstruction
			reconstructedError := enc.dequantizeError(quantizedError)

			// Compute reconstructed sample
			actualSample := reconstructedPred + reconstructedError

			// Clamp to valid range [0, maxVal]
			if actualSample < 0 {
				actualSample = 0
			} else if actualSample > enc.maxVal {
				actualSample = enc.maxVal
			}
			pixels[idx] = actualSample

			// Map error to non-negative
			mappedError := enc.mapErrorValue(quantizedError)

			// Get Golomb parameter
			k := ctx.ComputeGolombParameter()

			// Encode error
			if err := gw.WriteGolomb(mappedError, k); err != nil {
				return err
			}

			// Update context with quantized error
			ctx.UpdateContext(quantizedError)
		}
	}

	return nil
}

// quantize quantizes a value for near-lossless mode
func (enc *Encoder) quantize(val int) int {
	if enc.near == 0 {
		return val
	}
	return val / (2*enc.near + 1)
}

// dequantize reconstructs a value from quantized representation
func (enc *Encoder) dequantize(qval int) int {
	if enc.near == 0 {
		return qval
	}
	return qval * (2*enc.near + 1)
}

// quantizeError quantizes prediction error
func (enc *Encoder) quantizeError(err int) int {
	if enc.near == 0 {
		return err
	}

	if err > 0 {
		return (err + enc.near) / (2*enc.near + 1)
	}
	return -((-err + enc.near) / (2*enc.near + 1))
}

// dequantizeError reconstructs error from quantized error (encoder version)
// According to JPEG-LS standard T.87
func (enc *Encoder) dequantizeError(qerr int) int {
	if enc.near == 0 {
		return qerr
	}

	// JPEG-LS error reconstruction
	// Multiply by quantization step size to get reconstructed error
	return qerr * (2*enc.near + 1)
}

// mapErrorValue maps signed error to non-negative for Golomb coding
func (enc *Encoder) mapErrorValue(errValue int) int {
	if errValue >= 0 {
		return 2 * errValue
	}
	return -2*errValue - 1
}

// correctPrediction applies bias correction
func (enc *Encoder) correctPrediction(prediction, bias int) int {
	prediction += bias

	// Clamp to quantized range
	if prediction < 0 {
		prediction = 0
	} else if prediction >= enc.range_ {
		prediction = enc.range_ - 1
	}

	return prediction
}

// getNeighbors gets neighboring pixels
func (enc *Encoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := enc.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	if x > 0 {
		idx := (y*enc.width + (x - 1)) * stride + offset
		if idx < len(pixels) {
			a = pixels[idx]
		}
	}

	if y > 0 {
		idx := ((y-1)*enc.width + x) * stride + offset
		if idx < len(pixels) {
			b = pixels[idx]
		}
	}

	if x > 0 && y > 0 {
		idx := ((y-1)*enc.width + (x - 1)) * stride + offset
		if idx < len(pixels) {
			c = pixels[idx]
		}
	}

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
		pixels := make([]int, len(pixelData))
		for i, b := range pixelData {
			pixels[i] = int(b)
		}
		return pixels
	}

	numPixels := len(pixelData) / 2
	pixels := make([]int, numPixels)
	for i := 0; i < numPixels; i++ {
		idx := i * 2
		val := int(pixelData[idx]) | (int(pixelData[idx+1]) << 8)
		pixels[i] = val
	}
	return pixels
}

// computeQBPP computes quantized bits per pixel
func computeQBPP(maxVal, near int) int {
	if near == 0 {
		return 0
	}

	qbpp := 0
	temp := maxVal
	for temp > 0 {
		temp >>= 1
		qbpp++
	}
	return qbpp
}
