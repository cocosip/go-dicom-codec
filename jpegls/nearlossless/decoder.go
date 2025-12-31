package nearlossless

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
	jpegcommon "github.com/cocosip/go-dicom-codec/jpegls/common"
	"github.com/cocosip/go-dicom-codec/jpegls/lossless"
)

// Decoder represents a JPEG-LS near-lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	bitDepth   int
	maxVal     int
	near       int // NEAR parameter
	qbpp       int
	range_     int
	t1         int
	t2         int
	t3         int

	contextTable    *lossless.ContextTable
	runModeContexts [2]*lossless.RunModeContext // Two run mode contexts (index 0 and 1)
	runIndex        int                         // Current run index (0-31)
	limit           int
	reset           int
	quantizer       *lossless.GradientQuantizer
}

// NewDecoder creates a new JPEG-LS near-lossless decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes JPEG-LS near-lossless compressed data
func Decode(jpegLSData []byte) ([]byte, int, int, int, int, int, error) {
	decoder := NewDecoder()
	return decoder.decode(jpegLSData)
}

// decode performs the actual decoding
func (dec *Decoder) decode(jpegLSData []byte) ([]byte, int, int, int, int, int, error) {
	r := bytes.NewReader(jpegLSData)
	reader := common.NewReader(r)

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, 0, err
	}
	if marker != common.MarkerSOI {
		return nil, 0, 0, 0, 0, 0, common.ErrInvalidSOI
	}

	// Parse segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, 0, 0, 0, err
		}

		switch marker {
		case 0xFFF7: // SOF55 (JPEG-LS)
			if err := dec.parseSOF55(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

		case 0xFFF8: // LSE (JPEG-LS parameters)
			if err := dec.parseLSE(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

		case common.MarkerSOS:
			if err := dec.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

			// Decode scan data
			pixelData, err := dec.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, 0, err
			}

			return pixelData, dec.width, dec.height, dec.components, dec.bitDepth, dec.near, nil

		case common.MarkerEOI:
			return nil, 0, 0, 0, 0, 0, fmt.Errorf("unexpected EOI before scan data")

		default:
			if common.HasLength(marker) {
				_, err := reader.ReadSegment()
				if err != nil {
					return nil, 0, 0, 0, 0, 0, err
				}
			}
		}
	}

	return nil, 0, 0, 0, 0, 0, fmt.Errorf("incomplete JPEG-LS data")
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
	dec.reset = 64

	return nil
}

// parseLSE parses the LSE segment
func (dec *Decoder) parseLSE(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return fmt.Errorf("invalid LSE segment")
	}

	id := data[0]
	if id == 1 && len(data) >= 13 {
		// Read MAXVAL
		maxVal := int(data[1])<<8 | int(data[2])
		if maxVal > 0 {
			dec.maxVal = maxVal
		}

		dec.t1 = int(data[3])<<8 | int(data[4])
		dec.t2 = int(data[5])<<8 | int(data[6])
		dec.t3 = int(data[7])<<8 | int(data[8])

		// RESET interval
		dec.reset = int(data[9])<<8 | int(data[10])
		if dec.reset == 0 {
			dec.reset = 64
		}
	}

	return nil
}

// applyCodingParameters recomputes derived parameters once NEAR is known.
func (dec *Decoder) applyCodingParameters() {
	params := lossless.ComputeCodingParameters(dec.maxVal, dec.near, dec.reset)
	if dec.t1 > 0 {
		params.T1 = dec.t1
	}
	if dec.t2 > 0 {
		params.T2 = dec.t2
	}
	if dec.t3 > 0 {
		params.T3 = dec.t3
	}
	if dec.reset > 0 {
		params.Reset = dec.reset
	}

	dec.qbpp = params.Qbpp
	dec.range_ = params.Range
	dec.limit = params.Limit
	dec.quantizer = lossless.NewGradientQuantizer(params.T1, params.T2, params.T3, dec.near)
	dec.contextTable = lossless.NewContextTable(dec.maxVal, dec.near, params.Reset)
	dec.runModeContexts = [2]*lossless.RunModeContext{
		lossless.NewRunModeContext(0, params.Range),
		lossless.NewRunModeContext(1, params.Range),
	}
	dec.runIndex = 0
}

// parseSOS parses the SOS segment and extracts NEAR parameter
func (dec *Decoder) parseSOS(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 4 {
		return common.ErrInvalidSOS
	}

	numComponents := int(data[0])
	if numComponents != dec.components {
		return fmt.Errorf("SOS component count mismatch")
	}

	// Extract NEAR parameter (at position len-3)
	dec.near = int(data[len(data)-3])

	// Compute quantization parameters and contexts using NEAR + LSE thresholds
	dec.applyCodingParameters()

	return nil
}

// decodeScan decodes the scan data
func (dec *Decoder) decodeScan(reader *common.Reader) ([]byte, error) {
	// Read scan data
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
				scanData.WriteByte(b)
			} else if b2 == 0xD9 {
				break
			} else {
				break
			}
		} else {
			scanData.WriteByte(b)
		}
	}

	// Create Golomb reader
	gr := lossless.NewGolombReader(bytes.NewReader(scanData.Bytes()))

	// Allocate pixel array
	totalPixels := dec.width * dec.height * dec.components
	pixels := make([]int, totalPixels)

	// Decode each component
	for comp := 0; comp < dec.components; comp++ {
		if err := dec.decodeComponent(gr, pixels, comp); err != nil {
			return nil, err
		}
	}

	return dec.integersToPixels(pixels), nil
}

// decodeComponent decodes a single component
func (dec *Decoder) decodeComponent(gr *lossless.GolombReader, pixels []int, comp int) error {
	stride := dec.components
	offset := comp

	for y := 0; y < dec.height; y++ {
		// Reset run index at start of each line (JPEG-LS standard)
		dec.runIndex = 0

		x := 0
		for x < dec.width {
			idx := (y*dec.width+x)*stride + offset

			// Get neighbors
			a, b, c, d := dec.getNeighbors(pixels, x, y, comp)

			// Compute context on ORIGINAL values (before quantization)
			// This ensures thresholds work correctly
			q1, q2, q3 := dec.quantizer.ComputeContext(a, b, c, d)

			// Compute context ID to check for RUN mode
			qs := lossless.ComputeContextID(q1, q2, q3)

			// Check if we should use RUN mode (qs == 0 means flat region)
			if qs != 0 {
				// Regular mode
				ctx := dec.contextTable.GetContext(q1, q2, q3)

				// Quantize neighbors for prediction
				qa := dec.quantize(a)
				qb := dec.quantize(b)
				qc := dec.quantize(c)

				// Compute prediction on quantized values
				prediction := lossless.Predict(qa, qb, qc)

				// Apply prediction correction using C value
				correctionC := ctx.GetPredictionCorrection()
				correctedPred := dec.correctPrediction(prediction, correctionC)

				// Reconstruct prediction to original range
				reconstructedPred := dec.dequantize(correctedPred)

				// Get Golomb parameter
				k := ctx.ComputeGolombParameter()

				// Decode error using limited alphabet
				mappedError, err := gr.DecodeValue(k, dec.limit, dec.qbpp)
				if err != nil {
					return err
				}

				// Unmap error to get error value (after modulo_range)
				errorValue := dec.unmapErrorValue(mappedError)

				// Reconstruct sample with wraparound handling (CharLS: fix_reconstructed_value)
				reconstructedError := dec.dequantizeError(errorValue)
				sample := dec.fixReconstructedValue(reconstructedPred + reconstructedError)

				pixels[idx] = sample

				// Update context with error value
				ctx.UpdateContext(errorValue, dec.near, dec.reset)

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

// quantize quantizes a value
func (dec *Decoder) quantize(val int) int {
	if dec.near == 0 {
		return val
	}
	return val / (2*dec.near + 1)
}

// dequantize reconstructs a value
func (dec *Decoder) dequantize(qval int) int {
	if dec.near == 0 {
		return qval
	}
	return qval * (2*dec.near + 1)
}

// fixReconstructedValue handles wraparound for reconstructed values
// According to CharLS implementation
func (dec *Decoder) fixReconstructedValue(value int) int {
	if value < -dec.near {
		value += dec.range_ * (2*dec.near + 1)
	} else if value > dec.maxVal+dec.near {
		value -= dec.range_ * (2*dec.near + 1)
	}

	// Clamp to valid range
	if value < 0 {
		value = 0
	} else if value > dec.maxVal {
		value = dec.maxVal
	}

	return value
}

// dequantizeError reconstructs error from quantized error
// According to CharLS implementation and JPEG-LS standard T.87
func (dec *Decoder) dequantizeError(qerr int) int {
	if dec.near == 0 {
		return qerr
	}

	// CharLS implementation: simple multiplication
	// The NEAR offset is handled by modulo_range operation, not here
	return qerr * (2*dec.near + 1)
}

// unmapErrorValue reverses error mapping
func (dec *Decoder) unmapErrorValue(mappedError int) int {
	if mappedError%2 == 0 {
		return mappedError / 2
	}
	return -(mappedError + 1) / 2
}

// correctPrediction applies bias correction
func (dec *Decoder) correctPrediction(prediction, bias int) int {
	prediction += bias

	// Clamp to quantized range
	if prediction < 0 {
		prediction = 0
	} else if prediction >= dec.range_ {
		prediction = dec.range_ - 1
	}

	return prediction
}

// getNeighbors gets neighboring pixels
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


// doRunMode handles decoding in run mode (when qs == 0) for near-lossless
func (dec *Decoder) doRunMode(gr *lossless.GolombReader, pixels []int, x, y, comp int) (int, error) {
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

	dec.runIndex = jpegcommon.DecrementRunIndex(dec.runIndex)

	return runLength + 1, nil
}

// decodeRunPixels decodes a run of identical pixels
func (dec *Decoder) decodeRunPixels(gr *lossless.GolombReader, ra int, remainingInLine int) (int, error) {
	runLength := 0

	// Read 1 bit at a time
	for {
		bit, err := gr.ReadBit()
		if err != nil {
			return runLength, err
		}

		if bit == 1 {
			// Full run segment
			count := jpegcommon.Min(1<<uint(lossless.J[dec.runIndex]), remainingInLine-runLength)
			runLength += count

			if count == (1 << uint(lossless.J[dec.runIndex])) {
				dec.runIndex = jpegcommon.IncrementRunIndex(dec.runIndex)
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
	if lossless.J[dec.runIndex] > 0 {
		val, err := gr.ReadBits(lossless.J[dec.runIndex])
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
func (dec *Decoder) decodeRunInterruptionPixel(gr *lossless.GolombReader, ra, rb int) (int, error) {
	if jpegcommon.Abs(ra-rb) <= dec.near {
		// Use run mode context 1
		quantizedError, err := dec.decodeRunInterruptionError(gr, dec.runModeContexts[1])
		if err != nil {
			return 0, err
		}

		// Dequantize error for reconstruction
		reconstructedError := dec.dequantizeError(quantizedError)
		reconstructed := ra + reconstructedError

		// Clamp to valid range
		if reconstructed < 0 {
			reconstructed = 0
		} else if reconstructed > dec.maxVal {
			reconstructed = dec.maxVal
		}

		return reconstructed, nil
	}

	// Use run mode context 0
	quantizedError, err := dec.decodeRunInterruptionError(gr, dec.runModeContexts[0])
	if err != nil {
		return 0, err
	}

	// Dequantize error for reconstruction
	reconstructedError := dec.dequantizeError(quantizedError)
	reconstructed := rb + reconstructedError*jpegcommon.Sign(rb-ra)

	// Clamp to valid range
	if reconstructed < 0 {
		reconstructed = 0
	} else if reconstructed > dec.maxVal {
		reconstructed = dec.maxVal
	}

	return reconstructed, nil
}

// decodeRunInterruptionError decodes the error value for run interruption
func (dec *Decoder) decodeRunInterruptionError(gr *lossless.GolombReader, ctx *lossless.RunModeContext) (int, error) {
	k := ctx.GetGolombCode()

	// Decode using limited alphabet Golomb
	limitMinusJ := dec.limit - lossless.J[dec.runIndex] - 1
	if limitMinusJ < 0 {
		limitMinusJ = 0
	}
	eMappedErrorValue, err := gr.DecodeValue(k, limitMinusJ, dec.qbpp)
	if err != nil {
		return 0, err
	}

	// Get the runInterruptionType value
	runInterruptionType := 0
	if ctx == dec.runModeContexts[1] {
		runInterruptionType = 1
	}

	// Reconstruct error value
	errorValue := ctx.ComputeErrorValue(eMappedErrorValue+runInterruptionType, k)

	// Update context
	ctx.UpdateVariables(errorValue, eMappedErrorValue, dec.reset)

	return errorValue, nil
}
