package htj2k

import (
	"testing"
)

func TestNewHTJ2KParameters(t *testing.T) {
	params := NewHTJ2KParameters()

	if params.Quality != 80 {
		t.Errorf("Default Quality = %d, want 80", params.Quality)
	}
	if params.BlockWidth != 64 {
		t.Errorf("Default BlockWidth = %d, want 64", params.BlockWidth)
	}
	if params.BlockHeight != 64 {
		t.Errorf("Default BlockHeight = %d, want 64", params.BlockHeight)
	}
	if params.NumLevels != 3 {
		t.Errorf("Default NumLevels = %d, want 3", params.NumLevels)
	}
}

func TestNewHTJ2KLosslessParameters(t *testing.T) {
	params := NewHTJ2KLosslessParameters()

	if params.Quality != 100 {
		t.Errorf("Lossless Quality = %d, want 100", params.Quality)
	}
	if params.NumLevels != 0 {
		t.Errorf("Default NumLevels = %d, want 0", params.NumLevels)
	}
}

func TestHTJ2KParameters_GetParameter(t *testing.T) {
	params := NewHTJ2KParameters()
	params.Quality = 90
	params.BlockWidth = 32
	params.BlockHeight = 32
	params.NumLevels = 3

	tests := []struct {
		name  string
		param string
		want  interface{}
	}{
		{"quality", "quality", 90},
		{"blockWidth", "blockWidth", 32},
		{"blockHeight", "blockHeight", 32},
		{"numLevels", "numLevels", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := params.GetParameter(tt.param)
			if got != tt.want {
				t.Errorf("GetParameter(%q) = %v, want %v", tt.param, got, tt.want)
			}
		})
	}
}

func TestHTJ2KParameters_SetParameter(t *testing.T) {
	params := NewHTJ2KParameters()

	tests := []struct {
		name  string
		param string
		value interface{}
		check func() interface{}
	}{
		{
			name:  "quality",
			param: "quality",
			value: 50,
			check: func() interface{} { return params.Quality },
		},
		{
			name:  "blockWidth",
			param: "blockWidth",
			value: 128,
			check: func() interface{} { return params.BlockWidth },
		},
		{
			name:  "blockHeight",
			param: "blockHeight",
			value: 128,
			check: func() interface{} { return params.BlockHeight },
		},
		{
			name:  "numLevels",
			param: "numLevels",
			value: 6,
			check: func() interface{} { return params.NumLevels },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params.SetParameter(tt.param, tt.value)
			got := tt.check()
			if got != tt.value {
				t.Errorf("After SetParameter(%q, %v), got %v", tt.param, tt.value, got)
			}
		})
	}
}

func TestHTJ2KParameters_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*HTJ2KParameters)
		wantQuality int
		wantBW      int
		wantBH      int
		wantLevels  int
	}{
		{
			name: "Valid parameters",
			setup: func(p *HTJ2KParameters) {
				p.Quality = 80
				p.BlockWidth = 64
				p.BlockHeight = 64
				p.NumLevels = 5
			},
			wantQuality: 80,
			wantBW:      64,
			wantBH:      64,
			wantLevels:  5,
		},
		{
			name: "Quality too low",
			setup: func(p *HTJ2KParameters) {
				p.Quality = 0
			},
			wantQuality: 1,
			wantBW:      64,
			wantBH:      64,
			wantLevels:  3,
		},
		{
			name: "Quality too high",
			setup: func(p *HTJ2KParameters) {
				p.Quality = 150
			},
			wantQuality: 100,
			wantBW:      64,
			wantBH:      64,
			wantLevels:  3,
		},
		{
			name: "BlockWidth too small",
			setup: func(p *HTJ2KParameters) {
				p.BlockWidth = 2
			},
			wantQuality: 80,
			wantBW:      4,
			wantBH:      64,
			wantLevels:  3,
		},
		{
			name: "BlockWidth not power of 2",
			setup: func(p *HTJ2KParameters) {
				p.BlockWidth = 100 // Should round to 128
			},
			wantQuality: 80,
			wantBW:      128,
			wantBH:      64,
			wantLevels:  3,
		},
		{
			name: "BlockHeight too large",
			setup: func(p *HTJ2KParameters) {
				p.BlockHeight = 2000
			},
			wantQuality: 80,
			wantBW:      64,
			wantBH:      1024,
			wantLevels:  3,
		},
		{
			name: "NumLevels negative",
			setup: func(p *HTJ2KParameters) {
				p.NumLevels = -1
			},
			wantQuality: 80,
			wantBW:      64,
			wantBH:      64,
			wantLevels:  0,
		},
		{
			name: "NumLevels too high",
			setup: func(p *HTJ2KParameters) {
				p.NumLevels = 10
			},
			wantQuality: 80,
			wantBW:      64,
			wantBH:      64,
			wantLevels:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := NewHTJ2KParameters()
			tt.setup(params)
			err := params.Validate()
			if err != nil {
				t.Errorf("Validate() returned error: %v", err)
			}

			if params.Quality != tt.wantQuality {
				t.Errorf("Quality = %d, want %d", params.Quality, tt.wantQuality)
			}
			if params.BlockWidth != tt.wantBW {
				t.Errorf("BlockWidth = %d, want %d", params.BlockWidth, tt.wantBW)
			}
			if params.BlockHeight != tt.wantBH {
				t.Errorf("BlockHeight = %d, want %d", params.BlockHeight, tt.wantBH)
			}
			if params.NumLevels != tt.wantLevels {
				t.Errorf("NumLevels = %d, want %d", params.NumLevels, tt.wantLevels)
			}
		})
	}
}

func TestHTJ2KParameters_Chaining(t *testing.T) {
	params := NewHTJ2KParameters().
		WithQuality(90).
		WithBlockSize(128, 128).
		WithNumLevels(6)

	if params.Quality != 90 {
		t.Errorf("Quality = %d, want 90", params.Quality)
	}
	if params.BlockWidth != 128 {
		t.Errorf("BlockWidth = %d, want 128", params.BlockWidth)
	}
	if params.BlockHeight != 128 {
		t.Errorf("BlockHeight = %d, want 128", params.BlockHeight)
	}
	if params.NumLevels != 6 {
		t.Errorf("NumLevels = %d, want 6", params.NumLevels)
	}
}

func TestNearestPowerOf2(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 4},
		{6, 8},
		{7, 8},
		{8, 8},
		{10, 8},
		{12, 16},
		{15, 16},
		{16, 16},
		{20, 16},
		{24, 32},
		{32, 32},
		{48, 64},
		{64, 64},
		{96, 128},
		{100, 128},
		{128, 128},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := nearestPowerOf2(tt.input)
			if got != tt.want {
				t.Errorf("nearestPowerOf2(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
