package htj2k

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

// TestBaselineEBCOT tests that EBCOT T1 works correctly (baseline)
func TestBaselineEBCOT(t *testing.T) {
	width := 8
	height := 8

	// Create int32 component data
	componentData := make([][]int32, 1)
	componentData[0] = make([]int32, width*height)
	for i := 0; i < len(componentData[0]); i++ {
		componentData[0][i] = int32(i % 256)
	}

	t.Logf("Input data (first 16 pixels): %v", componentData[0][:16])

	// Encode with EBCOT T1 (default)
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 0 // No DWT
	// Don't set BlockEncoderFactory - use default EBCOT T1
	encoder := jpeg2000.NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded size: %d bytes", len(encoded))

	// Decode with EBCOT T1 (default)
	decoder := jpeg2000.NewDecoder()
	// Don't set BlockDecoderFactory - use default EBCOT T1
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded, err := decoder.GetComponentData(0)
	if err != nil {
		t.Fatalf("GetComponentData failed: %v", err)
	}

	t.Logf("Decoded data (first 16 pixels): %v", decoded[:16])

	// Check errors
	errors := 0
	for i := 0; i < len(componentData[0]); i++ {
		if componentData[0][i] != decoded[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d: expected %d, got %d", i, componentData[0][i], decoded[i])
			}
		}
	}
	if errors > 0 {
		t.Errorf("Total errors: %d/%d", errors, len(componentData[0]))
	} else {
		t.Logf("✓ Perfect reconstruction with EBCOT T1")
	}
}
