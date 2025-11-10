package codec

// Codec is the universal interface for all image codecs
type Codec interface {
	// Encode encodes pixel data
	Encode(params EncodeParams) ([]byte, error)

	// Decode decodes compressed data
	Decode(data []byte) (*DecodeResult, error)

	// UID returns the unique identifier (typically DICOM Transfer Syntax UID)
	UID() string

	// Name returns a human-readable name
	Name() string
}

// EncodeParams contains parameters for encoding
type EncodeParams struct {
	PixelData  []byte  // Raw pixel data
	Width      int     // Image width
	Height     int     // Image height
	Components int     // Number of color components (1=grayscale, 3=RGB)
	BitDepth   int     // Bits per sample (8, 12, 16, etc.)
	Options    Options // Codec-specific options
}

// Options is an interface for codec-specific encoding options
type Options interface {
	// Validate checks if the options are valid
	Validate() error
}

// DecodeResult contains the result of decoding
type DecodeResult struct {
	PixelData  []byte // Decoded pixel data
	Width      int    // Image width
	Height     int    // Image height
	Components int    // Number of color components
	BitDepth   int    // Bits per sample
}

// BaseOptions provides common options for all codecs
type BaseOptions struct {
	// Quality factor for lossy codecs (1-100, higher is better)
	// Not used for lossless codecs
	Quality int

	// NearLossless parameter for near-lossless codecs (e.g., JPEG-LS)
	// 0 = lossless, >0 = near-lossless with specified error bound
	NearLossless int
}

// Validate validates base options
func (o *BaseOptions) Validate() error {
	if o.Quality < 0 || o.Quality > 100 {
		return ErrInvalidQuality
	}
	if o.NearLossless < 0 {
		return ErrInvalidParameter
	}
	return nil
}
