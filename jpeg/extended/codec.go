package extended

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// ExtendedCodec implements the external codec.Codec interface for JPEG Extended
type ExtendedCodec struct {
	quality  int
	bitDepth int // 8 or 12
}

// NewExtendedCodec creates a new JPEG Extended codec
// bitDepth: 8 or 12 bits per sample
// quality: 1-100, where 100 is best quality (default 85)
func NewExtendedCodec(bitDepth int, quality int) *ExtendedCodec {
	if bitDepth != 8 && bitDepth != 12 {
		bitDepth = 12 // Default to 12-bit (main feature of Extended)
	}
	if quality < 1 || quality > 100 {
		quality = 85 // Default quality
	}
	return &ExtendedCodec{
		quality:  quality,
		bitDepth: bitDepth,
	}
}

// Name returns the codec name
func (c *ExtendedCodec) Name() string {
	return fmt.Sprintf("JPEG Extended (%d-bit, Quality %d)", c.bitDepth, c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *ExtendedCodec) TransferSyntax() *transfer.TransferSyntax {
	return transfer.JPEGExtended12Bit
}

// Encode encodes pixel data using JPEG Extended
func (c *ExtendedCodec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	// Extract parameters
	width := int(src.Width)
	height := int(src.Height)
	components := int(src.SamplesPerPixel)

	// Determine bit depth from source
	bitDepth := c.bitDepth
	if src.BitsStored > 0 && src.BitsStored <= 8 {
		bitDepth = 8
	} else if src.BitsStored > 8 && src.BitsStored <= 12 {
		bitDepth = 12
	}

	// Check for parameter overrides
	quality := c.quality
	if params != nil {
		if q, ok := params.GetParameter("quality").(int); ok && q >= 1 && q <= 100 {
			quality = q
		}
		if bd, ok := params.GetParameter("bitDepth").(int); ok && (bd == 8 || bd == 12) {
			bitDepth = bd
		}
	}

	// Encode
	encoded, err := Encode(src.Data, width, height, components, bitDepth, quality)
	if err != nil {
		return fmt.Errorf("JPEG Extended encode failed: %w", err)
	}

	// Set destination data
	dst.Data = encoded
	dst.Width = src.Width
	dst.Height = src.Height
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = src.BitsAllocated
	dst.BitsStored = src.BitsStored
	dst.HighBit = src.HighBit
	dst.SamplesPerPixel = src.SamplesPerPixel
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = transfer.JPEGExtended12Bit.UID().UID()

	return nil
}

// Decode decodes JPEG Extended data
func (c *ExtendedCodec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	// Decode
	decoded, width, height, components, bitDepth, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG Extended decode failed: %w", err)
	}

	// Determine bits allocated
	bitsAllocated := uint16(8)
	bitsStored := uint16(bitDepth)
	if bitDepth > 8 {
		bitsAllocated = 16
	}

	// Set destination data
	dst.Data = decoded
	dst.Width = uint16(width)
	dst.Height = uint16(height)
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = bitsAllocated
	dst.BitsStored = bitsStored
	dst.HighBit = bitsStored - 1
	dst.SamplesPerPixel = uint16(components)
	dst.PixelRepresentation = 0
	dst.PlanarConfiguration = 0

	// Set photometric interpretation
	if components == 1 {
		dst.PhotometricInterpretation = "MONOCHROME2"
	} else {
		dst.PhotometricInterpretation = "RGB"
	}

	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID()

	return nil
}

// RegisterExtendedCodec registers JPEG Extended codec with the global registry
// bitDepth: 8 or 12 (default 12)
// quality: 1-100 (default 85)
func RegisterExtendedCodec(bitDepth int, quality int) {
	c := NewExtendedCodec(bitDepth, quality)
	registry := codec.GetGlobalRegistry()
	registry.RegisterCodec(transfer.JPEGExtended12Bit, c)
}
