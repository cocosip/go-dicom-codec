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

	contextTable *ContextTable
	runDecoder   *RunModeDecoder
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
	dec.contextTable = NewContextTable(dec.maxVal)
	dec.runDecoder = NewRunModeDecoder()

	return nil
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
		if len(data) < 13 {
			return fmt.Errorf("invalid LSE preset parameters")
		}

		// Read MAXVAL
		maxVal := int(data[1])<<8 | int(data[2])
		if maxVal > 0 {
			dec.maxVal = maxVal
		}

		// T1, T2, T3 thresholds are at data[3:8]
		// RESET interval at data[9:12]
		// For now, we use standard values
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

// decodeComponent decodes a single component
func (dec *Decoder) decodeComponent(gr *GolombReader, pixels []int, comp int) error {
	stride := dec.components
	offset := comp

	// Process line by line
	for y := 0; y < dec.height; y++ {
		for x := 0; x < dec.width; x++ {
			idx := (y*dec.width + x) * stride + offset

			// Get neighboring pixels
			a, b, c, d := dec.getNeighbors(pixels, x, y, comp)

			// Compute prediction
			prediction := Predict(a, b, c)

			// Compute context
			q1, q2, q3 := ComputeContext(a, b, c, d)
			ctx := dec.contextTable.GetContext(q1, q2, q3)

			// Apply bias correction
			bias := ctx.GetBias()
			correctedPred := CorrectPrediction(prediction, bias, dec.maxVal+1)

			// Get Golomb parameter
			k := ctx.ComputeGolombParameter()

			// Decode error
			mappedError, err := gr.ReadGolomb(k)
			if err != nil {
				return err
			}

			// Unmap error
			errValue := UnmapErrorValue(mappedError)

			// Reconstruct sample
			sample := correctedPred + errValue

			// Clamp to valid range
			if sample < 0 {
				sample += (dec.maxVal + 1)
			} else if sample > dec.maxVal {
				sample -= (dec.maxVal + 1)
			}

			pixels[idx] = sample

			// Update context
			ctx.UpdateContext(errValue)
		}
	}

	return nil
}

// getNeighbors gets neighboring pixels for prediction
func (dec *Decoder) getNeighbors(pixels []int, x, y, comp int) (int, int, int, int) {
	stride := dec.components
	offset := comp

	a, b, c, d := 0, 0, 0, 0

	if x > 0 {
		idx := (y*dec.width + (x - 1)) * stride + offset
		a = pixels[idx]
	}

	if y > 0 {
		idx := ((y-1)*dec.width + x) * stride + offset
		b = pixels[idx]
	}

	if x > 0 && y > 0 {
		idx := ((y-1)*dec.width + (x - 1)) * stride + offset
		c = pixels[idx]
	}

	if x < dec.width-1 && y > 0 {
		idx := ((y-1)*dec.width + (x + 1)) * stride + offset
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
