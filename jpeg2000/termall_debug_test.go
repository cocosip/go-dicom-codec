package jpeg2000

import (
	"fmt"
	"testing"
)

// TestTERMALLDebug - minimal debugging test for TERMALL multi-layer
func TestTERMALLDebug(t *testing.T) {
	// Very small image
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	// Simple gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// 2-layer lossless (should enable TERMALL)
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 2
	params.Lossless = true
	params.NumLevels = 5

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("Encoded: %d bytes", len(encoded))

	// Decode
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// Get decoded data
	decodedPixels := decoder.GetPixelData()

	// Calculate error
	maxError := 0
	errorCount := 0
	firstErrors := make([]string, 0)
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedPixels[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		if diff > 0 {
			errorCount++
			if len(firstErrors) < 5 {
				firstErrors = append(firstErrors,
					fmt.Sprintf("pixel[%d] expected=%d got=%d diff=%d",
						i, pixelData[i], decodedPixels[i], diff))
			}
		}
	}

	t.Logf("Max error: %d, error count: %d/%d", maxError, errorCount, len(pixelData))
	for _, msg := range firstErrors {
		t.Logf("  %s", msg)
	}

	if maxError > 0 {
		t.Errorf("TERMALL multi-layer lossless should have error=0, got error=%d", maxError)
	}
}
