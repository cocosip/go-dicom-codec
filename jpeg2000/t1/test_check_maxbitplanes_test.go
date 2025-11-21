package t1

import (
	"testing"
)

// TestCheckMaxBitplanes - check what maxBitplane is calculated for each size
func TestCheckMaxBitplanes(t *testing.T) {
	sizes := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	for _, size := range sizes {
		numPixels := size * size
		input := make([]int32, numPixels)
		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		maxBitplane := CalculateMaxBitplane(input)

		// Also check the max absolute value
		maxAbs := int32(0)
		for _, val := range input {
			abs := val
			if abs < 0 {
				abs = -abs
			}
			if abs > maxAbs {
				maxAbs = abs
			}
		}

		t.Logf("%dx%d: maxBitplane=%d, maxAbsValue=%d", size, size, maxBitplane, maxAbs)
	}
}
