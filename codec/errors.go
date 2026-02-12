// Package codec provides common errors and interfaces for image codecs.
package codec

import "errors"

var (
	// ErrCodecNotFound is returned when a codec is not found in the registry.
	ErrCodecNotFound = errors.New("codec not found")

	// ErrInvalidParameter indicates encoding/decoding parameters are invalid.
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrInvalidQuality indicates an invalid quality parameter (must be 1-100).
	ErrInvalidQuality = errors.New("invalid quality (must be 1-100)")

	// ErrUnsupportedFormat indicates the format is not supported.
	ErrUnsupportedFormat = errors.New("unsupported format")
)
