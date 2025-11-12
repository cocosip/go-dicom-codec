package t1

import (
	"fmt"
	"testing"
)

// TestDebug5x5 tests 5x5 with detailed logging
func TestDebug5x5(t *testing.T) {
	width, height := 5, 5
	numPixels := width * height
	input := make([]int32, numPixels)

	// Simple gradient with DC shift
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Log("Input data (DC-shifted):")
	for y := 0; y < height; y++ {
		row := ""
		for x := 0; x < width; x++ {
			row += fmt.Sprintf("%4d ", input[y*width+x])
		}
		t.Logf("  Row %d: %s", y, row)
	}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	// Encode
	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded: %d bytes", len(encoded))
	if len(encoded) <= 20 {
		t.Logf("Encoded data: %v", encoded)
	}

	// Decode
	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	t.Log("Decoded data:")
	for y := 0; y < height; y++ {
		row := ""
		for x := 0; x < width; x++ {
			idx := y*width + x
			if decoded[idx] != input[idx] {
				row += fmt.Sprintf("[%4d]", decoded[idx])
			} else {
				row += fmt.Sprintf(" %4d ", decoded[idx])
			}
		}
		t.Logf("  Row %d: %s", y, row)
	}

	// Check mismatches
	mismatches := 0
	for i := 0; i < numPixels; i++ {
		if decoded[i] != input[i] {
			mismatches++
		}
	}

	errorRate := float64(mismatches) / float64(numPixels) * 100
	if mismatches > 0 {
		t.Errorf("%.1f%% errors (%d/%d)", errorRate, mismatches, numPixels)
	} else {
		t.Log("✓ Perfect")
	}
}

// TestDebug4x4 tests 4x4 (which works) for comparison
func TestDebug4x4(t *testing.T) {
	width, height := 4, 4
	numPixels := width * height
	input := make([]int32, numPixels)

	// Same pattern as 5x5
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Log("Input data (DC-shifted):")
	for y := 0; y < height; y++ {
		row := ""
		for x := 0; x < width; x++ {
			row += fmt.Sprintf("%4d ", input[y*width+x])
		}
		t.Logf("  Row %d: %s", y, row)
	}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded: %d bytes", len(encoded))

	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	t.Log("Decoded data:")
	for y := 0; y < height; y++ {
		row := ""
		for x := 0; x < width; x++ {
			idx := y*width + x
			row += fmt.Sprintf(" %4d ", decoded[idx])
		}
		t.Logf("  Row %d: %s", y, row)
	}

	mismatches := 0
	for i := 0; i < numPixels; i++ {
		if decoded[i] != input[i] {
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Errorf("%d/%d errors", mismatches, numPixels)
	} else {
		t.Log("✓ Perfect")
	}
}
