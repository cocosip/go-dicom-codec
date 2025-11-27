package jpeg2000

import (
	"testing"
)

// TestDebugPassData debugs pass data and slicing
func TestDebugPassData(t *testing.T) {
	width, height := 8, 8
	pixelData := make([]byte, width*height)
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	// Test 2-layer encoding
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 2
	params.Lossless = true
	params.NumLevels = 2

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Try to extract and print pass information from the encoder
	// This requires accessing internal state or adding debug methods
	t.Log("Pass information would be printed here if we had access to it")
}
