package jpeg2000

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
)

// Decoder implements JPEG 2000 decoding
type Decoder struct {
	// Codestream
	cs *codestream.Codestream

	// Decoded image data
	width      int
	height     int
	components int
	bitDepth   int
	isSigned   bool

	// Decoded pixel data per component
	data [][]int32
}

// NewDecoder creates a new JPEG 2000 decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode decodes a JPEG 2000 codestream
func (d *Decoder) Decode(data []byte) error {
	// Parse codestream
	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse codestream: %w", err)
	}

	d.cs = cs

	// Extract image parameters
	if err := d.extractImageParameters(); err != nil {
		return fmt.Errorf("failed to extract image parameters: %w", err)
	}

	// Decode all tiles
	if err := d.decodeTiles(); err != nil {
		return fmt.Errorf("failed to decode tiles: %w", err)
	}

	return nil
}

// extractImageParameters extracts image parameters from SIZ segment
func (d *Decoder) extractImageParameters() error {
	if d.cs.SIZ == nil {
		return fmt.Errorf("missing SIZ segment")
	}

	siz := d.cs.SIZ

	d.width = int(siz.Xsiz - siz.XOsiz)
	d.height = int(siz.Ysiz - siz.YOsiz)
	d.components = int(siz.Csiz)

	if d.components == 0 {
		return fmt.Errorf("invalid number of components: %d", d.components)
	}

	// Use first component's parameters
	d.bitDepth = siz.Components[0].BitDepth()
	d.isSigned = siz.Components[0].IsSigned()

	return nil
}

// decodeTiles decodes all tiles in the codestream
func (d *Decoder) decodeTiles() error {
	if len(d.cs.Tiles) == 0 {
		return fmt.Errorf("no tiles found in codestream")
	}

	// Create tile assembler
	assembler := NewTileAssembler(d.cs.SIZ)

	// Decode all tiles
	for tileIdx, tile := range d.cs.Tiles {
		// Create tile decoder
		tileDecoder := t2.NewTileDecoder(tile, d.cs.SIZ, d.cs.COD, d.cs.QCD)

		// Decode tile
		tileData, err := tileDecoder.Decode()
		if err != nil {
			return fmt.Errorf("failed to decode tile %d: %w", tileIdx, err)
		}

		// Assemble tile into image
		err = assembler.AssembleTile(tileIdx, tileData)
		if err != nil {
			return fmt.Errorf("failed to assemble tile %d: %w", tileIdx, err)
		}
	}

	// Get assembled image data
	d.data = assembler.GetImageData()

	// Note: Inverse DC level shift is already applied in tile decoder
	// Do not apply it again here to avoid double shifting

	return nil
}

// GetImageData returns the decoded image data for all components
func (d *Decoder) GetImageData() [][]int32 {
	return d.data
}

// GetComponentData returns the decoded data for a specific component
func (d *Decoder) GetComponentData(componentIdx int) ([]int32, error) {
	if componentIdx < 0 || componentIdx >= len(d.data) {
		return nil, fmt.Errorf("invalid component index: %d", componentIdx)
	}
	return d.data[componentIdx], nil
}

// Width returns the image width
func (d *Decoder) Width() int {
	return d.width
}

// Height returns the image height
func (d *Decoder) Height() int {
	return d.height
}

// Components returns the number of components
func (d *Decoder) Components() int {
	return d.components
}

// BitDepth returns the bit depth
func (d *Decoder) BitDepth() int {
	return d.bitDepth
}

// IsSigned returns whether the data is signed
func (d *Decoder) IsSigned() bool {
	return d.isSigned
}

// GetPixelData returns interleaved pixel data in a byte array
// Suitable for use with the Codec interface
func (d *Decoder) GetPixelData() []byte {
	if d.components == 1 {
		// Grayscale
		return d.getGrayscalePixelData()
	}
	// RGB or multi-component
	return d.getInterleavedPixelData()
}

// getGrayscalePixelData returns grayscale pixel data
func (d *Decoder) getGrayscalePixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit
		result := make([]byte, numPixels)
		for i := 0; i < numPixels; i++ {
			val := d.data[0][i]
			if val < 0 {
				val = 0
			} else if val > 255 {
				val = 255
			}
			result[i] = byte(val)
		}
		return result
	}

	// 16-bit (or 12-bit stored as 16-bit)
	result := make([]byte, numPixels*2)
	for i := 0; i < numPixels; i++ {
		val := d.data[0][i]
		if val < 0 {
			val = 0
		}
		maxVal := (1 << d.bitDepth) - 1
		if val > int32(maxVal) {
			val = int32(maxVal)
		}
		// Little-endian
		result[i*2] = byte(val)
		result[i*2+1] = byte(val >> 8)
	}
	return result
}

// getInterleavedPixelData returns interleaved RGB/multi-component pixel data
func (d *Decoder) getInterleavedPixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit per component
		result := make([]byte, numPixels*d.components)
		for i := 0; i < numPixels; i++ {
			for c := 0; c < d.components; c++ {
				val := d.data[c][i]
				if val < 0 {
					val = 0
				} else if val > 255 {
					val = 255
				}
				result[i*d.components+c] = byte(val)
			}
		}
		return result
	}

	// 16-bit per component
	result := make([]byte, numPixels*d.components*2)
	for i := 0; i < numPixels; i++ {
		for c := 0; c < d.components; c++ {
			val := d.data[c][i]
			if val < 0 {
				val = 0
			}
			maxVal := (1 << d.bitDepth) - 1
			if val > int32(maxVal) {
				val = int32(maxVal)
			}
			idx := (i*d.components + c) * 2
			result[idx] = byte(val)
			result[idx+1] = byte(val >> 8)
		}
	}
	return result
}

// applyInverseDCLevelShift applies inverse DC level shift for unsigned data
// For unsigned data: add 2^(bitDepth-1) to convert back from signed range
func (d *Decoder) applyInverseDCLevelShift() {
	if d.isSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - add 2^(bitDepth-1)
	shift := int32(1 << (d.bitDepth - 1))

	for c := 0; c < d.components; c++ {
		for i := 0; i < len(d.data[c]); i++ {
			d.data[c][i] += shift
		}
	}
}
