package t1

import (
	"testing"
)

// TestMinimalFailing - try to find the absolute minimal failing case
func TestMinimalFailing(t *testing.T) {
	// We know 5x5 fails. Let's try smaller variants to find the minimal failing case.

	testCases := []struct {
		width  int
		height int
		name   string
	}{
		// Test width=5 with varying height
		{5, 1, "5x1"},
		{5, 2, "5x2"},
		{5, 3, "5x3"},
		{5, 4, "5x4"},
		{5, 5, "5x5"},
		{5, 6, "5x6"},
		// Test height=5 with varying width
		{1, 5, "1x5"},
		{2, 5, "2x5"},
		{3, 5, "3x5"},
		{4, 5, "4x5"},
		{6, 5, "6x5"},
		// Test 10x10 and variants
		{10, 10, "10x10"},
		{10, 9, "10x9"},
		{9, 10, "9x10"},
		{10, 1, "10x1"},
		{1, 10, "1x10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			numPixels := tc.width * tc.height
			input := make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane + 1) * 3

			encoder := NewT1Encoder(tc.width, tc.height, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			decoder := NewT1Decoder(tc.width, tc.height, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			errorCount := 0
			for i := range input {
				if decoded[i] != input[i] {
					errorCount++
				}
			}

			errorRate := float64(errorCount) / float64(numPixels) * 100
			if errorCount > 0 {
				t.Errorf("%s: %d/%d errors (%.1f%%)", tc.name, errorCount, numPixels, errorRate)
			} else {
				t.Logf("%s: ✓ Perfect", tc.name)
			}
		})
	}
}
