package jpeg2000

import (
	"fmt"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// TestCoefficientComparison compares encoded vs decoded coefficients
func TestCoefficientComparison192x192(t *testing.T) {
	width, height := 192, 192
	numLevels := 1

	// Create gradient pattern
	original := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			original[y*width+x] = int32((x + y) % 256)
		}
	}

	// Apply DC level shift and wavelet transform (same as encoder)
	encoderCoeffs := make([]int32, width*height)
	copy(encoderCoeffs, original)

	// DC level shift (encoder does this)
	for i := range encoderCoeffs {
		encoderCoeffs[i] -= 128
	}

	// Apply forward wavelet transform
	wavelet.ForwardMultilevel(encoderCoeffs, width, height, numLevels)

	// Now encode and decode
	componentData := make([][]int32, 1)
	componentData[0] = original // Use unsigned original

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = numLevels
	encoder := NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	decoder := NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// Get the coefficients from the decoder (before IDWT)
	// We need to access the internal state - let me check the decoder structure

	// Compare final decoded pixels first
	decoded, err := decoder.GetComponentData(0)
	if err != nil {
		t.Fatalf("Failed to get decoded data: %v", err)
	}

	// Find mismatches
	mismatchCount := 0
	var firstMismatches []string
	for i := 0; i < len(original); i++ {
		if decoded[i] != original[i] {
			if mismatchCount < 10 {
				x := i % width
				y := i / width
				diff := decoded[i] - original[i]
				firstMismatches = append(firstMismatches,
					fmt.Sprintf("  Pixel (%d,%d): expected=%d got=%d diff=%d",
						x, y, original[i], decoded[i], diff))
			}
			mismatchCount++
		}
	}

	if mismatchCount > 0 {
		t.Logf("Pixel mismatches:\\n%s", firstMismatches)
		t.Errorf("Found %d pixel mismatches", mismatchCount)
	} else {
		t.Log("✓ All pixels match")
	}
}
