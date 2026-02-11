package lossy

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestNewLossyParameters(t *testing.T) {
	params := NewLossyParameters()

	if !params.Irreversible {
		t.Fatal("Default Irreversible should be true")
	}
	if params.Rate != 20 {
		t.Errorf("Default rate should be 20, got %d", params.Rate)
	}
	if len(params.RateLevels) != 9 {
		t.Errorf("Default rateLevels length should be 9, got %d", len(params.RateLevels))
	}
	if params.NumLevels != 5 {
		t.Errorf("Default numLevels should be 5, got %d", params.NumLevels)
	}
	if !params.AllowMCT {
		t.Fatal("Default AllowMCT should be true")
	}
}

func TestParametersInterface(t *testing.T) {
	var _ codec.Parameters = (*JPEG2000LossyParameters)(nil)

	params := NewLossyParameters()
	var genericParams codec.Parameters = params
	if genericParams == nil {
		t.Fatal("Parameters should implement codec.Parameters")
	}
}

func TestGetSetParameter(t *testing.T) {
	params := NewLossyParameters()
	params.SetParameter("rate", 18)
	params.SetParameter("numLevels", 3)
	params.SetParameter("irreversible", false)
	params.SetParameter("allowMCT", false)

	if got := params.GetParameter("rate"); got != 18 {
		t.Errorf("GetParameter(rate) = %v, want 18", got)
	}
	if got := params.GetParameter("numLevels"); got != 3 {
		t.Errorf("GetParameter(numLevels) = %v, want 3", got)
	}
	if got := params.GetParameter("irreversible"); got != false {
		t.Errorf("GetParameter(irreversible) = %v, want false", got)
	}
	if got := params.GetParameter("allowMCT"); got != false {
		t.Errorf("GetParameter(allowMCT) = %v, want false", got)
	}
}

func TestValidate(t *testing.T) {
	params := &JPEG2000LossyParameters{
		Rate:      -1,
		NumLevels: 10,
		NumLayers: 0,
		params:    make(map[string]interface{}),
	}

	if err := params.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if params.Rate != 20 {
		t.Errorf("After validation, Rate = %d, want 20", params.Rate)
	}
	if params.NumLevels != 5 {
		t.Errorf("After validation, NumLevels = %d, want 5", params.NumLevels)
	}
	if params.NumLayers != 1 {
		t.Errorf("After validation, NumLayers = %d, want 1", params.NumLayers)
	}
	if len(params.RateLevels) == 0 {
		t.Error("After validation, RateLevels should be defaulted")
	}
}

func TestChaining(t *testing.T) {
	params := NewLossyParameters().
		WithIrreversible(true).
		WithRate(12).
		WithNumLevels(3).
		WithAllowMCT(false)

	if !params.Irreversible {
		t.Error("Irreversible should be true")
	}
	if params.Rate != 12 {
		t.Errorf("Rate = %d, want 12", params.Rate)
	}
	if params.NumLevels != 3 {
		t.Errorf("NumLevels = %d, want 3", params.NumLevels)
	}
	if params.AllowMCT {
		t.Error("AllowMCT should be false")
	}
}

func TestCustomParameters(t *testing.T) {
	params := NewLossyParameters()
	params.SetParameter("customKey", "customValue")
	if got := params.GetParameter("customKey"); got != "customValue" {
		t.Errorf("Custom parameter = %v, want customValue", got)
	}
}
