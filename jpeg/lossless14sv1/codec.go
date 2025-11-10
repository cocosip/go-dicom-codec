package lossless14sv1

import (
	"github.com/cocosip/go-dicom-codec/codec"
)

// Codec implements the codec.Codec interface for JPEG Lossless SV1
type Codec struct{}

// NewCodec creates a new JPEG Lossless SV1 codec
func NewCodec() *Codec {
	return &Codec{}
}

// Encode encodes pixel data using JPEG Lossless First-Order Prediction
func (c *Codec) Encode(params codec.EncodeParams) ([]byte, error) {
	// Validate options if provided
	if params.Options != nil {
		if err := params.Options.Validate(); err != nil {
			return nil, err
		}
	}

	// Call the lossless encoder
	return Encode(
		params.PixelData,
		params.Width,
		params.Height,
		params.Components,
		params.BitDepth,
	)
}

// Decode decodes JPEG Lossless SV1 data
func (c *Codec) Decode(data []byte) (*codec.DecodeResult, error) {
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

// UID returns the DICOM Transfer Syntax UID for JPEG Lossless SV1
func (c *Codec) UID() string {
	return "1.2.840.10008.1.2.4.70"
}

// Name returns the human-readable name
func (c *Codec) Name() string {
	return "jpeg-lossless-sv1"
}

// Options contains encoding options for JPEG Lossless SV1
// Lossless encoding doesn't use quality parameter
type Options struct {
	// No additional options for lossless
}

// Validate validates the options
func (o *Options) Validate() error {
	// No validation needed for lossless
	return nil
}

// Register registers this codec with the global registry
func init() {
	codec.Register(NewCodec())
}
