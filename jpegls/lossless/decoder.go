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

	contextTable          *ContextTable
	runModeContexts       [2]*RunModeContext
	runIndex              int
	resetThreshold        int
	limit                 int
	quantizedBitsPerPixel int
	quantizer             *GradientQuantizer
}

// NewDecoder creates a new JPEG-LS decoder
func NewDecoder() *Decoder {
	return &Decoder{}
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
	dec.resetThreshold = 64
	dec.initCodingParameters(0, 0, 0)

	return nil
}

// initCodingParameters recomputes derived parameters and contexts (legacy/bitDepth-based).
func (dec *Decoder) initCodingParameters(t1, t2, t3 int) {
	// Legacy qbpp/limit based on bit depth
	qbpp := dec.bitDepth
	if dec.bitDepth > 12 {
		qbpp = 16
	} else if dec.bitDepth > 8 {
		qbpp = 12
	} else {
		qbpp = 8
	}
	// LIMIT: larger value to avoid overflow of mapped errors; cap exponent to avoid overflow
	exp := qbpp + max(8, qbpp)
	if exp > 24 {
		exp = 24
	}
	limit := 1 << uint(exp)

	if t1 == 0 || t2 == 0 || t3 == 0 {
		t1, t2, t3 = computeThresholds(dec.maxVal, 0)
	}

	dec.limit = limit
	dec.quantizedBitsPerPixel = qbpp
	dec.quantizer = NewGradientQuantizer(t1, t2, t3, 0)

	range_ := dec.maxVal + 1
	dec.contextTable = NewContextTable(dec.maxVal, 0, dec.resetThreshold)
	dec.runModeContexts = [2]*RunModeContext{
		NewRunModeContext(0, range_),
		NewRunModeContext(1, range_),
	}
	dec.runIndex = 0
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
		if reset > 0 {
			dec.resetThreshold = reset
		}
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
			// Check next byte
			b2, err := reader.ReadByte()
			if err != nil {
				if err == io.EOF {
					scanData.WriteByte(b)
					break
				}
				return nil, err
			}

			if b2 == 0x00 {
				// Byte stuffing
				scanData.WriteByte(b)
			} else if b2 == 0xD9 {
				// EOI marker
				break
			} else {
				// Other marker, stop
				break
			}
		} else {
			scanData.WriteByte(b)
		}
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
		dec.runIndex = 0

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
				// Regular mode

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

				// Apply prediction correction with sign
				correctionC := ApplySign(ctx.C, sign)
				predicted_value := CorrectPrediction(predicted, correctionC, dec.maxVal+1)

				// Decode mapped error using CharLS decode_value with limit
				mapped_error, err := gr.DecodeValue(k, dec.limit, dec.quantizedBitsPerPixel)
				if err != nil {
					return err
				}

				// Unmap error
				corrected_error := UnmapErrorValue(mapped_error)

				// Apply error correction (XOR) - must ALWAYS be done, not just when k==0
				errorCorrection := ctx.GetErrorCorrection(k, 0)
				error_value := errorCorrection ^ corrected_error

				// Update context
				ctx.UpdateContext(error_value, 0, dec.resetThreshold)

				// Apply sign to error and reconstruct
				error_value = ApplySign(error_value, sign)
				reconstructed := predicted_value + error_value

				// Clamp to valid range
				if reconstructed < 0 {
					reconstructed += (dec.maxVal + 1)
				} else if reconstructed > dec.maxVal {
					reconstructed -= (dec.maxVal + 1)
				}

				pixels[idx] = reconstructed
				x++
			} else {
				// RUN mode (qs == 0, flat region)
				pixelsProcessed, err := dec.doRunMode(gr, pixels, x, y, comp)
				if err != nil {
					return err
				}
				x += pixelsProcessed
			}
		}
	}

	return nil
}

// incrementRunIndex increments the run index
func (dec *Decoder) incrementRunIndex() {
	if dec.runIndex < 31 {
		dec.runIndex++
	}
}

// decrementRunIndex decrements the run index
func (dec *Decoder) decrementRunIndex() {
	if dec.runIndex > 0 {
		dec.runIndex--
	}
}

// decodeRunPixels decodes a run of identical pixels
// This matches CharLS decode_run_pixels (scan.h line 715)
func (dec *Decoder) decodeRunPixels(gr *GolombReader, ra int, remainingInLine int) (int, error) {
	runLength := 0

	// Read 1 bit at a time (CharLS: while (Strategy::read_bit()))
	for {
		bit, err := gr.ReadBit()
		if err != nil {
			return runLength, err
		}

		if bit == 1 {
			// Full run segment
			count := min(1<<uint(J[dec.runIndex]), remainingInLine-runLength)
			runLength += count

			if count == (1 << uint(J[dec.runIndex])) {
				dec.incrementRunIndex()
			}

			if runLength >= remainingInLine {
				return remainingInLine, nil
			}
		} else {
			// Bit is 0: incomplete run
			break
		}
	}

	// Read remaining run length if J[runIndex] > 0
	// CharLS: index += (J[run_index_] > 0) ? Strategy::read_value(J[run_index_]) : 0;
	if J[dec.runIndex] > 0 {
		val, err := gr.ReadBits(J[dec.runIndex])
		if err != nil {
			return runLength, err
		}
		runLength += int(val)
	}

	if runLength > remainingInLine {
		return 0, fmt.Errorf("run length exceeds line: %d > %d", runLength, remainingInLine)
	}

	return runLength, nil
}

// decodeRunInterruptionPixel decodes the pixel that interrupts a run
func (dec *Decoder) decodeRunInterruptionPixel(gr *GolombReader, ra, rb int) (int, error) {
	const nearLossless = 0

	if abs(ra-rb) <= nearLossless {
		// Use run mode context 1
		errorValue, err := dec.decodeRunInterruptionError(gr, dec.runModeContexts[1])
		if err != nil {
			return 0, err
		}
		return ra + errorValue, nil
	}

	// Use run mode context 0
	errorValue, err := dec.decodeRunInterruptionError(gr, dec.runModeContexts[0])
	if err != nil {
		return 0, err
	}
	return rb + errorValue*sign(rb-ra), nil
}

// decodeRunInterruptionError decodes the error value for run interruption
func (dec *Decoder) decodeRunInterruptionError(gr *GolombReader, ctx *RunModeContext) (int, error) {
	k := ctx.GetGolombCode()

	// Decode using limited alphabet Golomb
	limitMinusJ := dec.limit - J[dec.runIndex] - 1
	if limitMinusJ < 0 {
		limitMinusJ = 0
	}
	eMappedErrorValue, err := dec.decodeGolombLimited(gr, k, limitMinusJ)
	if err != nil {
		return 0, err
	}

	// Reconstruct error value
	errorValue := ctx.ComputeErrorValue(eMappedErrorValue+ctx.runInterruptionType, k)

	// Update context
	ctx.UpdateVariables(errorValue, eMappedErrorValue, dec.resetThreshold)

	return errorValue, nil
}

// decodeGolombLimited decodes a value using Golomb coding with limited alphabet
func (dec *Decoder) decodeGolombLimited(gr *GolombReader, k, limit int) (int, error) {
	// Use DecodeValue with the limit parameter
	return gr.DecodeValue(k, limit, dec.quantizedBitsPerPixel)
}

// doRunMode handles decoding in run mode (when qs == 0)
func (dec *Decoder) doRunMode(gr *GolombReader, pixels []int, x, y, comp int) (int, error) {
	stride := dec.components
	offset := comp

	startIdx := y*dec.width + x
	remainingInLine := dec.width - x

	// Get ra (left pixel)
	raIdx := (startIdx-1)*stride + offset
	ra := 0
	if raIdx >= 0 && raIdx < len(pixels) {
		ra = pixels[raIdx]
	}

	// Decode run length
	runLength, err := dec.decodeRunPixels(gr, ra, remainingInLine)
	if err != nil {
		return 0, err
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
		dec.runIndex = 0
		return runLength, nil
	}

	// Handle run interruption
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
		return 0, err
	}

	// Store reconstructed value
	if interruptIdx < len(pixels) {
		pixels[interruptIdx] = reconstructed
	}

	dec.decrementRunIndex()

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
