package nearlossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
	jpegcommon "github.com/cocosip/go-dicom-codec/jpegls/common"
	"github.com/cocosip/go-dicom-codec/jpegls/lossless"
)

// Encoder represents a JPEG-LS near-lossless encoder
type Encoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int // Maximum sample value (2^bitDepth - 1)
	near       int // NEAR parameter (maximum error bound)

	traits         lossless.Traits
	contextTable   *lossless.ContextTable
	quantizer      *lossless.GradientQuantizer
	runModeScanner *lossless.RunModeScanner
}

// NewEncoder creates a new JPEG-LS near-lossless encoder
func NewEncoder(width, height, components, bitDepth, near int) *Encoder {
	maxVal := (1 << uint(bitDepth)) - 1
	traits := lossless.NewTraits(maxVal, near, 64)

	return &Encoder{
		width:          width,
		height:         height,
		components:     components,
		bitDepth:       bitDepth,
		maxVal:         maxVal,
		near:           near,
		traits:         traits,
		contextTable:   lossless.NewContextTable(maxVal, near, traits.Reset),
		quantizer:      lossless.NewGradientQuantizer(traits.T1, traits.T2, traits.T3, near),
		runModeScanner: lossless.NewRunModeScanner(traits),
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
	data[3] = byte(enc.quantizer.T1 >> 8)
	data[4] = byte(enc.quantizer.T1 & 0xFF)
	data[5] = byte(enc.quantizer.T2 >> 8)
	data[6] = byte(enc.quantizer.T2 & 0xFF)
	data[7] = byte(enc.quantizer.T3 >> 8)
	data[8] = byte(enc.quantizer.T3 & 0xFF)

	// RESET interval
	data[9] = byte(enc.traits.Reset >> 8)
	data[10] = byte(enc.traits.Reset & 0xFF)
	// Keep 2-byte RESET only; extra padding bytes remain zero
	data[11] = 0
	data[12] = 0

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
		// Reset run index at start of each line (JPEG-LS standard)
		enc.runModeScanner.ResetLine()

		x := 0
		for x < enc.width {
			idx := (y*enc.width+x)*stride + offset
			if idx >= len(pixels) {
				x++
				continue
			}

			sample := pixels[idx]

			// Get neighbors
			a, b, c, d := enc.getNeighbors(pixels, x, y, comp)

			// Compute context on ORIGINAL values (before quantization)
			// This ensures thresholds work correctly
			q1, q2, q3 := enc.quantizer.ComputeContext(a, b, c, d)

			// Compute context ID to check for RUN mode
			qs := lossless.ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode - EXACTLY following CharLS do_regular (encoder)
				// Reference: CharLS scan.h line 336-351

				// const int32_t sign{bit_wise_sign(qs)};
				sign := lossless.BitwiseSign(qs)

				// context_regular_mode& context{contexts_[apply_sign(qs, sign)]};
				ctx := enc.contextTable.GetContext(q1, q2, q3)

				// const int32_t k{context.get_golomb_coding_parameter()};
				k := ctx.ComputeGolombParameter()

				// predicted = get_predicted_value(ra, rb, rc)
				prediction := lossless.Predict(a, b, c)

				// const int32_t predicted_value{traits_.correct_prediction(predicted + apply_sign(context.c(), sign))};
				predictedValue := enc.correctPrediction(prediction + lossless.ApplySign(ctx.GetPredictionCorrection(), sign))

				// const int32_t error_value{traits_.compute_error_value(apply_sign(x - predicted_value, sign))};
				// where compute_error_value(e) = modulo_range(quantize(e))
				errorValue := enc.computeErrorValue(lossless.ApplySign(sample-predictedValue, sign))

				// encode_mapped_value(k, map_error_value(context.get_error_correction(k | traits_.near_lossless) ^ error_value), traits_.limit);
				mappedError := lossless.MapErrorValue(ctx.GetErrorCorrection(k|enc.near, enc.near) ^ errorValue)
				if err := gw.EncodeMappedValue(k, mappedError, enc.traits.Limit, enc.traits.Qbpp); err != nil {
					return err
				}

				// context.update_variables_and_bias(error_value, traits_.near_lossless, traits_.reset_threshold);
				ctx.UpdateContext(errorValue, enc.near, enc.traits.Reset)

				// return traits_.compute_reconstructed_sample(predicted_value, apply_sign(error_value, sign));
				// where compute_reconstructed_sample(pv, ev) = fix_reconstructed_value(pv + dequantize(ev))
				actualSample := enc.traits.ComputeReconstructedSample(predictedValue, lossless.ApplySign(errorValue, sign))
				pixels[idx] = actualSample

				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := enc.doRunMode(gw, pixels, x, y, comp)
				if err != nil {
					return err
				}
				x += pixelsProcessed
			}
		}
	}

	return nil
}

// computeErrorValue matches CharLS traits_.compute_error_value(e)
// which is: modulo_range(quantize(e))
func (enc *Encoder) computeErrorValue(e int) int {
	// quantize
	quantized := enc.quantizeError(e)
	// modulo_range
	return enc.traits.ModuloRange(quantized)
}

// quantizeError quantizes prediction error (CharLS quantize)
func (enc *Encoder) quantizeError(err int) int {
	if enc.near == 0 {
		return err
	}

	// CharLS default_traits.h line 127-133
	if err > 0 {
		return (err + enc.near) / (2*enc.near + 1)
	}
	return -(enc.near - err) / (2*enc.near + 1)
}

// fixReconstructedValue handles wraparound for reconstructed values
func (enc *Encoder) fixReconstructedValue(value int) int {
	return enc.traits.ComputeReconstructedSample(0, value)
}

// dequantizeError reconstructs error from quantized error (encoder version)
// According to CharLS implementation and JPEG-LS standard T.87
func (enc *Encoder) dequantizeError(qerr int) int {
	if enc.near == 0 {
		return qerr
	}

	// CharLS implementation: simple multiplication
	// The NEAR offset is handled by modulo_range operation, not here
	return qerr * (2*enc.near + 1)
}

// correctPrediction clamps predicted value to [0, MaxVal].
// CharLS: default_traits.h correct_prediction() line 83-89
func (enc *Encoder) correctPrediction(predicted int) int {
	// if ((predicted & maximum_sample_value) == predicted) return predicted;
	if (predicted & enc.maxVal) == predicted {
		return predicted
	}

	// return (~(predicted >> (int32_t_bit_count - 1))) & maximum_sample_value;
	// This handles negative values by returning 0, and values > maxVal by clamping
	if predicted < 0 {
		return 0
	}
	return enc.maxVal
}

// getNeighbors gets neighboring pixels following CharLS edge handling
// CharLS initializes edge pixels as:
//   current_line_[-1] = previous_line_[0]  (left edge = pixel above)
//   previous_line_[width_] = previous_line_[width_ - 1] (right padding = rightmost top pixel)
func (enc *Encoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := enc.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	// b: pixel above current position
	if y > 0 {
		idx := ((y-1)*enc.width+x)*stride + offset
		if idx < len(pixels) {
			b = pixels[idx]
		}
	}

	// a: pixel to the left
	// CharLS: when x=0, current_line_[-1] = previous_line_[0], so a = b
	if x > 0 {
		idx := (y*enc.width+(x-1))*stride + offset
		if idx < len(pixels) {
			a = pixels[idx]
		}
	} else if y > 0 {
		// Left edge: a = b (pixel above)
		a = b
	}

	// c: pixel diagonally above-left
	if x > 0 && y > 0 {
		idx := ((y-1)*enc.width+(x-1))*stride + offset
		if idx < len(pixels) {
			c = pixels[idx]
		}
	}

	// d: pixel above-right
	// CharLS: previous_line_[width_] = previous_line_[width_ - 1]
	if y > 0 {
		if x < enc.width-1 {
			idx := ((y-1)*enc.width+(x+1))*stride + offset
			if idx < len(pixels) {
				d = pixels[idx]
			}
		} else {
			// Right edge: d = b (rightmost top pixel)
			d = b
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

// doRunMode handles encoding in run mode (when qs == 0) for near-lossless
func (enc *Encoder) doRunMode(gw *lossless.GolombWriter, pixels []int, x, y, comp int) (int, error) {
	stride := enc.components
	offset := comp

	startIdx := y*enc.width + x
	remainingInLine := enc.width - x

	// Get ra (left pixel)
	raIdx := (startIdx-1)*stride + offset
	ra := 0
	if raIdx >= 0 && raIdx < len(pixels) {
		ra = pixels[raIdx]
	}

	// Count run length
	// In JPEG-LS standard, RUN mode continues while |x - ra| <= NEAR
	// where ra is the reconstructed value of the left pixel
	runLength := 0
	for runLength < remainingInLine {
		currIdx := (startIdx+runLength)*stride + offset
		if currIdx >= len(pixels) {
			break
		}

		currPixel := pixels[currIdx]

		// Check if pixel is close enough to ra to be in the run
		// According to JPEG-LS T.87 standard, RUN mode continues while |Ix - Ra| <= NEAR
		if jpegcommon.Abs(currPixel-ra) <= enc.near {
			runLength++
		} else {
			break
		}
	}

	// Set all pixels in the run to ra (reconstructed value in near-lossless)
	for i := 0; i < runLength; i++ {
		idx := (startIdx+i)*stride + offset
		if idx < len(pixels) {
			pixels[idx] = ra
		}
	}

	// Encode the run using RunModeScanner
	endOfLine := (runLength == remainingInLine)
	if err := enc.runModeScanner.EncodeRunLength(gw, runLength, endOfLine); err != nil {
		return 0, err
	}

	if endOfLine {
		return runLength, nil
	}

	// Handle run interruption
	interruptIdx := (startIdx+runLength)*stride + offset
	xInterrupt := pixels[interruptIdx]

	// Get rb (top pixel at interruption point)
	rbIdx := ((y-1)*enc.width+(x+runLength))*stride + offset
	rb := 0
	if rbIdx >= 0 && rbIdx < len(pixels) {
		rb = pixels[rbIdx]
	}

	// Encode interruption pixel
	reconstructed, err := enc.encodeRunInterruptionPixel(gw, xInterrupt, ra, rb)
	if err != nil {
		return 0, err
	}

	// Store reconstructed value
	pixels[interruptIdx] = reconstructed

	enc.runModeScanner.DecRunIndex()

	return runLength + 1, nil
}

// encodeRunInterruptionPixel encodes the pixel that interrupts a run
// Matches CharLS encode_run_interruption_pixel (scan.h lines 779-791)
func (enc *Encoder) encodeRunInterruptionPixel(gw *lossless.GolombWriter, x, ra, rb int) (int, error) {
	// CharLS: if (std::abs(ra - rb) <= traits_.near_lossless)
	if jpegcommon.Abs(ra-rb) <= enc.near {
		// Use run mode context 1
		// CharLS: const int32_t error_value{traits_.compute_error_value(x - ra)};
		errorValue := enc.computeErrorValue(x - ra)

		if err := enc.runModeScanner.EncodeRunInterruption(gw, enc.runModeScanner.RunModeContexts[1], errorValue); err != nil {
			return 0, err
		}

		// CharLS: return traits_.compute_reconstructed_sample(ra, error_value)
		reconstructed := enc.traits.ComputeReconstructedSample(ra, errorValue)
		return reconstructed, nil
	}

	// Use run mode context 0
	// CharLS: const int32_t error_value{traits_.compute_error_value((x - rb) * sign(rb - ra))};
	errorValue := enc.computeErrorValue((x - rb) * jpegcommon.Sign(rb-ra))

	if err := enc.runModeScanner.EncodeRunInterruption(gw, enc.runModeScanner.RunModeContexts[0], errorValue); err != nil {
		return 0, err
	}

	// CharLS: return traits_.compute_reconstructed_sample(rb, error_value * sign(rb - ra))
	reconstructed := enc.traits.ComputeReconstructedSample(rb, errorValue*jpegcommon.Sign(rb-ra))
	return reconstructed, nil
}
