package t1

import (
	"fmt"
	"testing"
)

// TestDebugBoundary examines boundary flag behavior
func TestDebugBoundary(t *testing.T) {
	// Simple test: 2 rows, check if bottom row's flags affect top row

	t.Run("2x1_vertical", func(t *testing.T) {
		// 2 rows, 1 column - vertical strip
		width, height := 1, 2
		input := []int32{-128, -127}

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

		for i := range input {
			if decoded[i] != input[i] {
				t.Errorf("[%d] expected=%d got=%d", i, input[i], decoded[i])
			}
		}

		if decoded[0] == input[0] && decoded[1] == input[1] {
			t.Logf("✓ 2x1: Perfect")
		}
	})

	t.Run("1x2_horizontal", func(t *testing.T) {
		// 1 row, 2 columns - horizontal strip
		width, height := 2, 1
		input := []int32{-128, -127}

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

		for i := range input {
			if decoded[i] != input[i] {
				t.Errorf("[%d] expected=%d got=%d", i, input[i], decoded[i])
			}
		}

		if decoded[0] == input[0] && decoded[1] == input[1] {
			t.Logf("✓ 1x2: Perfect")
		}
	})

	t.Run("examine_contexts", func(t *testing.T) {
		// Let's examine what contexts are calculated for edge vs non-edge coefficients
		width, height := 5, 5

		// Create encoder to examine flag array structure
		enc := NewT1Encoder(width, height, 0)

		paddedWidth := width + 2
		paddedHeight := height + 2

		t.Logf("width=%d height=%d paddedWidth=%d paddedHeight=%d", width, height, paddedWidth, paddedHeight)
		t.Logf("flags array size: %d", len(enc.flags))

		// Simulate a coefficient at (0,3) becoming significant
		x, y := 0, 3
		idx := (y+1)*paddedWidth + (x + 1)

		t.Logf("\nSimulating coefficient at (x=%d, y=%d) idx=%d becoming significant", x, y, idx)

		// Mark it significant
		enc.flags[idx] = T1_SIG | T1_SIGN

		// Manually check what updateNeighborFlags would do
		t.Logf("\nBoundary checks:")
		t.Logf("  North: y > 0? %v (y=%d)", y > 0, y)
		t.Logf("  South: y < height-1? %v (y=%d, height-1=%d)", y < height-1, y, height-1)
		t.Logf("  West: x > 0? %v (x=%d)", x > 0, x)
		t.Logf("  East: x < width-1? %v (x=%d, width-1=%d)", x < width-1, x, width-1)

		// Now call updateNeighborFlags
		enc.updateNeighborFlags(x, y, idx)

		// Check which neighbors got updated
		t.Logf("\nNeighbor flags after update:")

		// North neighbor
		if y > 0 {
			nIdx := (y)*paddedWidth + (x + 1)
			t.Logf("  North[%d]: SIG_S=%v", nIdx, (enc.flags[nIdx]&T1_SIG_S) != 0)
		} else {
			t.Logf("  North: skipped (boundary)")
		}

		// South neighbor
		if y < height-1 {
			sIdx := (y+2)*paddedWidth + (x + 1)
			t.Logf("  South[%d]: SIG_N=%v", sIdx, (enc.flags[sIdx]&T1_SIG_N) != 0)
		} else {
			noPadIdx := (y+2)*paddedWidth + (x + 1)
			t.Logf("  South: skipped (boundary), but padding cell [%d] has SIG_N=%v",
				noPadIdx, (enc.flags[noPadIdx]&T1_SIG_N) != 0)
		}

		// Check if this demonstrates the problem
		fmt.Printf("\n")
	})
}
