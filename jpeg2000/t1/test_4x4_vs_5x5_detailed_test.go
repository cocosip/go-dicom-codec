package t1

import (
	"fmt"
	"testing"
)

// Test4x4vs5x5Detailed compares 4x4 (passing) vs 5x5 (failing) in detail
func Test4x4vs5x5Detailed(t *testing.T) {
	t.Run("4x4_gradient_full", func(t *testing.T) {
		width, height := 4, 4
		numPixels := width * height
		input := make([]int32, numPixels)

		// Same gradient pattern
		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("4x4 Input:")
		printMatrixSimple(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("4x4 Encoded: %d bytes, maxBitplane=%d", len(encoded), maxBitplane)

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		t.Logf("\n4x4 Decoded:")
		printMatrixSimple(t, decoded, width, height)

		errors := 0
		for i := 0; i < numPixels; i++ {
			if decoded[i] != input[i] {
				errors++
				t.Logf("  [%d] expected=%d got=%d diff=%d",
					i, input[i], decoded[i], decoded[i]-input[i])
			}
		}

		if errors > 0 {
			t.Errorf("4x4: %d errors", errors)
		} else {
			t.Logf("4x4: ✓ Perfect")
		}
	})

	t.Run("5x5_gradient_full", func(t *testing.T) {
		width, height := 5, 5
		numPixels := width * height
		input := make([]int32, numPixels)

		// Same gradient pattern
		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("\n5x5 Input:")
		printMatrixSimple(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("5x5 Encoded: %d bytes, maxBitplane=%d", len(encoded), maxBitplane)

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		t.Logf("\n5x5 Decoded:")
		printMatrixSimple(t, decoded, width, height)

		t.Logf("\n5x5 Errors:")
		errors := 0
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				i := y*width + x
				if decoded[i] != input[i] {
					errors++
					t.Logf("  [%d,%d] idx=%d expected=%d got=%d diff=%d",
						y, x, i, input[i], decoded[i], decoded[i]-input[i])
				}
			}
		}

		if errors > 0 {
			t.Logf("5x5: %d/%d errors (%.1f%%)", errors, numPixels, float64(errors)*100/float64(numPixels))
		} else {
			t.Logf("5x5: ✓ Perfect")
		}
	})
}

func printMatrixSimple(t *testing.T, data []int32, width, height int) {
	for y := 0; y < height; y++ {
		line := ""
		for x := 0; x < width; x++ {
			line += fmt.Sprintf("%4d ", data[y*width+x])
		}
		t.Logf("  Row %d: %s", y, line)
	}
}
