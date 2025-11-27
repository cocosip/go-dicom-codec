package jpeg2000

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
	_ "fmt" // For future debugging
)

// TestCoefficientArrayComparison compares coefficient arrays before IDWT
func TestCoefficientArrayComparison192x192(t *testing.T) {
	width, height := 192, 192
	numLevels := 1

	// Create original data
	original := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			original[y*width+x] = int32((x + y) % 256)
		}
	}

	// Simulate encoder: DC shift + forward wavelet
	encoderCoeffs := make([]int32, width*height)
	copy(encoderCoeffs, original)
	for i := range encoderCoeffs {
		encoderCoeffs[i] -= 128 // DC level shift
	}
	wavelet.ForwardMultilevel(encoderCoeffs, width, height, numLevels)

	// Now do full encode/decode
	componentData := make([][]int32, 1)
	componentData[0] = original

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

	// We need to access the decoder's internal coefficient array before IDWT
	// This requires modifying the decoder or using reflection
	// For now, let's just verify the final output matches

	decoded, err := decoder.GetComponentData(0)
	if err != nil {
		t.Fatalf("Failed to get decoded data: %v", err)
	}

	// Compare final pixels
	mismatchCount := 0
	firstMismatch := -1
	for i := 0; i < len(original); i++ {
		if decoded[i] != original[i] {
			if firstMismatch < 0 {
				firstMismatch = i
			}
			mismatchCount++
		}
	}

	if mismatchCount > 0 {
		x := firstMismatch % width
		y := firstMismatch / width
		t.Logf("First mismatch at (%d,%d): expected=%d got=%d",
			x, y, original[firstMismatch], decoded[firstMismatch])
		t.Errorf("Found %d pixel mismatches out of %d", mismatchCount, len(original))
	} else {
		t.Log("✓ Perfect match")
	}
}
