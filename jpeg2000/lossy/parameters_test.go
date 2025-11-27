package lossy

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestNewLossyParameters(t *testing.T) {
	params := NewLossyParameters()

	if params.Quality != 80 {
		t.Errorf("Default quality should be 80, got %d", params.Quality)
	}

	if params.NumLevels != 5 {
		t.Errorf("Default numLevels should be 5, got %d", params.NumLevels)
	}
}

func TestParametersInterface(t *testing.T) {
	var _ codec.Parameters = (*JPEG2000LossyParameters)(nil)

	params := NewLossyParameters()

	// Test type assertion
	var genericParams codec.Parameters = params
	if genericParams == nil {
		t.Fatal("Parameters should implement codec.Parameters")
	}
}

func TestGetParameter(t *testing.T) {
	params := NewLossyParameters()
	params.Quality = 95
	params.NumLevels = 3

	tests := []struct {
		name     string
		expected interface{}
	}{
		{"quality", 95},
		{"numLevels", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := params.GetParameter(tt.name)
			if got != tt.expected {
				t.Errorf("GetParameter(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestSetParameter(t *testing.T) {
	params := NewLossyParameters()

	tests := []struct {
		name  string
		value interface{}
		check func() bool
	}{
		{"quality", 95, func() bool { return params.Quality == 95 }},
		{"numLevels", 3, func() bool { return params.NumLevels == 3 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params.SetParameter(tt.name, tt.value)
			if !tt.check() {
				t.Errorf("SetParameter(%q, %v) failed", tt.name, tt.value)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		quality        int
		numLevels      int
		expectedQuality int
		expectedLevels  int
	}{
		{"Valid quality", 80, 5, 80, 5},
		{"Quality too low", -10, 5, 80, 5},
		{"Quality too high", 200, 5, 80, 5},
		{"NumLevels too low", 80, -1, 80, 5},
		{"NumLevels too high", 80, 10, 80, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &JPEG2000LossyParameters{
				Quality:   tt.quality,
				NumLevels: tt.numLevels,
				params:    make(map[string]interface{}),
			}

			params.Validate()

			if params.Quality != tt.expectedQuality {
				t.Errorf("After validation, Quality = %d, want %d", params.Quality, tt.expectedQuality)
			}

			if params.NumLevels != tt.expectedLevels {
				t.Errorf("After validation, NumLevels = %d, want %d", params.NumLevels, tt.expectedLevels)
			}
		})
	}
}

func TestChaining(t *testing.T) {
	params := NewLossyParameters().
		WithQuality(95).
		WithNumLevels(3)

	if params.Quality != 95 {
		t.Errorf("Quality = %d, want 95", params.Quality)
	}

	if params.NumLevels != 3 {
		t.Errorf("NumLevels = %d, want 3", params.NumLevels)
	}
}

func TestCustomParameters(t *testing.T) {
	params := NewLossyParameters()

	// Set custom parameter
	params.SetParameter("customKey", "customValue")

	// Get custom parameter
	got := params.GetParameter("customKey")
	if got != "customValue" {
		t.Errorf("Custom parameter = %v, want %q", got, "customValue")
	}

	// Get non-existent parameter
	notFound := params.GetParameter("nonExistent")
	if notFound != nil {
		t.Errorf("Non-existent parameter should be nil, got %v", notFound)
	}
}

func TestTypeSafety(t *testing.T) {
	params := NewLossyParameters()

	// Test that setting wrong type doesn't panic
	params.SetParameter("quality", "not an int") // Should be ignored

	if params.Quality != 80 { // Should still be default
		t.Errorf("Quality should remain 80 after invalid type, got %d", params.Quality)
	}

	// Test valid int
	params.SetParameter("quality", 95)
	if params.Quality != 95 {
		t.Errorf("Quality should be 95, got %d", params.Quality)
	}
}
