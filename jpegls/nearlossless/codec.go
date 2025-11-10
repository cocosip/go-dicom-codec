package nearlossless

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/codec"
)

// Codec implements the codec.Codec interface for JPEG-LS Near-Lossless
type Codec struct{}

// NewCodec creates a new JPEG-LS Near-Lossless codec
func NewCodec() *Codec {
	return &Codec{}
}

// UID returns the DICOM Transfer Syntax UID for JPEG-LS Near-Lossless
func (c *Codec) UID() string {
	return "1.2.840.10008.1.2.4.81"
}

// Name returns the human-readable name of this codec
func (c *Codec) Name() string {
	return "JPEG-LS Near-Lossless"
}

// Encode encodes pixel data to JPEG-LS Near-Lossless format
func (c *Codec) Encode(params codec.EncodeParams) ([]byte, error) {
	// Validate dimensions
	if params.Width <= 0 || params.Height <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%d", params.Width, params.Height)
	}

	// Validate components
	if params.Components != 1 && params.Components != 3 {
		return nil, fmt.Errorf("invalid components: %d (must be 1 or 3)", params.Components)
	}

	// Validate bit depth
	if params.BitDepth < 2 || params.BitDepth > 16 {
		return nil, fmt.Errorf("invalid bit depth: %d (must be 2-16)", params.BitDepth)
	}

	// Extract NEAR parameter from options
	near := 0 // Default to lossless
	if params.Options != nil {
		if opts, ok := params.Options.(*codec.BaseOptions); ok {
			near = opts.NearLossless
		} else if opts, ok := params.Options.(*Options); ok {
			near = opts.NEAR
		}
	}

	// Validate NEAR parameter
	if near < 0 || near > 255 {
		return nil, fmt.Errorf("invalid NEAR parameter: %d (must be 0-255)", near)
	}

	// Encode using the package-level function
	return Encode(params.PixelData, params.Width, params.Height, params.Components, params.BitDepth, near)
}

// Decode decodes JPEG-LS Near-Lossless compressed data
func (c *Codec) Decode(compressedData []byte) (*codec.DecodeResult, error) {
	if len(compressedData) == 0 {
		return nil, codec.ErrInvalidParameter
	}

	// Decode using the package-level function
	pixelData, width, height, components, bitDepth, _, err := Decode(compressedData)
	if err != nil {
		return nil, err
	}

	// Create decode result
	result := &codec.DecodeResult{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   bitDepth,
	}

	return result, nil
}

// Options defines JPEG-LS Near-Lossless specific encoding options
type Options struct {
	// NEAR parameter: maximum allowed absolute difference for near-lossless mode
	// 0 = lossless, >0 = near-lossless with bounded error
	NEAR int
}

// Validate validates the options
func (o *Options) Validate() error {
	if o.NEAR < 0 || o.NEAR > 255 {
		return fmt.Errorf("invalid NEAR parameter: %d (must be 0-255)", o.NEAR)
	}
	return nil
}

// RegisterCodec registers JPEG-LS Near-Lossless codec in the global registry
func RegisterCodec() {
	c := NewCodec()
	codec.Register(c)
}

// init automatically registers the codec
func init() {
	RegisterCodec()
}
