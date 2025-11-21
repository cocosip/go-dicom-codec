package t1

import (
	"fmt"
	"testing"
)

// TestTrace5x4vs5x5 traces the difference between 5x4 (pass) and 5x5 (fail)
func TestTrace5x4vs5x5(t *testing.T) {
	// Use identical first 20 values for both tests
	baseValues := make([]int32, 20)
	for i := 0; i < 20; i++ {
		baseValues[i] = int32(i%256) - 128
	}

	t.Run("5x4_pass", func(t *testing.T) {
		width, height := 5, 4
		input := make([]int32, 20)
		copy(input, baseValues)

		t.Logf("=== 5x4 (PASS) ===")
		printTestMatrix(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

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
		printTestMatrix(t, decoded, width, height)

		errors := 0
		for i := 0; i < len(input); i++ {
			if decoded[i] != input[i] {
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("5x4: %d errors", errors)
		} else {
			t.Logf("5x4: ✓ Perfect")
		}
	})

	t.Run("5x5_fail", func(t *testing.T) {
		width, height := 5, 5
		input := make([]int32, 25)
		// First 20 values same as 5x4
		copy(input, baseValues)
		// Add 5 more for row 4
		for i := 20; i < 25; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("\n=== 5x5 (FAIL) ===")
		printTestMatrix(t, input, width, height)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

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
		printTestMatrix(t, decoded, width, height)

		t.Logf("\nComparing first 20 values (same as 5x4):")
		errors := 0
		errorsInFirst20 := 0
		for i := 0; i < 20; i++ {
			if decoded[i] != input[i] {
				errorsInFirst20++
				y, x := i/width, i%width
				t.Logf("  [%d,%d] idx=%d expected=%d got=%d diff=%d",
					y, x, i, input[i], decoded[i], decoded[i]-input[i])
			}
		}
		for i := 20; i < 25; i++ {
			if decoded[i] != input[i] {
				errors++
			}
		}

		t.Logf("\nErrors in first 20 values (rows 0-3): %d", errorsInFirst20)
		t.Logf("Errors in last 5 values (row 4): %d", errors)
		t.Logf("Total errors: %d/25", errorsInFirst20+errors)

		if errorsInFirst20 > 0 {
			t.Errorf("CRITICAL: Errors in rows 0-3 (same data as 5x4 which passes!)")
		}
	})
}

func printTestMatrix(t *testing.T, data []int32, width, height int) {
	for y := 0; y < height; y++ {
		line := ""
		for x := 0; x < width; x++ {
			line += fmt.Sprintf("%4d ", data[y*width+x])
		}
		t.Logf("  Row %d: %s", y, line)
	}
}
