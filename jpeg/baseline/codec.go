package baseline

import (
	"github.com/cocosip/go-dicom-codec/codec"
)

// Codec implements the codec.Codec interface for JPEG Baseline
type Codec struct{}

// NewCodec creates a new JPEG Baseline codec
func NewCodec() *Codec {
	return &Codec{}
}

// Encode encodes pixel data using JPEG Baseline
func (c *Codec) Encode(params codec.EncodeParams) ([]byte, error) {
	// Extract quality from options
	quality := 85 // default quality
	if params.Options != nil {
		if opts, ok := params.Options.(*Options); ok {
			if err := opts.Validate(); err != nil {
				return nil, err
			}
			quality = opts.Quality
		}
	}

	// Call the baseline encoder
	return Encode(
		params.PixelData,
		params.Width,
		params.Height,
		params.Components,
		quality,
	)
}

// Decode decodes JPEG Baseline data
func (c *Codec) Decode(data []byte) (*codec.DecodeResult, error) {
	pixelData, width, height, components, err := Decode(data)
	if err != nil {
		return nil, err
	}

	return &codec.DecodeResult{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   8, // Baseline is always 8-bit
	}, nil
}

// UID returns the DICOM Transfer Syntax UID for JPEG Baseline
func (c *Codec) UID() string {
	return "1.2.840.10008.1.2.4.50"
}

// Name returns the human-readable name
func (c *Codec) Name() string {
	return "jpeg-baseline"
}

// Options contains encoding options for JPEG Baseline
type Options struct {
	codec.BaseOptions
}

// Validate validates the options
func (o *Options) Validate() error {
	// Quality is validated in BaseOptions
	return o.BaseOptions.Validate()
}

// Register registers this codec with the global registry
func init() {
	codec.Register(NewCodec())
}
