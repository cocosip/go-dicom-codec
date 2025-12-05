package htj2k

import "github.com/cocosip/go-dicom/pkg/imaging/codec"

// Ensure HTJ2KParameters implements codec.Parameters
var _ codec.Parameters = (*HTJ2KParameters)(nil)

// HTJ2KParameters contains parameters for HTJ2K (High-Throughput JPEG 2000) compression
type HTJ2KParameters struct {
	// Quality controls lossy compression quality (1-100)
	// - 1: Maximum compression, lowest quality
	// - 50: Medium quality
	// - 80: High quality (default)
	// - 100: Near-lossless, highest quality
	//
	// Only applies to lossy encoding. For lossless, this parameter is ignored.
	Quality int

	// BlockWidth specifies the code-block width
	// Default: 64 (or image width for small images)
	// Valid range: 4-1024 (must be power of 2)
	BlockWidth int

	// BlockHeight specifies the code-block height
	// Default: 64 (or image height for small images)
	// Valid range: 4-1024 (must be power of 2)
	BlockHeight int

	// NumLevels controls the number of wavelet decomposition levels (0-6)
	// - 0: No decomposition (minimal compression, fastest)
	// - 1: Single-level decomposition
	// - 3: Medium levels (good balance)
	// - 5: Default, recommended for most images
	// - 6: Maximum levels (best compression for large images)
	NumLevels int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewHTJ2KParameters creates a new HTJ2KParameters with default values
func NewHTJ2KParameters() *HTJ2KParameters {
	return &HTJ2KParameters{
		Quality:     80, // Default quality 80 for lossy
		BlockWidth:  64, // Default block width
		BlockHeight: 64, // Default block height
		NumLevels:   3,  // Conservative default (TODO: fix HTJ2K block coding issue for >64x64 images with gradient patterns)
		params:      make(map[string]interface{}),
	}
}

// NewHTJ2KLosslessParameters creates parameters optimized for lossless encoding
func NewHTJ2KLosslessParameters() *HTJ2KParameters {
	return &HTJ2KParameters{
		Quality:     100, // Quality 100 for lossless
		BlockWidth:  64,
		BlockHeight: 64,
		NumLevels:   3, // Conservative default (TODO: fix HTJ2K block coding issue for >64x64 images with gradient patterns)
		params:      make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *HTJ2KParameters) GetParameter(name string) interface{} {
	switch name {
	case "quality":
		return p.Quality
	case "blockWidth":
		return p.BlockWidth
	case "blockHeight":
		return p.BlockHeight
	case "numLevels":
		return p.NumLevels
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *HTJ2KParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "quality":
		if v, ok := value.(int); ok {
			p.Quality = v
		}
	case "blockWidth":
		if v, ok := value.(int); ok {
			p.BlockWidth = v
		}
	case "blockHeight":
		if v, ok := value.(int); ok {
			p.BlockHeight = v
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

// Validate checks if the parameters are valid and adjusts them if needed
func (p *HTJ2KParameters) Validate() error {
	// Quality must be in range 1-100
	if p.Quality < 1 {
		p.Quality = 1
	} else if p.Quality > 100 {
		p.Quality = 100
	}

	// BlockWidth must be power of 2 and in range 4-1024
	if p.BlockWidth < 4 {
		p.BlockWidth = 4
	} else if p.BlockWidth > 1024 {
		p.BlockWidth = 1024
	}
	// Round to nearest power of 2
	p.BlockWidth = nearestPowerOf2(p.BlockWidth)

	// BlockHeight must be power of 2 and in range 4-1024
	if p.BlockHeight < 4 {
		p.BlockHeight = 4
	} else if p.BlockHeight > 1024 {
		p.BlockHeight = 1024
	}
	// Round to nearest power of 2
	p.BlockHeight = nearestPowerOf2(p.BlockHeight)

	// NumLevels must be in range 0-6
	if p.NumLevels < 0 {
		p.NumLevels = 0
	} else if p.NumLevels > 6 {
		p.NumLevels = 6
	}

	return nil
}

// WithQuality sets the quality and returns the parameters for chaining
func (p *HTJ2KParameters) WithQuality(quality int) *HTJ2KParameters {
	p.Quality = quality
	return p
}

// WithBlockSize sets both block width and height and returns the parameters for chaining
func (p *HTJ2KParameters) WithBlockSize(width, height int) *HTJ2KParameters {
	p.BlockWidth = width
	p.BlockHeight = height
	return p
}

// WithNumLevels sets the number of decomposition levels and returns the parameters for chaining
func (p *HTJ2KParameters) WithNumLevels(numLevels int) *HTJ2KParameters {
	p.NumLevels = numLevels
	return p
}

// nearestPowerOf2 returns the nearest power of 2 to the given value
func nearestPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}

	// Find the highest bit set
	power := 1
	for power < n {
		power <<= 1
	}

	// Check if previous power of 2 is closer
	prevPower := power >> 1
	if n-prevPower < power-n {
		return prevPower
	}
	return power
}
