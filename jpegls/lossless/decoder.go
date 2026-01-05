package lossless

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
)

// Decoder represents a JPEG-LS lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int
	traits     Traits

	contextTable   *ContextTable
	quantizer      *GradientQuantizer
	runModeScanner *RunModeScanner
}

// NewDecoder creates a new JPEG-LS decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// computeErrorValue mirrors encoder compute_error_value: for Near=0 use unsigned wrap.
func (dec *Decoder) computeErrorValue(delta int) int {
	if dec.bitDepth <= 8 {
		return int(int8(delta))
	}
	return int(int16(delta))
}

// Decode decodes JPEG-LS compressed data
// Returns: pixelData, width, height, components, bitDepth, error
func Decode(jpegLSData []byte) ([]byte, int, int, int, int, error) {
	decoder := NewDecoder()
	return decoder.decode(jpegLSData)
}

// decode performs the actual decoding
func (dec *Decoder) decode(jpegLSData []byte) ([]byte, int, int, int, int, error) {
	r := bytes.NewReader(jpegLSData)
	reader := common.NewReader(r)

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if marker != common.MarkerSOI {
		return nil, 0, 0, 0, 0, common.ErrInvalidSOI
	}

	// Parse segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, 0, 0, err
		}

		switch marker {
		case 0xFFF7: // SOF55 (JPEG-LS)
			if err := dec.parseSOF55(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case 0xFFF8: // LSE (JPEG-LS parameters)
			if err := dec.parseLSE(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case common.MarkerSOS:
			if err := dec.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

			// Decode scan data
			pixelData, err := dec.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}

			return pixelData, dec.width, dec.height, dec.components, dec.bitDepth, nil

		case common.MarkerEOI:
			return nil, 0, 0, 0, 0, fmt.Errorf("unexpected EOI before scan data")

		default:
			// Skip unknown markers
			if common.HasLength(marker) {
				_, err := reader.ReadSegment()
				if err != nil {
					return nil, 0, 0, 0, 0, err
				}
			}
		}
	}

	return nil, 0, 0, 0, 0, fmt.Errorf("incomplete JPEG-LS data")
}

// parseSOF55 parses the SOF55 segment
func (dec *Decoder) parseSOF55(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 6 {
		return common.ErrInvalidSOF
	}

	dec.bitDepth = int(data[0])
	dec.height = int(data[1])<<8 | int(data[2])
	dec.width = int(data[3])<<8 | int(data[4])
	dec.components = int(data[5])

	if dec.width <= 0 || dec.height <= 0 {
		return common.ErrInvalidDimensions
	}

	if dec.components != 1 && dec.components != 3 {
		return common.ErrInvalidComponents
	}

	dec.maxVal = (1 << uint(dec.bitDepth)) - 1
	dec.traits = NewTraits(dec.maxVal, 0, 64)
	dec.initCodingParameters(0, 0, 0)

	return nil
}

// initCodingParameters recomputes derived parameters and contexts (legacy/bitDepth-based).
func (dec *Decoder) initCodingParameters(t1, t2, t3 int) {
	params := ComputeCodingParameters(dec.maxVal, 0, dec.traits.Reset)
	if t1 == 0 || t2 == 0 || t3 == 0 {
		t1, t2, t3 = params.T1, params.T2, params.T3
	}

	dec.traits = NewTraits(dec.maxVal, 0, params.Reset)
	dec.quantizer = NewGradientQuantizer(t1, t2, t3, dec.traits.Near)
	dec.contextTable = NewContextTable(dec.maxVal, 0, dec.traits.Reset)
	dec.runModeScanner = NewRunModeScanner(dec.traits)
}

// parseLSE parses the LSE segment (JPEG-LS parameters)
func (dec *Decoder) parseLSE(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return fmt.Errorf("invalid LSE segment")
	}

	// ID byte
	id := data[0]

	if id == 1 {
		// Preset parameters
		// Expected: 11 bytes of data (ID + MAXVAL + T1 + T2 + T3 + RESET)
		if len(data) < 11 {
			return fmt.Errorf("invalid LSE preset parameters")
		}

		// Read MAXVAL
		maxVal := int(data[1])<<8 | int(data[2])
		t1 := int(data[3])<<8 | int(data[4])
		t2 := int(data[5])<<8 | int(data[6])
		t3 := int(data[7])<<8 | int(data[8])
		reset := int(data[9])<<8 | int(data[10])

		if maxVal <= 0 {
			maxVal = dec.maxVal
		}
		dec.maxVal = maxVal
		if reset == 0 {
			reset = 64
		}
		dec.traits.Reset = reset
		dec.initCodingParameters(t1, t2, t3)
	}

	return nil
}

// parseSOS parses the SOS segment
func (dec *Decoder) parseSOS(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return common.ErrInvalidSOS
	}

	numComponents := int(data[0])
	if numComponents != dec.components {
		return fmt.Errorf("SOS component count mismatch")
	}

	return nil
}

// decodeScan decodes the scan data
func (dec *Decoder) decodeScan(reader *common.Reader) ([]byte, error) {
	// Read all remaining data until EOI
	var scanData bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if b == 0xFF {
			b2, err := reader.ReadByte()
			if err != nil {
				if err == io.EOF {
					scanData.WriteByte(b)
					break
				}
				return nil, err
			}

			if b2 == 0x00 {
				// stuffed 0xFF
				scanData.WriteByte(0xFF)
				continue
			}
			if b2 == 0xD9 {
				break
			}
			// restart markers: ignore marker, continue decoding data after it
			continue
		}
		scanData.WriteByte(b)
	}

	// Create Golomb reader
	gr := NewGolombReader(bytes.NewReader(scanData.Bytes()))

	// Allocate pixel array
	totalPixels := dec.width * dec.height * dec.components
	pixels := make([]int, totalPixels)

	// Decode each component
	for comp := 0; comp < dec.components; comp++ {
		if err := dec.decodeComponent(gr, pixels, comp); err != nil {
			return nil, err
		}
	}

	// Convert integers to bytes
	return dec.integersToPixels(pixels), nil
}

// decodeComponent decodes a single component with RUN mode support
func (dec *Decoder) decodeComponent(gr *GolombReader, pixels []int, comp int) error {
	stride := dec.components
	offset := comp

	// Process line by line
	for y := 0; y < dec.height; y++ {
		// Reset run index at start of each line (JPEG-LS standard)
		dec.runModeScanner.ResetLine()

		x := 0
		for x < dec.width {
			idx := (y*dec.width+x)*stride + offset

			// Get neighboring pixels (ra=left, rb=top, rc=top-left, rd=top-right)
			ra, rb, rc, rd := dec.getNeighbors(pixels, x, y, comp)

			// Compute prediction
			predicted := Predict(ra, rb, rc)

			// Compute context ID with sign symmetry
			q1, q2, q3 := dec.quantizer.ComputeContext(ra, rb, rc, rd)
			qs := ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode - matches CharLS do_regular for decoder

				// Extract sign and apply sign symmetry
				sign := BitwiseSign(qs)
				contextIdx := ApplySign(qs, sign)

				// Get context with bounds check
				if contextIdx < 0 || contextIdx >= len(dec.contextTable.contexts) {
					return fmt.Errorf("context index %d out of bounds [0, %d)", contextIdx, len(dec.contextTable.contexts))
				}
				ctx := dec.contextTable.contexts[contextIdx]

				// Get Golomb parameter
				k := ctx.ComputeGolombParameter()

				// Apply prediction correction (CharLS: traits_.correct_prediction(predicted + apply_sign(context.c(), sign)))
				correctionC := ApplySign(ctx.C, sign)
				predicted_value := dec.traits.CorrectPrediction(predicted + correctionC)

				limitMinusJ := dec.traits.Limit - J[dec.runModeScanner.RunIndex] - 1
				if limitMinusJ < 0 {
					limitMinusJ = 0
				}

				mapped_error, err := gr.DecodeValue(k, limitMinusJ, dec.traits.Qbpp)
				if err != nil {
					return fmt.Errorf("decode regular at x=%d y=%d comp=%d (bits=%d): %w", x, y, comp, gr.bitsRead, err)
				}

				// Unmap error (before sign)
				error_value := UnmapErrorValue(mapped_error)
				if k == 0 {
					error_value ^= ctx.GetErrorCorrection(k, 0)
				}

				// Update context using unsigned error (before sign)
				ctx.UpdateContext(error_value, 0, dec.traits.Reset)

				// Apply sign to error and reconstruct
				signedError := ApplySign(error_value, sign)
				reconstructed := dec.traits.ComputeReconstructedSample(predicted_value, signedError)

				pixels[idx] = reconstructed
				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := dec.doRunMode(gr, pixels, x, y, comp)
				if err != nil {
					return fmt.Errorf("decode run at x=%d y=%d comp=%d (bits=%d): %w", x, y, comp, gr.bitsRead, err)
				}
				x += pixelsProcessed
			}
		}
	}

	return nil
}

// decodeRunInterruptionPixel decodes the pixel that interrupts a run (CharLS: decode_run_interruption_pixel)
func (dec *Decoder) decodeRunInterruptionPixel(gr *GolombReader, ra, rb int) (int, error) {
	if abs(ra-rb) <= dec.traits.Near {
		// Use run mode context 1
		errorValue, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[1])
		if err != nil {
			return 0, err
		}
		errorValue = dec.computeErrorValue(errorValue)
		return dec.traits.ComputeReconstructedSample(ra, errorValue), nil
	}

	// Use run mode context 0
	errorValue, err := dec.runModeScanner.DecodeRunInterruption(gr, dec.runModeScanner.RunModeContexts[0])
	if err != nil {
		return 0, err
	}
	errorValue = dec.computeErrorValue(errorValue * signInt(rb-ra))
	return dec.traits.ComputeReconstructedSample(rb, errorValue), nil
}

// doRunMode handles decoding in run mode (when qs == 0) - CharLS: do_run_mode for decoder
func (dec *Decoder) doRunMode(gr *GolombReader, pixels []int, x, y, comp int) (int, error) {
	stride := dec.components
	offset := comp

	startIdx := y*dec.width + x
	remainingInLine := dec.width - x

	// Get ra (left pixel) - CharLS: pixel_type ra{current_line_[start_index - 1]}
	raIdx := (startIdx-1)*stride + offset
	ra := 0
	if raIdx >= 0 && raIdx < len(pixels) {
		ra = pixels[raIdx]
	}

	// Decode run length using RunModeScanner - CharLS: decode_run_pixels(ra, current_line_ + start_index, width_ - start_index)
	runLength, err := dec.runModeScanner.DecodeRunLength(gr, remainingInLine)
	if err != nil {
		return 0, fmt.Errorf("runLength (bits=%d): %w", gr.bitsRead, err)
	}

	// Fill run with ra value
	for i := 0; i < runLength; i++ {
		idx := (startIdx+i)*stride + offset
		if idx < len(pixels) {
			pixels[idx] = ra
		}
	}

	// Check if run reaches end of line
	if runLength >= remainingInLine {
		return runLength, nil
	}

	// Handle run interruption - CharLS: current_line_[end_index] = decode_run_interruption_pixel(ra, rb)
	interruptIdx := (startIdx+runLength)*stride + offset

	// Get rb (top pixel at interruption point)
	rbIdx := ((y-1)*dec.width+(x+runLength))*stride + offset
	rb := 0
	if rbIdx >= 0 && rbIdx < len(pixels) {
		rb = pixels[rbIdx]
	}

	// Decode interruption pixel
	reconstructed, err := dec.decodeRunInterruptionPixel(gr, ra, rb)
	if err != nil {
		return 0, fmt.Errorf("runInterruption (bits=%d): %w", gr.bitsRead, err)
	}

	// Store reconstructed value
	if interruptIdx < len(pixels) {
		pixels[interruptIdx] = reconstructed
	}

	// Decrement run index - CharLS: decrement_run_index()
	dec.runModeScanner.DecRunIndex()

	return runLength + 1, nil
}

// getNeighbors gets neighboring pixels for prediction
func (dec *Decoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := dec.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	if x > 0 {
		idx := (y*dec.width+(x-1))*stride + offset
		a = pixels[idx]
	}

	if y > 0 {
		idx := ((y-1)*dec.width+x)*stride + offset
		b = pixels[idx]
	}

	if x > 0 && y > 0 {
		idx := ((y-1)*dec.width+(x-1))*stride + offset
		c = pixels[idx]
	}

	if x < dec.width-1 && y > 0 {
		idx := ((y-1)*dec.width+(x+1))*stride + offset
		d = pixels[idx]
	}

	return a, b, c, d
}

// integersToPixels converts integer array to pixel bytes
func (dec *Decoder) integersToPixels(pixels []int) []byte {
	if dec.bitDepth <= 8 {
		// 8-bit: one byte per sample
		pixelData := make([]byte, len(pixels))
		for i, val := range pixels {
			if val < 0 {
				val = 0
			} else if val > dec.maxVal {
				val = dec.maxVal
			}
			pixelData[i] = byte(val)
		}
		return pixelData
	}

	// 9-16 bit: two bytes per sample (little-endian)
	pixelData := make([]byte, len(pixels)*2)
	for i, val := range pixels {
		if val < 0 {
			val = 0
		} else if val > dec.maxVal {
			val = dec.maxVal
		}
		idx := i * 2
		pixelData[idx] = byte(val & 0xFF)
		pixelData[idx+1] = byte((val >> 8) & 0xFF)
	}
	return pixelData
}
