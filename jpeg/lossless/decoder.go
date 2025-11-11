package lossless

import (
	"bytes"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
)

// Decoder represents a JPEG Lossless decoder
type Decoder struct {
	width      int
	height     int
	components int
	precision  int // Bit depth (2-16)
	predictor  int // Predictor selection (1-7)

	dcTables [2]*common.HuffmanTable
}

// Decode decodes JPEG Lossless data
func Decode(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	r := bytes.NewReader(jpegData)
	reader := common.NewReader(r)

	decoder := &Decoder{}

	// Read SOI marker
	marker, err := reader.ReadMarker()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if marker != common.MarkerSOI {
		return nil, 0, 0, 0, 0, common.ErrInvalidSOI
	}

	// Parse JPEG segments
	for {
		marker, err := reader.ReadMarker()
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}

		switch marker {
		case common.MarkerSOF3:
			if err := decoder.parseSOF3(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case common.MarkerDHT:
			if err := decoder.parseDHT(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

		case common.MarkerSOS:
			if err := decoder.parseSOS(reader); err != nil {
				return nil, 0, 0, 0, 0, err
			}

			// Decode scan data
			samples, err := decoder.decodeScan(reader)
			if err != nil {
				return nil, 0, 0, 0, 0, err
			}

			// Convert samples to pixel data
			pixelData = decoder.samplesToPixels(samples)

			return pixelData, decoder.width, decoder.height, decoder.components, decoder.precision, nil

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
}

// parseSOF3 parses Start of Frame (Lossless)
func (d *Decoder) parseSOF3(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 6 {
		return common.ErrInvalidSOF
	}

	d.precision = int(data[0])
	if d.precision < 2 || d.precision > 16 {
		return fmt.Errorf("invalid precision: %d (must be 2-16)", d.precision)
	}

	d.height = int(data[1])<<8 | int(data[2])
	d.width = int(data[3])<<8 | int(data[4])
	d.components = int(data[5])

	if d.width <= 0 || d.height <= 0 {
		return common.ErrInvalidDimensions
	}

	if d.components != 1 && d.components != 3 {
		return common.ErrInvalidComponents
	}

	return nil
}

// parseDHT parses Define Huffman Table
func (d *Decoder) parseDHT(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	offset := 0
	for offset < len(data) {
		if offset >= len(data) {
			break
		}

		tcTh := data[offset]
		offset++

		tc := (tcTh >> 4) & 0x0F // Table class (0 = DC)
		th := tcTh & 0x0F        // Table ID

		if th >= 2 {
			return fmt.Errorf("invalid Huffman table ID: %d", th)
		}

		// Read bit counts
		var bits [16]int
		totalSymbols := 0
		for i := 0; i < 16; i++ {
			if offset >= len(data) {
				return common.ErrInvalidDHT
			}
			bits[i] = int(data[offset])
			totalSymbols += bits[i]
			offset++
		}

		// Read symbol values
		if offset+totalSymbols > len(data) {
			return common.ErrInvalidDHT
		}
		values := make([]byte, totalSymbols)
		copy(values, data[offset:offset+totalSymbols])
		offset += totalSymbols

		// Build Huffman table
		var bitsArray [16]int
		copy(bitsArray[:], bits[:])
		table := common.BuildStandardHuffmanTable(bitsArray, values)

		if tc == 0 {
			// DC table (used for lossless)
			d.dcTables[th] = table
		}
	}

	return nil
}

// parseSOS parses Start of Scan
func (d *Decoder) parseSOS(reader *common.Reader) error {
	data, err := reader.ReadSegment()
	if err != nil {
		return err
	}

	if len(data) < 1+d.components*2+3 {
		return common.ErrInvalidSOS
	}

	numComponents := int(data[0])
	if numComponents != d.components {
		return fmt.Errorf("SOS component count %d does not match SOF %d", numComponents, d.components)
	}

	// Parse spectral selection (contains predictor)
	d.predictor = int(data[1+d.components*2]) // Ss field contains predictor

	if d.predictor < 1 || d.predictor > 7 {
		return fmt.Errorf("invalid predictor: %d (must be 1-7)", d.predictor)
	}

	return nil
}

// decodeScan decodes the scan data
func (d *Decoder) decodeScan(reader *common.Reader) ([][]int, error) {
	// Read scan data until we hit a marker or EOF
	// NOTE: Do NOT process byte stuffing here - HuffmanDecoder handles it
	var scanData bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		if b == 0xFF {
			// Peek at next byte to check if it's a marker
			b2, err := reader.ReadByte()
			if err != nil {
				// EOF after 0xFF, include it
				scanData.WriteByte(b)
				break
			}

			if b2 == 0x00 {
				// Byte stuffing: 0xFF 0x00 - include both bytes
				// HuffmanDecoder will handle the unstuffing
				scanData.WriteByte(b)
				scanData.WriteByte(b2)
			} else {
				// Marker found (0xFF followed by non-zero)
				// We're done with scan data
				// Put back the marker bytes for potential future reading
				break
			}
		} else {
			scanData.WriteByte(b)
		}
	}

	huffDec := common.NewHuffmanDecoder(bytes.NewReader(scanData.Bytes()))

	// Allocate sample arrays
	samples := make([][]int, d.components)
	for i := range samples {
		samples[i] = make([]int, d.width*d.height)
	}

	// Decode pixel by pixel, interleaved
	for row := 0; row < d.height; row++ {
		for col := 0; col < d.width; col++ {
			for comp := 0; comp < d.components; comp++ {
				// Select Huffman table
				tableIdx := 0
				if comp > 0 && d.components > 1 {
					tableIdx = 1
				}

				table := d.dcTables[tableIdx]
				if table == nil {
					return nil, fmt.Errorf("huffman table %d not defined", tableIdx)
				}

				// Decode category
				category, err := huffDec.Decode(table)
				if err != nil {
					return nil, err
				}

				// Decode difference value
				var diff int
				if category > 0 {
					// ReceiveExtend will call ReadBits internally
					var err error
					diff, err = huffDec.ReceiveExtend(int(category))
					if err != nil {
						return nil, err
					}
				}

				// Get neighbor values for prediction
				var ra, rb, rc int
				defaultVal := 1 << uint(d.precision-1) // 2^(P-1)

				// Ra: left pixel
				if col > 0 {
					ra = samples[comp][row*d.width+(col-1)]
				} else {
					ra = defaultVal
				}

				// Rb: above pixel
				if row > 0 {
					rb = samples[comp][(row-1)*d.width+col]
				} else {
					rb = defaultVal
				}

				// Rc: above-left pixel
				if row > 0 && col > 0 {
					rc = samples[comp][(row-1)*d.width+(col-1)]
				} else {
					rc = defaultVal
				}

				// Apply predictor
				var predicted int
				if col == 0 && row == 0 {
					// First pixel: use default
					predicted = defaultVal
				} else {
					predicted = Predictor(d.predictor, ra, rb, rc)
				}

				// Reconstruct sample
				sample := predicted + diff

				// Clamp to valid range
				maxVal := (1 << uint(d.precision)) - 1
				if sample < 0 {
					sample = 0
				}
				if sample > maxVal {
					sample = maxVal
				}

				samples[comp][row*d.width+col] = sample
			}
		}
	}

	return samples, nil
}

// samplesToPixels converts sample arrays to byte array
func (d *Decoder) samplesToPixels(samples [][]int) []byte {
	bytesPerSample := (d.precision + 7) / 8
	pixelData := make([]byte, d.width*d.height*d.components*bytesPerSample)

	if d.precision <= 8 {
		// 8-bit or less: one byte per sample
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				for i := 0; i < d.components; i++ {
					val := samples[i][y*d.width+x]
					pixelData[(y*d.width+x)*d.components+i] = byte(val)
				}
			}
		}
	} else {
		// 9-16 bit: two bytes per sample (little-endian)
		offset := 0
		for y := 0; y < d.height; y++ {
			for x := 0; x < d.width; x++ {
				for i := 0; i < d.components; i++ {
					val := samples[i][y*d.width+x]
					pixelData[offset] = byte(val & 0xFF)
					pixelData[offset+1] = byte((val >> 8) & 0xFF)
					offset += 2
				}
			}
		}
	}

	return pixelData
}
