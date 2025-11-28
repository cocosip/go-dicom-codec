package jpeg2000

import (
	"fmt"
	"testing"
)

// TestTERMALLCompareLayers - compare single vs multi layer encoding
func TestTERMALLCompareLayers(t *testing.T) {
	// Very small image
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	// Simple gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Test 1-layer
	params1 := DefaultEncodeParams(width, height, 1, 8, false)
	params1.NumLayers = 1
	params1.Lossless = true
	params1.NumLevels = 5

	encoder1 := NewEncoder(params1)
	encoded1, err := encoder1.Encode(pixelData)
	if err != nil {
		t.Fatalf("1-layer encoding failed: %v", err)
	}

	// Decode 1-layer
	decoder1 := NewDecoder()
	if err := decoder1.Decode(encoded1); err != nil {
		t.Fatalf("1-layer decoding failed: %v", err)
	}
	decoded1 := decoder1.GetPixelData()

	// Test 2-layer
	params2 := DefaultEncodeParams(width, height, 1, 8, false)
	params2.NumLayers = 2
	params2.Lossless = true
	params2.NumLevels = 5

	encoder2 := NewEncoder(params2)
	encoded2, err := encoder2.Encode(pixelData)
	if err != nil {
		t.Fatalf("2-layer encoding failed: %v", err)
	}

	// Decode 2-layer
	decoder2 := NewDecoder()
	if err := decoder2.Decode(encoded2); err != nil {
		t.Fatalf("2-layer decoding failed: %v", err)
	}
	decoded2 := decoder2.GetPixelData()

	// Calculate errors
	maxError1, errorCount1 := 0, 0
	maxError2, errorCount2 := 0, 0

	for i := 0; i < len(pixelData); i++ {
		diff1 := int(pixelData[i]) - int(decoded1[i])
		if diff1 < 0 {
			diff1 = -diff1
		}
		if diff1 > maxError1 {
			maxError1 = diff1
		}
		if diff1 > 0 {
			errorCount1++
		}

		diff2 := int(pixelData[i]) - int(decoded2[i])
		if diff2 < 0 {
			diff2 = -diff2
		}
		if diff2 > maxError2 {
			maxError2 = diff2
		}
		if diff2 > 0 {
			errorCount2++
		}
	}

	t.Logf("1-layer: maxError=%d, errorCount=%d/%d", maxError1, errorCount1, len(pixelData))
	t.Logf("2-layer: maxError=%d, errorCount=%d/%d", maxError2, errorCount2, len(pixelData))

	if maxError1 > 0 {
		t.Errorf("1-layer should have error=0, got error=%d", maxError1)
	}
	if maxError2 > 0 {
		// Print first few errors for debugging
		for i := 0; i < len(pixelData) && i < 10; i++ {
			diff := int(pixelData[i]) - int(decoded2[i])
			if diff != 0 {
				fmt.Printf("  pixel[%d]: expected=%d got=%d diff=%d\n",
					i, pixelData[i], decoded2[i], diff)
			}
		}
		t.Errorf("2-layer should have error=0, got error=%d", maxError2)
	}
}
