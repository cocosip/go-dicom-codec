package t1

import (
	"fmt"
	"testing"
)

// TestSquareAnalysis - Systematically test all square sizes 1-25
func TestSquareAnalysis(t *testing.T) {
	results := make(map[int]string)

	for size := 1; size <= 25; size++ {
		numPixels := size * size
		input := make([]int32, numPixels)
		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(size, size, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("%dx%d: Encoding failed: %v", size, size, err)
		}

		decoder := NewT1Decoder(size, size, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("%dx%d: Decoding failed: %v", size, size, err)
		}

		decoded := decoder.GetData()

		errorCount := 0
		for i := range input {
			if decoded[i] != input[i] {
				errorCount++
			}
		}

		if errorCount > 0 {
			errorRate := float64(errorCount) / float64(numPixels) * 100
			results[size] = fmt.Sprintf("FAIL (%.1f%%)", errorRate)
		} else {
			results[size] = "PASS"
		}
	}

	// Print summary
	t.Log("\n=== SUMMARY ===")
	passing := []int{}
	failing := []int{}
	for size := 1; size <= 25; size++ {
		result := results[size]
		t.Logf("%dx%d: %s", size, size, result)
		if result == "PASS" {
			passing = append(passing, size)
		} else {
			failing = append(failing, size)
		}
	}

	t.Logf("\nPassing sizes: %v", passing)
	t.Logf("Failing sizes: %v", failing)
}
