package t1

import (
	"testing"
)

// TestCheckMaxBitplane checks what maxBitplane should be for gradient patterns
func TestCheckMaxBitplane(t *testing.T) {
	patterns := []struct {
		size int
		name string
	}{
		{4, "4x4"},
		{5, "5x5"},
		{8, "8x8"},
		{17, "17x17"},
		{32, "32x32"},
	}

	for _, p := range patterns {
		t.Run(p.name, func(t *testing.T) {
			numPixels := p.size * p.size
			input := make([]int32, numPixels)

			// Gradient pattern same as failing tests
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			maxBP := CalculateMaxBitplane(input)
			t.Logf("%s: maxBitplane = %d", p.name, maxBP)

			// Also log first few values
			t.Logf("  First values: %v", input[:min(10, len(input))])
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
