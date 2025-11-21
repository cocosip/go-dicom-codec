package t1

import (
	"testing"
)

// TestRLPathTrace traces whether RL or Normal path is used for isolated coefficient
func TestRLPathTrace(t *testing.T) {
	t.Run("isolated_single", func(t *testing.T) {
		width, height := 4, 4
		input := make([]int32, 16)
		input[0] = -127 // Position [0,0] - first coefficient

		t.Logf("Testing isolated coefficient at [0,0] = -127")

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

		if decoded[0] != input[0] {
			t.Errorf("expected=%d, got=%d, diff=%d",
				input[0], decoded[0], decoded[0]-input[0])
		} else {
			t.Logf("SUCCESS: decoded correctly as %d", decoded[0])
		}
	})

	t.Run("dense_pattern", func(t *testing.T) {
		width, height := 4, 4
		input := []int32{
			-128, -127, -126, -125,
			-124, -123, -122, -121,
			-120, -119, -118, -117,
			-116, -115, -114, -113,
		}

		t.Logf("Testing dense pattern")

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

		errors := 0
		for i := range input {
			if decoded[i] != input[i] {
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("Dense pattern: %d errors", errors)
		} else {
			t.Logf("SUCCESS: dense pattern decoded perfectly")
		}
	})
}
