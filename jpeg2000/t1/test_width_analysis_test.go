package t1

import (
	"fmt"
	"testing"
)

// TestWidthAnalysis - Check if issue is width-related
func TestWidthAnalysis(t *testing.T) {
	testCases := []struct {
		width, height int
	}{
		{4, 4}, // PASS
		{5, 5}, // FAIL
		{4, 5}, // ?
		{5, 4}, // ?
		{5, 1}, // ?
		{1, 5}, // ?
		{10, 1}, // ?
		{1, 10}, // ?
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%dx%d", tc.width, tc.height), func(t *testing.T) {
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
				t.Logf("%dx%d: %d/%d errors (%.1f%%)", tc.width, tc.height, errorCount, numPixels, errorRate)
			} else {
				t.Logf("%dx%d: PASS (0 errors)", tc.width, tc.height)
			}
		})
	}
}
