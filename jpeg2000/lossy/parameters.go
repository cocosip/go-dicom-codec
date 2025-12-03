package lossy

import (
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEG2000LossyParameters implements codec.Parameters
var _ codec.Parameters = (*JPEG2000LossyParameters)(nil)

// JPEG2000LossyParameters contains parameters for JPEG 2000 lossy compression.
type JPEG2000LossyParameters struct {
	// Quality controls the compression quality vs. file size tradeoff (1-100).
	// 100 = near-lossless, 80 = high quality (default), 1 = max compression.
	Quality int

	// NumLevels specifies the number of wavelet decomposition levels (0-6).
	// More levels = better compression but slower encoding/decoding. Default: 5.
	NumLevels int

	// NumLayers controls JPEG 2000 quality layers (progressive refinement). Default: 1.
	NumLayers int

	// TargetRatio optionally requests a compression ratio (orig_size / compressed_size).
	// If >0, quality will be estimated from this target.
	TargetRatio float64

	// QuantStepScale scales the quantization step derived from quality (>1 = more compression).
	// Default: 1.0 (no scaling).
	QuantStepScale float64

	// SubbandSteps allows explicit per-subbands quantization steps (lossy). Length must be 3*NumLevels+1 when set.
	SubbandSteps []float64

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewLossyParameters creates a new JPEG2000LossyParameters with default values.
func NewLossyParameters() *JPEG2000LossyParameters {
	return &JPEG2000LossyParameters{
		Quality:        80, // Default high quality
		NumLevels:      5,  // Default 5 decomposition levels
		NumLayers:      1,
		TargetRatio:    0,
		QuantStepScale: 1.0,
		SubbandSteps:   nil,
		params:         make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters).
func (p *JPEG2000LossyParameters) GetParameter(name string) interface{} {
	switch name {
	case "quality":
		return p.Quality
	case "numLevels":
		return p.NumLevels
	case "numLayers":
		return p.NumLayers
	case "targetRatio":
		return p.TargetRatio
	case "quantStepScale":
		return p.QuantStepScale
	case "subbandSteps":
		return p.SubbandSteps
	default:
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters).
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
	case "numLayers":
		if v, ok := value.(int); ok {
			p.NumLayers = v
		}
	case "targetRatio":
		if v, ok := value.(float64); ok {
			p.TargetRatio = v
		}
	case "quantStepScale":
		switch v := value.(type) {
		case float64:
			p.QuantStepScale = v
		case float32:
			p.QuantStepScale = float64(v)
		}
	case "subbandSteps":
		if v, ok := value.([]float64); ok {
			p.SubbandSteps = v
		}
	default:
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and normalizes values.
func (p *JPEG2000LossyParameters) Validate() error {
	if p.Quality < 1 || p.Quality > 100 {
		p.Quality = 80
	}
	if p.NumLevels < 0 || p.NumLevels > 6 {
		p.NumLevels = 5
	}
	if p.NumLayers < 1 {
		p.NumLayers = 1
	}
	if p.QuantStepScale <= 0 {
		p.QuantStepScale = 1.0
	}
	return nil
}

// WithQuality sets the quality and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithQuality(quality int) *JPEG2000LossyParameters {
	p.Quality = quality
	return p
}

// WithNumLevels sets the number of wavelet levels and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithNumLevels(levels int) *JPEG2000LossyParameters {
	p.NumLevels = levels
	return p
}

// WithNumLayers sets the number of quality layers and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithNumLayers(layers int) *JPEG2000LossyParameters {
	p.NumLayers = layers
	return p
}

// WithTargetRatio sets the desired compression ratio and returns the parameters for chaining.
// ratio = original_size / compressed_size (e.g., 5 means target ~5:1 compression).
func (p *JPEG2000LossyParameters) WithTargetRatio(ratio float64) *JPEG2000LossyParameters {
	p.TargetRatio = ratio
	return p
}

// WithQuantStepScale sets the global quantization step scale (>1 increases compression)
// and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithQuantStepScale(scale float64) *JPEG2000LossyParameters {
	p.QuantStepScale = scale
	return p
}

// WithSubbandSteps sets explicit per-subbands quantization steps and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithSubbandSteps(steps []float64) *JPEG2000LossyParameters {
	p.SubbandSteps = steps
	return p
}

func (p *JPEG2000LossyParameters) WithMCTBindings(bindings []jpeg2000.MCTBindingParams) *JPEG2000LossyParameters {
	p.SetParameter("mctBindings", bindings)
	return p
}
