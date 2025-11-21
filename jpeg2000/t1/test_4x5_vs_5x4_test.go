package t1

import (
	"fmt"
	"testing"
)

// Test4x5vs5x4 tests if the problem is width-specific or partial-row-specific
func Test4x5vs5x4(t *testing.T) {
	t.Run("4x5_gradient", func(t *testing.T) {
		width, height := 4, 5
		numPixels := width * height
		input := make([]int32, numPixels)

		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("4x5 (width=4, height=5) - has partial row group")
		t.Logf("Input:")
		printMatrixSimple2(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		t.Logf("Decoded:")
		printMatrixSimple2(t, decoded, width, height)

		errors := 0
		for i := 0; i < numPixels; i++ {
			if decoded[i] != input[i] {
				errors++
				y, x := i/width, i%width
				t.Logf("  [%d,%d] idx=%d expected=%d got=%d diff=%d",
					y, x, i, input[i], decoded[i], decoded[i]-input[i])
			}
		}

		if errors > 0 {
			t.Errorf("4x5: %d/%d errors (%.1f%%)", errors, numPixels, float64(errors)*100/float64(numPixels))
		} else {
			t.Logf("4x5: ✓ Perfect")
		}
	})

	t.Run("5x4_gradient", func(t *testing.T) {
		width, height := 5, 4
		numPixels := width * height
		input := make([]int32, numPixels)

		for i := 0; i < numPixels; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("\n5x4 (width=5, height=4) - no partial row group")
		t.Logf("Input:")
		printMatrixSimple2(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		t.Logf("Decoded:")
		printMatrixSimple2(t, decoded, width, height)

		errors := 0
		for i := 0; i < numPixels; i++ {
			if decoded[i] != input[i] {
				errors++
				y, x := i/width, i%width
				t.Logf("  [%d,%d] idx=%d expected=%d got=%d diff=%d",
					y, x, i, input[i], decoded[i], decoded[i]-input[i])
			}
		}

		if errors > 0 {
			t.Errorf("5x4: %d/%d errors (%.1f%%)", errors, numPixels, float64(errors)*100/float64(numPixels))
		} else {
			t.Logf("5x4: ✓ Perfect")
		}
	})
}

func printMatrixSimple2(t *testing.T, data []int32, width, height int) {
	for y := 0; y < height; y++ {
		line := ""
		for x := 0; x < width; x++ {
			line += fmt.Sprintf("%4d ", data[y*width+x])
		}
		t.Logf("  Row %d: %s", y, line)
	}
}
