package lossy

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEG2000LossyParameters implements codec.Parameters
var _ codec.Parameters = (*JPEG2000LossyParameters)(nil)

// JPEG2000LossyParameters contains parameters for JPEG 2000 lossy compression
type JPEG2000LossyParameters struct {
	// Quality controls the compression quality vs. file size tradeoff (1-100)
	// - 100: Near-lossless quality (~3:1 compression, max error ≤1 pixel)
	// - 80:  High quality (~3-4:1 compression, max error ≤3 pixels) - Default
	// - 50:  Medium quality (~6:1 compression, max error ~12 pixels)
	// - 20:  High compression (~10:1 compression, max error ~30 pixels)
	// - 1:   Maximum compression (max error can be large)
	Quality int

	// NumLevels specifies the number of wavelet decomposition levels (0-6)
	// More levels = better compression but slower encoding/decoding
	// Default: 5
	NumLevels int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewLossyParameters creates a new JPEG2000LossyParameters with default values
func NewLossyParameters() *JPEG2000LossyParameters {
	return &JPEG2000LossyParameters{
		Quality:   80, // Default high quality
		NumLevels: 5,  // Default 5 decomposition levels
		params:    make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEG2000LossyParameters) GetParameter(name string) interface{} {
	switch name {
	case "quality":
		return p.Quality
	case "numLevels":
		return p.NumLevels
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEG2000LossyParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "quality":
		if v, ok := value.(int); ok {
			p.Quality = v
		}
	case "numLevels":
		if v, ok := value.(int); ok {
			p.NumLevels = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid
func (p *JPEG2000LossyParameters) Validate() error {
	if p.Quality < 1 || p.Quality > 100 {
		p.Quality = 80 // Reset to default
	}
	if p.NumLevels < 0 || p.NumLevels > 6 {
		p.NumLevels = 5 // Reset to default
	}
	return nil
}

// WithQuality sets the quality and returns the parameters for chaining
func (p *JPEG2000LossyParameters) WithQuality(quality int) *JPEG2000LossyParameters {
	p.Quality = quality
	return p
}

// WithNumLevels sets the number of wavelet levels and returns the parameters for chaining
func (p *JPEG2000LossyParameters) WithNumLevels(levels int) *JPEG2000LossyParameters {
	p.NumLevels = levels
	return p
}
