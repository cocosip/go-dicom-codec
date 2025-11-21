package t1

import (
	"fmt"
	"testing"
)

// Test5x5FirstError - Find the FIRST error in 5x5 gradient
func Test5x5FirstError(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Gradient pattern (same as TestSquareSizes)
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	// Print input pattern
	t.Log("\nInput 5x5 gradient pattern:")
	for y := 0; y < size; y++ {
		line := ""
		for x := 0; x < size; x++ {
			idx := y*size + x
			line += fmt.Sprintf("%5d ", input[idx])
		}
		t.Log(line)
	}

	maxBitplane := CalculateMaxBitplane(input)
	t.Logf("\nMaxBitplane: %d", maxBitplane)
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

	// Print decoded pattern
	t.Log("\nDecoded 5x5 pattern:")
	for y := 0; y < size; y++ {
		line := ""
		for x := 0; x < size; x++ {
			idx := y*size + x
			line += fmt.Sprintf("%5d ", decoded[idx])
		}
		t.Log(line)
	}

	// Find ALL errors
	t.Log("\nErrors:")
	firstErrorIdx := -1
	errorCount := 0
	for i := range input {
		if decoded[i] != input[i] {
			y := i / size
			x := i % size
			t.Logf("  Position (%d,%d) idx=%d: expected %d, got %d (diff=%d)",
				x, y, i, input[i], decoded[i], decoded[i]-input[i])
			errorCount++
			if firstErrorIdx == -1 {
				firstErrorIdx = i
			}
		}
	}

	t.Logf("\nTotal errors: %d/%d (%.1f%%)", errorCount, numPixels,
		float64(errorCount)/float64(numPixels)*100)

	if firstErrorIdx >= 0 {
		t.Logf("FIRST error at index %d (position %d,%d)",
			firstErrorIdx, firstErrorIdx%size, firstErrorIdx/size)
	}
}
