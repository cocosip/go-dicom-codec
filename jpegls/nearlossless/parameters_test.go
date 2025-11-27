package nearlossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestNewNearLosslessParameters(t *testing.T) {
	params := NewNearLosslessParameters()

	if params.NEAR != 2 {
		t.Errorf("Default NEAR = %d, want 2", params.NEAR)
	}

	if params.params == nil {
		t.Error("params map should be initialized")
	}
}

func TestNearLosslessParameters_GetParameter(t *testing.T) {
	params := NewNearLosslessParameters()
	params.NEAR = 5

	tests := []struct {
		name     string
		paramKey string
		want     interface{}
	}{
		{"NEAR parameter", "near", 5},
		{"Unknown parameter", "unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := params.GetParameter(tt.paramKey)
			if got != tt.want {
				t.Errorf("GetParameter(%q) = %v, want %v", tt.paramKey, got, tt.want)
			}
		})
	}
}

func TestNearLosslessParameters_SetParameter(t *testing.T) {
	params := NewNearLosslessParameters()

	params.SetParameter("near", 10)
	if params.NEAR != 10 {
		t.Errorf("After SetParameter(near, 10), NEAR = %d, want 10", params.NEAR)
	}

	// Test custom parameter
	params.SetParameter("custom", "value")
	if params.params["custom"] != "value" {
		t.Error("Custom parameter not set correctly")
	}
}

func TestNearLosslessParameters_Validate(t *testing.T) {
	tests := []struct {
		name        string
		inputNEAR   int
		expectedNEAR int
	}{
		{"Valid NEAR 0", 0, 0},
		{"Valid NEAR 1", 1, 1},
		{"Valid NEAR 2", 2, 2},
		{"Valid NEAR 10", 10, 10},
		{"Valid NEAR 255", 255, 255},
		{"Invalid NEAR -1", -1, 2}, // Reset to default
		{"Invalid NEAR 256", 256, 2}, // Reset to default
		{"Invalid NEAR 1000", 1000, 2}, // Reset to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := NewNearLosslessParameters()
			params.NEAR = tt.inputNEAR
			err := params.Validate()
			if err != nil {
				t.Errorf("Validate() returned error: %v", err)
			}
			if params.NEAR != tt.expectedNEAR {
				t.Errorf("After Validate(), NEAR = %d, want %d", params.NEAR, tt.expectedNEAR)
			}
		})
	}
}

func TestNearLosslessParameters_WithNEAR(t *testing.T) {
	params := NewNearLosslessParameters()

	// Test method chaining
	result := params.WithNEAR(5)

	// Should return same instance
	if result != params {
		t.Error("WithNEAR should return same instance for chaining")
	}

	if params.NEAR != 5 {
		t.Errorf("NEAR = %d, want 5", params.NEAR)
	}

	// Test chaining multiple calls
	params = NewNearLosslessParameters().WithNEAR(10)
	if params.NEAR != 10 {
		t.Errorf("Chained NEAR = %d, want 10", params.NEAR)
	}
}

func TestNearLosslessParameters_ImplementsCodecParameters(t *testing.T) {
	var _ codec.Parameters = (*JPEGLSNearLosslessParameters)(nil)
	var _ codec.Parameters = NewNearLosslessParameters()
}

func TestNearLosslessParameters_TypeSafety(t *testing.T) {
	// Demonstrate type-safe usage
	params := NewNearLosslessParameters()
	params.NEAR = 3 // Type-safe, IDE autocomplete works

	// Should equal GetParameter
	if params.GetParameter("near") != params.NEAR {
		t.Error("Direct field access and GetParameter should return same value")
	}

	// Test that SetParameter updates field
	params.SetParameter("near", 7)
	if params.NEAR != 7 {
		t.Error("SetParameter should update NEAR field")
	}
}

func TestNearLosslessParameters_CustomParameters(t *testing.T) {
	params := NewNearLosslessParameters()

	// Standard parameter
	params.NEAR = 5

	// Custom parameter
	params.SetParameter("customKey", "customValue")

	// Both should work
	if params.NEAR != 5 {
		t.Errorf("NEAR = %d, want 5", params.NEAR)
	}

	if params.GetParameter("customKey") != "customValue" {
		t.Error("Custom parameter not retrieved correctly")
	}
}

// BenchmarkTypeSafeVsStringBased compares performance of type-safe vs string-based parameter access
func BenchmarkTypeSafeVsStringBased(b *testing.B) {
	b.Run("TypeSafe", func(b *testing.B) {
		params := NewNearLosslessParameters()
		for i := 0; i < b.N; i++ {
			params.NEAR = 5
			_ = params.NEAR
		}
	})

	b.Run("StringBased", func(b *testing.B) {
		params := NewNearLosslessParameters()
		for i := 0; i < b.N; i++ {
			params.SetParameter("near", 5)
			_ = params.GetParameter("near")
		}
	})
}
