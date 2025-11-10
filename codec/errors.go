package codec

import "errors"

var (
	// ErrCodecNotFound is returned when a codec is not found in the registry
	ErrCodecNotFound = errors.New("codec not found")

	// ErrInvalidParameters is returned when encoding/decoding parameters are invalid
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrInvalidQuality is returned when quality parameter is invalid
	ErrInvalidQuality = errors.New("invalid quality (must be 1-100)")

	// ErrUnsupportedFormat is returned when the format is not supported
	ErrUnsupportedFormat = errors.New("unsupported format")
)
