package lossless

import (
	"fmt"
	"testing"
)

// TestWrapAround tests the lossless encoder with wrap-around pattern
func TestWrapAround(t *testing.T) {
	width, height := 10, 5
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, 8)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Lossless encoded size: %d bytes\n", len(encoded))

	// Decode
	decoded, w, h, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded %dx%d\n", w, h)

	// Check for lossless
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(decoded[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	fmt.Printf("Max error: %d\n", maxError)

	if maxError > 0 {
		t.Errorf("Lossless mode has error=%d, want 0", maxError)
	}
}
