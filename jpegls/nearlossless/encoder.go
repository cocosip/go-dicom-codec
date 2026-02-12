package nearlossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/standard"
	"github.com/cocosip/go-dicom-codec/jpegls/lossless"
	"github.com/cocosip/go-dicom-codec/jpegls/runmode"
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
		return nil, standard.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, standard.ErrInvalidComponents
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
	writer := standard.NewWriter(&buf)

	// Write SOI marker
	if err := writer.WriteMarker(standard.MarkerSOI); err != nil {
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
	if err := writer.WriteMarker(standard.MarkerEOI); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeSOF55 writes Start of Frame marker for JPEG-LS
// Format matches CharLS jpeg_stream_writer.cpp:96
// SOF55 data: 6 (fixed header) + components*3 (component specs)
func (enc *Encoder) writeSOF55(writer *standard.Writer) error {
	length := 6 + enc.components*3 // Fixed: was 8, should be 6
	data := make([]byte, length)

	data[0] = byte(enc.bitDepth)      // P = Sample precision
	data[1] = byte(enc.height >> 8)   // Y = Number of lines (MSB)
	data[2] = byte(enc.height & 0xFF) // Y = Number of lines (LSB)
	data[3] = byte(enc.width >> 8)    // X = Number of samples per line (MSB)
	data[4] = byte(enc.width & 0xFF)  // X = Number of samples per line (LSB)
	data[5] = byte(enc.components)    // Nf = Number of components

	// Component specifications (3 bytes per component)
	for i := 0; i < enc.components; i++ {
		offset := 6 + i*3
		data[offset] = byte(i + 1) // Component ID (1-based)
		data[offset+1] = 0x11      // Sampling factors (H=1, V=1)
		data[offset+2] = 0         // Quantization table (not used in JPEG-LS)
	}

	return writer.WriteSegment(0xFFF7, data)
}

// writeLSE writes JPEG-LS parameters (LSE marker) with NEAR
// Format matches CharLS jpeg_stream_writer.cpp:149-158
// LSE segment data: 11 bytes = 1 (ID) + 5*2 (five uint16 values)
func (enc *Encoder) writeLSE(writer *standard.Writer) error {
	data := make([]byte, 11) // ID (1) + MAXVAL (2) + T1 (2) + T2 (2) + T3 (2) + RESET (2)
	data[0] = 1              // ID = 1 (preset parameters)

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

	return writer.WriteSegment(0xFFF8, data)
}

// writeSOS writes Start of Scan marker with NEAR parameter
func (enc *Encoder) writeSOS(writer *standard.Writer) error {
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

	return writer.WriteSegment(standard.MarkerSOS, data)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *standard.Writer, pixelData []byte) error {
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

	prevFirstPrev := 0 // previous line first pixel
	prevNeg1 := 0      // previous_line[-1] (line-2 first pixel)

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
			var a, b, c, d int
			if x == 0 {
				a = prevFirstPrev
				b = 0
				if y > 0 {
					b = prevFirstPrev
				}
				c = prevNeg1
				if y > 0 && enc.width > 1 {
					rdIdx := ((y-1)*enc.width+(x+1))*stride + offset
					if rdIdx < len(pixels) {
						d = pixels[rdIdx]
					} else {
						d = b
					}
				} else {
					d = b
				}
			} else {
				a, b, c, d = enc.getNeighbors(pixels, x, y, comp)
			}

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
				pixelsProcessed, err := enc.doRunMode(gw, pixels, x, y, comp, a)
				if err != nil {
					return err
				}
				x += pixelsProcessed
			}
		}

		firstIdx := (y*enc.width+0)*stride + offset
		if firstIdx < len(pixels) {
			nextFirst := pixels[firstIdx]
			prevNeg1 = prevFirstPrev
			prevFirstPrev = nextFirst
		}
	}

	return nil
}

// computeErrorValue delegates to Traits.ComputeErrorValue
// CharLS traits_.compute_error_value(e) = modulo_range(quantize(e))
func (enc *Encoder) computeErrorValue(e int) int {
	return enc.traits.ComputeErrorValue(e)
}

// correctPrediction delegates to Traits.CorrectPrediction
// CharLS: default_traits.h correct_prediction() line 83-89
func (enc *Encoder) correctPrediction(predicted int) int {
	return enc.traits.CorrectPrediction(predicted)
}

// getNeighbors gets neighboring pixels following CharLS edge handling
// CharLS initializes edge pixels as:
//
//	current_line_[-1] = previous_line_[0]  (left edge = pixel above)
//	previous_line_[width_] = previous_line_[width_ - 1] (right padding = rightmost top pixel)
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
func (enc *Encoder) doRunMode(gw *lossless.GolombWriter, pixels []int, x, y, comp int, ra int) (int, error) {
	stride := enc.components
	offset := comp

	startIdx := y*enc.width + x
	remainingInLine := enc.width - x

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
		if runmode.Abs(currPixel-ra) <= enc.near {
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
	if runmode.Abs(ra-rb) <= enc.near {
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
	errorValue := enc.computeErrorValue((x - rb) * runmode.Sign(rb-ra))

	if err := enc.runModeScanner.EncodeRunInterruption(gw, enc.runModeScanner.RunModeContexts[0], errorValue); err != nil {
		return 0, err
	}

	// CharLS: return traits_.compute_reconstructed_sample(rb, error_value * sign(rb - ra))
	reconstructed := enc.traits.ComputeReconstructedSample(rb, errorValue*runmode.Sign(rb-ra))
	return reconstructed, nil
}
