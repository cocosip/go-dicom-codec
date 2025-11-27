package lossless

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEG2000LosslessParameters implements codec.Parameters
var _ codec.Parameters = (*JPEG2000LosslessParameters)(nil)

// JPEG2000LosslessParameters contains parameters for JPEG 2000 Lossless compression
type JPEG2000LosslessParameters struct {
	// NumLevels controls the number of wavelet decomposition levels (0-6)
	// - 0: No decomposition (minimal compression, fastest)
	// - 1: Single-level decomposition
	// - 3: Medium levels (good balance)
	// - 5: Default, recommended for most images
	// - 6: Maximum levels (best compression for large images)
	//
	// More levels generally provide better compression but require more computation.
	// For small images (< 128x128), use fewer levels (1-3).
	// For large images (>= 512x512), use more levels (5-6).
	NumLevels int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewLosslessParameters creates a new JPEG2000LosslessParameters with default values
func NewLosslessParameters() *JPEG2000LosslessParameters {
	return &JPEG2000LosslessParameters{
		NumLevels: 5, // Default 5 decomposition levels (recommended)
		params:    make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEG2000LosslessParameters) GetParameter(name string) interface{} {
	switch name {
	case "numLevels":
		return p.NumLevels
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEG2000LosslessParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "numLevels":
		if v, ok := value.(int); ok {
			p.NumLevels = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and adjusts them if needed
func (p *JPEG2000LosslessParameters) Validate() error {
	// NumLevels must be in range 0-6
	if p.NumLevels < 0 || p.NumLevels > 6 {
		p.NumLevels = 5 // Reset to default
	}
	return nil
}

// WithNumLevels sets the number of decomposition levels and returns the parameters for chaining
func (p *JPEG2000LosslessParameters) WithNumLevels(numLevels int) *JPEG2000LosslessParameters {
	p.NumLevels = numLevels
	return p
}
