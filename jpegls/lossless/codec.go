package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

// LosslessCodec implements the codec.Codec interface for JPEG-LS Lossless
type LosslessCodec struct {
	bitDepth int
}

// NewLosslessCodec creates a new JPEG-LS Lossless codec
func NewLosslessCodec(bitDepth int) *LosslessCodec {
	return &LosslessCodec{
		bitDepth: bitDepth,
	}
}

// Encode encodes frame data using JPEG-LS Lossless compression
func (c *LosslessCodec) Encode(params codec.EncodeParams) ([]byte, error) {
	// Validate frame data
	if params.Width <= 0 || params.Height <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%d", params.Width, params.Height)
	}

	if params.Components != 1 && params.Components != 3 {
		return nil, fmt.Errorf("invalid components: %d (must be 1 or 3)", params.Components)
	}

	// Use specified bit depth or codec's bit depth
	bitDepth := c.bitDepth
	if bitDepth <= 0 {
		bitDepth = params.BitDepth
	}

	if bitDepth < 2 || bitDepth > 16 {
		return nil, fmt.Errorf("invalid bit depth: %d (must be 2-16)", bitDepth)
	}

	// Encode using JPEG-LS
	return Encode(params.PixelData, params.Width, params.Height, params.Components, bitDepth)
}

// Decode decodes JPEG-LS Lossless compressed data
func (c *LosslessCodec) Decode(data []byte) (*codec.DecodeResult, error) {
	pixelData, width, height, components, bitDepth, err := Decode(data)
	if err != nil {
		return nil, err
	}

	return &codec.DecodeResult{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   bitDepth,
	}, nil
}

// UID returns the DICOM Transfer Syntax UID for JPEG-LS Lossless
func (c *LosslessCodec) UID() string {
	return transfer.JPEGLSLossless.UID().UID()
}

// Name returns a human-readable name for this codec
func (c *LosslessCodec) Name() string {
	if c.bitDepth > 0 {
		return fmt.Sprintf("JPEG-LS Lossless (%d-bit)", c.bitDepth)
	}
	return "JPEG-LS Lossless"
}

// RegisterLosslessCodec registers JPEG-LS Lossless codec in the global registry
func RegisterLosslessCodec(bitDepth int) {
	c := NewLosslessCodec(bitDepth)
	codec.Register(c)
}

// init automatically registers the codec
func init() {
	// Register with default 8-bit configuration
	RegisterLosslessCodec(8)
}
