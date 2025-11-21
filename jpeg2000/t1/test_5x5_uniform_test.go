package t1

import (
	"testing"
)

// Test5x5Uniform - Test 5x5 with uniform value
func Test5x5Uniform(t *testing.T) {
	testCases := []struct {
		name  string
		value int32
	}{
		{"uniform_minus128", -128},
		{"uniform_0", 0},
		{"uniform_100", 100},
		{"uniform_127", 127},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := 5
			numPixels := size * size
			input := make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				input[i] = tc.value
			}

			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane + 1) * 3

			encoder := NewT1Encoder(size, size, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			decoder := NewT1Decoder(size, size, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			errorCount := 0
			for i := range input {
				if decoded[i] != input[i] {
					errorCount++
					t.Logf("Error at position %d: expected %d, got %d", i, input[i], decoded[i])
				}
			}

			if errorCount > 0 {
				errorRate := float64(errorCount) / float64(numPixels) * 100
				t.Errorf("%s: %d/%d errors (%.1f%%)", tc.name, errorCount, numPixels, errorRate)
			} else {
				t.Logf("%s: PASS (0 errors)", tc.name)
			}
		})
	}
}
