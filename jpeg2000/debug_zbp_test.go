package jpeg2000

import (
	"testing"
)

// TestDebugZBP tests ZeroBitPlanes encoding/decoding specifically
func TestDebugZBP(t *testing.T) {
	// Create 24x24 image with all ones
	size := 24
	componentData := make([][]int32, 1)
	componentData[0] = make([]int32, size*size)
	for i := 0; i < size*size; i++ {
		componentData[0][i] = 1
	}

	// Encode
	params := DefaultEncodeParams(size, size, 1, 8, false)
	params.NumLevels = 0
	encoder := NewEncoder(params)

	t.Logf("Encoding %dx%d image with all ones (value=1)", size, size)
	t.Logf("BitDepth: %d", params.BitDepth)
	t.Logf("MaxBitplane for value=1 should be: 0")
	t.Logf("ZeroBitPlanes should be: %d - 1 - 0 = %d", params.BitDepth, params.BitDepth-1)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded to %d bytes", len(encoded))

	// Decode
	decoder := NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decodedData, err := decoder.GetComponentData(0)
	if err != nil {
		t.Fatalf("GetComponentData failed: %v", err)
	}

	// Check a few pixels
	t.Logf("First 10 pixels:")
	for i := 0; i < 10; i++ {
		orig := componentData[0][i]
		decoded := decodedData[i]
		match := "✓"
		if orig != decoded {
			match = "✗"
		}
		t.Logf("  [%d] orig=%d, decoded=%d %s", i, orig, decoded, match)
	}

	// Check if any errors
	errorCount := 0
	for i := 0; i < len(componentData[0]); i++ {
		if componentData[0][i] != decodedData[i] {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Found %d/%d errors", errorCount, len(componentData[0]))
	} else {
		t.Logf("Perfect reconstruction!")
	}
}
