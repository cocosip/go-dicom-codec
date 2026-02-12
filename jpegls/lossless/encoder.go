package lossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/standard"
	"github.com/cocosip/go-dicom-codec/jpegls/runmode"
)

// Encoder represents a JPEG-LS lossless encoder
type Encoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int // Maximum sample value (2^bitDepth - 1)

	traits         Traits
	contextTable   *ContextTable
	quantizer      *GradientQuantizer
	runModeScanner *RunModeScanner // Manages run mode state and operations
}

// NewEncoder creates a new JPEG-LS encoder
func NewEncoder(width, height, components, bitDepth int) *Encoder {
	maxVal := (1 << uint(bitDepth)) - 1
	traits := NewTraits(maxVal, 0, 64)

	return &Encoder{
		width:          width,
		height:         height,
		components:     components,
		bitDepth:       bitDepth,
		maxVal:         maxVal,
		traits:         traits,
		contextTable:   NewContextTable(maxVal, 0, traits.Reset),
		quantizer:      NewGradientQuantizer(traits.T1, traits.T2, traits.T3, traits.Near),
		runModeScanner: NewRunModeScanner(traits),
	}
}

// Encode encodes pixel data to JPEG-LS format
// pixelData: raw pixel values (interleaved for multi-component)
// Returns: JPEG-LS compressed data
func Encode(pixelData []byte, width, height, components, bitDepth int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, standard.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, standard.ErrInvalidComponents
	}

	if bitDepth < 2 || bitDepth > 16 {
		return nil, fmt.Errorf("invalid bit depth: %d (must be 2-16)", bitDepth)
	}

	encoder := NewEncoder(width, height, components, bitDepth)
	return encoder.encode(pixelData)
}

// computeErrorValue implements CharLS compute_error_value: narrow to sample precision.
func (enc *Encoder) computeErrorValue(delta int) int {
	if enc.bitDepth <= 8 {
		return int(int8(delta))
	}
	return int(int16(delta))
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
	if err := writer.WriteMarker(standard.MarkerEOI); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeSOF55 writes Start of Frame marker for JPEG-LS
func (enc *Encoder) writeSOF55(writer *standard.Writer) error {
	// SOF55 = 0xFFF7
	// Data length = 6 (fixed header) + components*3 (component specs)
	// Note: WriteSegment adds the 2-byte length field automatically
	length := 6 + enc.components*3
	data := make([]byte, length)

	data[0] = byte(enc.bitDepth)      // Precision
	data[1] = byte(enc.height >> 8)   // Height MSB
	data[2] = byte(enc.height & 0xFF) // Height LSB
	data[3] = byte(enc.width >> 8)    // Width MSB
	data[4] = byte(enc.width & 0xFF)  // Width LSB
	data[5] = byte(enc.components)    // Number of components

	// Component specifications
	for i := 0; i < enc.components; i++ {
		offset := 6 + i*3
		data[offset] = byte(i + 1) // Component ID
		data[offset+1] = 0x11      // Sampling factors (1x1)
		data[offset+2] = 0         // Quantization table (not used in lossless)
	}

	return writer.WriteSegment(0xFFF7, data)
}

// writeLSE writes JPEG-LS parameters (LSE marker)
func (enc *Encoder) writeLSE(writer *standard.Writer) error {
	// LSE = 0xFFF8
	// Write default parameters (11 bytes of data, WriteSegment adds 2-byte length field)
	data := make([]byte, 11)
	data[0] = 1 // ID = 1 (preset parameters)

	// MAXVAL (maximum sample value)
	maxVal := uint16(enc.maxVal)
	data[1] = byte(maxVal >> 8)
	data[2] = byte(maxVal & 0xFF)

	// T1, T2, T3 (thresholds) from traits to keep encode/decode in sync
	t1 := uint16(enc.traits.T1)
	t2 := uint16(enc.traits.T2)
	t3 := uint16(enc.traits.T3)

	data[3] = byte(t1 >> 8)
	data[4] = byte(t1 & 0xFF)
	data[5] = byte(t2 >> 8)
	data[6] = byte(t2 & 0xFF)
	data[7] = byte(t3 >> 8)
	data[8] = byte(t3 & 0xFF)

	// RESET interval
	reset := uint16(enc.traits.Reset)
	data[9] = byte(reset >> 8)
	data[10] = byte(reset & 0xFF)

	return writer.WriteSegment(0xFFF8, data)
}

// writeSOS writes Start of Scan marker
func (enc *Encoder) writeSOS(writer *standard.Writer) error {
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

	return writer.WriteSegment(standard.MarkerSOS, data)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *standard.Writer, pixelData []byte) error {
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
// This matches CharLS do_line encoding logic with RUN mode support
func (enc *Encoder) encodeComponent(gw *GolombWriter, pixels []int, comp int) error {
	stride := enc.components
	offset := comp

	// Track edge state to mimic CharLS current_line[-1]/previous_line[-1].
	prevFirstPrev := 0 // first pixel of previous line (y-1), or 0 when y=0
	prevNeg1 := 0      // stored value for rc when x=0: line (y-2) first pixel, or 0

	// Process line by line
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

			xSample := pixels[idx] // Current sample value

			// Get neighboring pixels (ra=left, rb=top, rc=top-left, rd=top-right)
			var ra, rb, rc, rd int
			if x == 0 {
				// CharLS: current_line[-1] = previous_line[0]; previous_line[-1] carries line-2 first pixel.
				ra = prevFirstPrev
				rb = 0
				if y > 0 {
					rb = prevFirstPrev
				}
				rc = prevNeg1
				if y > 0 && enc.width > 1 {
					rdIdx := ((y-1)*enc.width+(x+1))*stride + offset
					if rdIdx < len(pixels) {
						rd = pixels[rdIdx]
					} else {
						rd = rb
					}
				} else {
					rd = rb
				}
			} else {
				ra, rb, rc, rd = enc.getNeighbors(pixels, x, y, comp)
			}

			// Compute prediction using MED predictor
			predicted := Predict(ra, rb, rc)

			// Compute context ID
			q1, q2, q3 := enc.quantizer.ComputeContext(ra, rb, rc, rd)
			qs := ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode - matches CharLS do_regular
				sign := BitwiseSign(qs)
				contextIdx := ApplySign(qs, sign)

				// Get context with bounds check
				if contextIdx < 0 || contextIdx >= len(enc.contextTable.contexts) {
					return fmt.Errorf("context index %d out of bounds [0, %d)", contextIdx, len(enc.contextTable.contexts))
				}
				ctx := enc.contextTable.contexts[contextIdx]

				// Get Golomb parameter
				k := ctx.ComputeGolombParameter()

				// Apply prediction correction (CharLS: traits_.correct_prediction(predicted + apply_sign(context.c(), sign)))
				correctionC := ApplySign(ctx.C, sign)
				predictedValue := enc.traits.CorrectPrediction(predicted + correctionC)

				// Compute error value (CharLS: traits_.compute_error_value(apply_sign(x - predicted_value, sign)))
				// CRITICAL: Sign must be applied BEFORE modulo/wrapping, not after!
				rawErr := xSample - predictedValue
				signedErr := ApplySign(rawErr, sign)
				errorValue := enc.computeErrorValue(signedErr)

				// Apply error correction and map (CharLS: map_error_value(context.get_error_correction(k | NEAR) ^ error_value))
				errorCorrection := ctx.GetErrorCorrection(k, 0) // k|0 = k for lossless
				correctedError := errorCorrection ^ errorValue
				mappedError := MapErrorValue(correctedError)

				// In REGULAR MODE, use limit directly (J[RunIndex] is only for RUN MODE)
				// Encode mapped error
				if err := gw.EncodeMappedValue(k, mappedError, enc.traits.Limit, enc.traits.Qbpp); err != nil {
					return err
				}

				// Update context
				ctx.UpdateContext(errorValue, 0, enc.traits.Reset)

				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := enc.doRunMode(gw, pixels, x, y, comp, ra)
				if err != nil {
					return err
				}
				x += pixelsProcessed
			}
		}

		// Update edge state for next line
		firstIdx := (y*enc.width+0)*stride + offset
		if firstIdx < len(pixels) {
			nextFirst := pixels[firstIdx]
			prevNeg1 = prevFirstPrev
			prevFirstPrev = nextFirst
		}
	}

	return nil
}

// encodeRunInterruptionPixel encodes the pixel that interrupts a run (CharLS: encode_run_interruption_pixel)
func (enc *Encoder) encodeRunInterruptionPixel(gw *GolombWriter, x, ra, rb int) (int, error) {
	if runmode.Abs(ra-rb) <= enc.traits.Near {
		// Use run mode context 1
		errorValue := enc.computeErrorValue(x - ra)
		if err := enc.runModeScanner.EncodeRunInterruption(gw, enc.runModeScanner.RunModeContexts[1], errorValue); err != nil {
			return 0, err
		}
		return enc.traits.ComputeReconstructedSample(ra, errorValue), nil
	}
	// Use run mode context 0
	errorValue := enc.computeErrorValue((x - rb) * signInt(rb-ra))
	if err := enc.runModeScanner.EncodeRunInterruption(gw, enc.runModeScanner.RunModeContexts[0], errorValue); err != nil {
		return 0, err
	}
	return enc.traits.ComputeReconstructedSample(rb, errorValue*signInt(rb-ra)), nil
}

// doRunMode handles encoding in run mode (when qs == 0)
// This matches CharLS do_run_mode for encoder
func (enc *Encoder) doRunMode(gw *GolombWriter, pixels []int, x, y, comp int, ra int) (int, error) {
	stride := enc.components
	offset := comp

	startIdx := y*enc.width + x
	remainingInLine := enc.width - x

	// Count run length - CharLS: while (traits_.is_near(type_cur_x[run_length], ra))
	runLength := 0
	for runLength < remainingInLine {
		currIdx := (startIdx+runLength)*stride + offset
		if currIdx >= len(pixels) {
			break
		}

		currPixel := pixels[currIdx]

		// In lossless mode, is_near means exact equality
		if currPixel != ra {
			break
		}

		// Set pixel to ra (for reconstruction)
		pixels[currIdx] = ra
		runLength++
	}

	// Encode run length using RunModeScanner - CharLS: encode_run_pixels(run_length, run_length == count_type_remain)
	endOfLine := (runLength == remainingInLine)
	if err := enc.runModeScanner.EncodeRunLength(gw, runLength, endOfLine); err != nil {
		return 0, err
	}

	if endOfLine {
		return runLength, nil
	}

	// Handle run interruption - CharLS: type_cur_x[run_length] = encode_run_interruption_pixel(...)
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

	// Decrement run index - CharLS: decrement_run_index()
	enc.runModeScanner.DecRunIndex()

	return runLength + 1, nil
}

// getNeighbors gets the neighboring pixels for prediction
// Returns: a (left), b (top), c (top-left), d (top-right)
func (enc *Encoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := enc.components
	offset := comp

	// Default values for edges
	a, b, c, d := 0, 0, 0, 0

	// a = left pixel (West)
	// CharLS: current_line_[-1] is initialized to previous_line_[0] for first pixel of row
	if x > 0 {
		idx := (y*enc.width+(x-1))*stride + offset
		if idx < len(pixels) {
			a = pixels[idx]
		}
	} else if y > 0 {
		// Special case: x=0, y>0 - CharLS uses previous_line_[0] as current_line_[-1]
		idx := ((y-1)*enc.width+0)*stride + offset
		if idx < len(pixels) {
			a = pixels[idx]
		}
	}

	// b = top pixel (North)
	if y > 0 {
		idx := ((y-1)*enc.width+x)*stride + offset
		if idx < len(pixels) {
			b = pixels[idx]
		}
	}

	// c = top-left pixel (North-West)
	// CharLS: when x=0, rc = previous_line_[-1], which gets updated at END of each row to current_line_[0]
	// So for row y at x=0: previous_line_[-1] = current_line_[0] from row y-1 = pixels[y-1, 0]
	if x > 0 && y > 0 {
		idx := ((y-1)*enc.width+(x-1))*stride + offset
		if idx < len(pixels) {
			c = pixels[idx]
		}
	} else if x == 0 && y > 0 {
		// Special case: x=0, y>0 - CharLS uses previous_line_[-1] which is kept at 0
		c = 0
	}

	// d = top-right pixel (North-East)
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
// JPEG-LS operates on unsigned values in the range [0, MAXVAL]
// For signed data, we must first convert to unsigned by adding an offset
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
	// Read as int16 to properly handle signed data, then convert to unsigned range
	numPixels := len(pixelData) / 2
	pixels := make([]int, numPixels)
	for i := 0; i < numPixels; i++ {
		idx := i * 2
		// Read as little-endian uint16 first
		val := uint16(pixelData[idx]) | (uint16(pixelData[idx+1]) << 8)

		// JPEG-LS uses unsigned values [0, MAXVAL]
		// The data is already in the correct unsigned representation
		// (signed values are stored in two's complement, which maps correctly to unsigned range)
		pixels[i] = int(val)
	}
	return pixels
}
