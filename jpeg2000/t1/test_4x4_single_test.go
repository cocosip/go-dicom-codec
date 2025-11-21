package t1

import (
	"testing"
)

// Test4x4SingleCoeff tests a single coefficient in 4x4 (which passes)
func Test4x4SingleCoeff(t *testing.T) {
	width, height := 4, 4
	input := make([]int32, 16)

	// Test same value that fails in 5x5
	input[1] = -127 // Position [0,1]

	t.Logf("Testing 4x4 with single coefficient at [0,1] = -127")

	// Calculate actual maxBitplane from data (simulates T2 layer)
	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3
	t.Logf("Calculated maxBitplane = %d", maxBitplane)

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

	if decoded[1] != input[1] {
		t.Errorf("Position [0,1]: expected=%d, got=%d, diff=%d",
			input[1], decoded[1], decoded[1]-input[1])

		// Analyze which bitplane is wrong
		expAbs := int32(127)
		gotAbs := -decoded[1]

		t.Logf("\nBitplane analysis:")
		for bp := 7; bp >= 0; bp-- {
			expBit := (expAbs >> uint(bp)) & 1
			gotBit := (gotAbs >> uint(bp)) & 1
			status := "✓"
			if expBit != gotBit {
				status = "✗ ERROR"
			}
			t.Logf("  BP%d: expected=%d got=%d %s", bp, expBit, gotBit, status)
		}
	} else {
		t.Logf("SUCCESS: 4x4 [0,1] decoded correctly as %d", decoded[1])
	}
}
