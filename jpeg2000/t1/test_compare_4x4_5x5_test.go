package t1

import (
	"fmt"
	"testing"
)

// TestCompare4x4vs5x5 compares 4x4 (passing) vs 5x5 (failing) with identical value ranges
func TestCompare4x4vs5x5(t *testing.T) {
	// Use first 16 values for both tests
	baseValues := []int32{
		-128, -127, -126, -125,
		-124, -123, -122, -121,
		-120, -119, -118, -117,
		-116, -115, -114, -113,
	}

	t.Run("4x4_baseline", func(t *testing.T) {
		width, height := 4, 4
		input := make([]int32, 16)
		copy(input, baseValues)

		maxBitplane := 7
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("4x4 encoded: %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		errorCount := 0
		for i := range input {
			if decoded[i] != input[i] {
				errorCount++
				t.Errorf("[%d] expected=%d, got=%d", i, input[i], decoded[i])
			}
		}

		if errorCount == 0 {
			t.Logf("4x4: PASS ✅")
		}
	})

	t.Run("5x5_failing", func(t *testing.T) {
		width, height := 5, 5
		input := make([]int32, 25)
		// First 16 values same as 4x4
		copy(input, baseValues)
		// Add 9 more values
		for i := 16; i < 25; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("5x5 input:")
		for y := 0; y < height; y++ {
			line := ""
			for x := 0; x < width; x++ {
				line += fmt.Sprintf("%4d ", input[y*width+x])
			}
			t.Logf("  Row %d: %s", y, line)
		}

		maxBitplane := 7
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("5x5 encoded: %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		t.Logf("\n5x5 decoded:")
		for y := 0; y < height; y++ {
			line := ""
			for x := 0; x < width; x++ {
				i := y*width + x
				marker := "✓"
				if decoded[i] != input[i] {
					marker = "✗"
				}
				line += fmt.Sprintf("%4d%s ", decoded[i], marker)
			}
			t.Logf("  Row %d: %s", y, line)
		}

		errorCount := 0
		for i := range input {
			if decoded[i] != input[i] {
				errorCount++
			}
		}

		if errorCount > 0 {
			t.Errorf("5x5: %d/%d errors", errorCount, len(input))
		}
	})
}
