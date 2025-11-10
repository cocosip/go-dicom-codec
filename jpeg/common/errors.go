package common

import "errors"

// Common errors
var (
	ErrInvalidMarker      = errors.New("invalid JPEG marker")
	ErrInvalidSOI         = errors.New("missing SOI marker")
	ErrInvalidEOI         = errors.New("missing EOI marker")
	ErrInvalidSOF         = errors.New("invalid Start of Frame")
	ErrInvalidDHT         = errors.New("invalid Huffman table")
	ErrInvalidDQT         = errors.New("invalid Quantization table")
	ErrInvalidSOS         = errors.New("invalid Start of Scan")
	ErrUnsupportedFormat  = errors.New("unsupported JPEG format")
	ErrInvalidData        = errors.New("invalid JPEG data")
	ErrUnexpectedEOF      = errors.New("unexpected end of file")
	ErrInvalidDimensions  = errors.New("invalid image dimensions")
	ErrInvalidComponents  = errors.New("invalid number of components")
	ErrInvalidBitDepth    = errors.New("invalid bit depth")
	ErrInvalidPrecision   = errors.New("invalid precision")
	ErrInvalidQuality     = errors.New("invalid quality factor")
	ErrInvalidPredictor   = errors.New("invalid predictor value")
	ErrHuffmanDecode      = errors.New("Huffman decode error")
	ErrBufferTooSmall     = errors.New("buffer too small")
)
