package lossless

import (
	"bytes"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
)

// Encoder represents a JPEG Lossless encoder (supporting all 7 predictors)
type Encoder struct {
	width      int
	height     int
	components int
	precision  int // Bit depth (2-16)
	predictor  int // Predictor selection (1-7)

	dcTables [2]*common.HuffmanTable
	dcCodes  [2][]common.HuffmanCode
}

// Encode encodes pixel data to JPEG Lossless format
// predictor: 0 for auto-select, 1-7 for specific predictor
// bitDepth: 2-16 bits per sample
func Encode(pixelData []byte, width, height, components, bitDepth, predictor int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, common.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, common.ErrInvalidComponents
	}

	if bitDepth < 2 || bitDepth > 16 {
		return nil, common.ErrInvalidBitDepth
	}

	if predictor < 0 || predictor > 7 {
		return nil, common.ErrInvalidPredictor
	}

	bytesPerSample := (bitDepth + 7) / 8
	expectedSize := width * height * components * bytesPerSample
	if len(pixelData) < expectedSize {
		return nil, common.ErrBufferTooSmall
	}

	enc := &Encoder{
		width:      width,
		height:     height,
		components: components,
		precision:  bitDepth,
		predictor:  predictor,
	}

	// Auto-select best predictor if predictor == 0
	if enc.predictor == 0 {
		samples := enc.pixelsToSamples(pixelData)
		enc.predictor = SelectBestPredictor(samples, width, height)
	}

	// Use standard DC Huffman tables
	enc.dcTables[0] = common.BuildStandardHuffmanTable(
		common.StandardDCLuminanceBits,
		common.StandardDCLuminanceValues,
	)
	enc.dcTables[1] = common.BuildStandardHuffmanTable(
		common.StandardDCChrominanceBits,
		common.StandardDCChrominanceValues,
	)

	// Build Huffman codes
	enc.dcCodes[0] = common.BuildHuffmanCodes(enc.dcTables[0])
	enc.dcCodes[1] = common.BuildHuffmanCodes(enc.dcTables[1])

	var buf bytes.Buffer
	writer := common.NewWriter(&buf)

	// Write SOI
	if err := writer.WriteMarker(common.MarkerSOI); err != nil {
		return nil, err
	}

	// Write SOF3 (Lossless)
	if err := enc.writeSOF3(writer); err != nil {
		return nil, err
	}

	// Write DHT
	if err := enc.writeDHT(writer); err != nil {
		return nil, err
	}

	// Write SOS and scan data
	if err := enc.writeSOS(writer, pixelData); err != nil {
		return nil, err
	}

	// Write EOI
	if err := writer.WriteMarker(common.MarkerEOI); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeSOF3 writes Start of Frame (Lossless)
func (enc *Encoder) writeSOF3(writer *common.Writer) error {
	data := make([]byte, 6+enc.components*3)

	data[0] = byte(enc.precision)       // Precision
	data[1] = byte(enc.height >> 8)     // Height high byte
	data[2] = byte(enc.height)          // Height low byte
	data[3] = byte(enc.width >> 8)      // Width high byte
	data[4] = byte(enc.width)           // Width low byte
	data[5] = byte(enc.components)      // Number of components

	if enc.components == 1 {
		// Grayscale
		data[6] = 1    // Component ID
		data[7] = 0x11 // Sampling factors: 1x1
		data[8] = 0    // Tq (not used in lossless, set to 0)
	} else {
		// YCbCr or RGB (no subsampling in lossless)
		for i := 0; i < enc.components; i++ {
			offset := 6 + i*3
			data[offset] = byte(i + 1) // Component ID
			data[offset+1] = 0x11      // Sampling factors: 1x1
			data[offset+2] = 0         // Tq (not used)
		}
	}

	return writer.WriteSegment(common.MarkerSOF3, data)
}

// writeDHT writes Define Huffman Table segments
func (enc *Encoder) writeDHT(writer *common.Writer) error {
	numTables := 1
	if enc.components == 3 {
		numTables = 2
	}

	for i := 0; i < numTables; i++ {
		table := enc.dcTables[i]
		totalValues := 0
		for _, count := range table.Bits {
			totalValues += count
		}

		data := make([]byte, 1+16+totalValues)
		data[0] = byte(i) // Table class 0 (DC/Lossless), ID i

		for j := 0; j < 16; j++ {
			data[1+j] = byte(table.Bits[j])
		}

		copy(data[17:], table.Values)

		if err := writer.WriteSegment(common.MarkerDHT, data); err != nil {
			return err
		}
	}

	return nil
}

// writeSOS writes Start of Scan and scan data
func (enc *Encoder) writeSOS(writer *common.Writer, pixelData []byte) error {
	// Write SOS header
	data := make([]byte, 1+enc.components*2+3)
	data[0] = byte(enc.components)

	if enc.components == 1 {
		data[1] = 1    // Component ID
		data[2] = 0x00 // Table selector 0
	} else {
		for i := 0; i < enc.components; i++ {
			data[1+i*2] = byte(i + 1) // Component ID
			if i == 0 {
				data[2+i*2] = 0x00 // Table 0 for Y
			} else {
				data[2+i*2] = 0x01 // Table 1 for Cb/Cr
			}
		}
	}

	// Spectral selection
	data[1+enc.components*2] = byte(enc.predictor) // Ss: Predictor selection value (1-7)
	data[2+enc.components*2] = 0                   // Se: not used
	data[3+enc.components*2] = 0                   // Ah/Al: not used

	if err := writer.WriteSegment(common.MarkerSOS, data); err != nil {
		return err
	}

	// Encode scan data
	return enc.encodeScan(writer, pixelData)
}

// encodeScan encodes the scan data
func (enc *Encoder) encodeScan(writer *common.Writer, pixelData []byte) error {
	var scanBuf bytes.Buffer
	huffEnc := common.NewHuffmanEncoder(&scanBuf)

	// Convert pixel data to sample array
	samples := enc.pixelsToSamples(pixelData)

	// Encode line by line, interleaved
	for row := 0; row < enc.height; row++ {
		for col := 0; col < enc.width; col++ {
			for comp := 0; comp < enc.components; comp++ {
				sample := samples[comp][row*enc.width+col]

				// Get neighbor values for prediction
				var ra, rb, rc int
				defaultVal := 1 << uint(enc.precision-1) // 2^(P-1)

				// Ra: left pixel
				if col > 0 {
					ra = samples[comp][row*enc.width+(col-1)]
				} else {
					ra = defaultVal
				}

				// Rb: above pixel
				if row > 0 {
					rb = samples[comp][(row-1)*enc.width+col]
				} else {
					rb = defaultVal
				}

				// Rc: above-left pixel
				if row > 0 && col > 0 {
					rc = samples[comp][(row-1)*enc.width+(col-1)]
				} else {
					rc = defaultVal
				}

				// Apply predictor
				var predicted int
				if col == 0 && row == 0 {
					// First pixel: use default
					predicted = defaultVal
				} else {
					predicted = Predictor(enc.predictor, ra, rb, rc)
				}

				// Calculate difference
				diff := sample - predicted

				// Encode difference
				tableIdx := 0
				if comp > 0 && enc.components > 1 {
					tableIdx = 1
				}

				cat, bits := huffEnc.EncodeCategory(diff)
				code := enc.dcCodes[tableIdx][cat]
				if err := huffEnc.WriteBits(uint32(code.Code), code.Len); err != nil {
					return err
				}
				if cat > 0 {
					if err := huffEnc.WriteBits(bits, cat); err != nil {
						return err
					}
				}
			}
		}
	}

	if err := huffEnc.Flush(); err != nil {
		return err
	}

	// Write scan data
	return writer.WriteBytes(scanBuf.Bytes())
}

// pixelsToSamples converts byte array to sample arrays
func (enc *Encoder) pixelsToSamples(pixelData []byte) [][]int {
	samples := make([][]int, enc.components)
	for i := range samples {
		samples[i] = make([]int, enc.width*enc.height)
	}

	if enc.precision <= 8 {
		// 8-bit or less: one byte per sample
		for y := 0; y < enc.height; y++ {
			for x := 0; x < enc.width; x++ {
				for i := 0; i < enc.components; i++ {
					val := int(pixelData[(y*enc.width+x)*enc.components+i])
					samples[i][y*enc.width+x] = val
				}
			}
		}
	} else {
		// 9-16 bit: two bytes per sample (little-endian)
		offset := 0
		for y := 0; y < enc.height; y++ {
			for x := 0; x < enc.width; x++ {
				for i := 0; i < enc.components; i++ {
					val := int(pixelData[offset]) | (int(pixelData[offset+1]) << 8)
					samples[i][y*enc.width+x] = val
					offset += 2
				}
			}
		}
	}

	return samples
}
