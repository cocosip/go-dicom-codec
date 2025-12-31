package nearlossless

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
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

	contextTable *lossless.ContextTable
	runDecoder   *lossless.RunModeDecoder
	limit        int
	reset        int
	quantizer    *lossless.GradientQuantizer
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
	dec.runDecoder = lossless.NewRunModeDecoder()
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
		for x := 0; x < dec.width; x++ {
			idx := (y*dec.width+x)*stride + offset

			// Get neighbors
			a, b, c, d := dec.getNeighbors(pixels, x, y, comp)

			// Compute context on ORIGINAL values (before quantization)
			// This ensures thresholds work correctly
			q1, q2, q3 := dec.quantizer.ComputeContext(a, b, c, d)
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

			// Unmap error
			quantizedError := dec.unmapErrorValue(mappedError)

			// Reconstruct sample
			reconstructedError := dec.dequantizeError(quantizedError)
			sample := reconstructedPred + reconstructedError

			// Clamp to valid range
			if sample < 0 {
				sample = 0
			} else if sample > dec.maxVal {
				sample = dec.maxVal
			}

			pixels[idx] = sample

			// Update context with quantized error
			ctx.UpdateContext(quantizedError, dec.near, dec.reset)
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

// dequantizeError reconstructs error from quantized error
// According to JPEG-LS standard T.87
func (dec *Decoder) dequantizeError(qerr int) int {
	if dec.near == 0 {
		return qerr
	}

	// JPEG-LS error reconstruction
	// Multiply by quantization step size to get reconstructed error
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
