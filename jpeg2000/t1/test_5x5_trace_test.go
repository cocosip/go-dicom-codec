package t1

import (
	"fmt"
	"testing"
)

// Test5x5Trace traces encoding and decoding of 5x5 gradient with detailed position logging
func Test5x5Trace(t *testing.T) {
	width, height := 5, 5
	numPixels := width * height
	input := make([]int32, numPixels)

	// Generate pattern (same as TestRLBoundaryConditions)
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Logf("Input pattern:")
	for y := 0; y < height; y++ {
		line := ""
		for x := 0; x < width; x++ {
			line += fmt.Sprintf("%4d ", input[y*width+x])
		}
		t.Logf("  Row %d: %s", y, line)
	}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	// Encode
	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	// Decode
	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Compare
	t.Logf("\nComparison:")
	errorCount := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			status := "✓"
			if decoded[i] != input[i] {
				status = "✗"
				errorCount++
			}
			t.Logf("  [%d,%d] input=%4d decoded=%4d %s", x, y, input[i], decoded[i], status)
		}
	}

	errorRate := float64(errorCount) / float64(numPixels) * 100
	t.Logf("\nError rate: %d/%d (%.1f%%)", errorCount, numPixels, errorRate)

	if errorCount > 0 {
		t.Errorf("Found %d errors", errorCount)
	}
}
