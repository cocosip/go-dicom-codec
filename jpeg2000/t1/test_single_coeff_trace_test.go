package t1

import (
	"testing"
)

// TestSingleCoeffTrace traces the encoding/decoding of a single problematic coefficient
func TestSingleCoeffTrace(t *testing.T) {
	// From the 5x5 test, position [0,1] has error:
	// expected=-127 (01111111) got=-126 (01111110) diff=+1 xor=00000001
	// This means bit 0 is missing

	width, height := 5, 5
	input := make([]int32, 25)

	// Fill with zeros except the problematic position
	input[1] = -127 // Position [0,1] (row 0, column 1)

	t.Logf("Testing single coefficient at [0,1] = -127")
	t.Logf("Binary: 01111111 (abs=127)")
	t.Logf("Bitplanes: 7=0, 6=1, 5=1, 4=1, 3=1, 2=1, 1=1, 0=1")

	maxBitplane := 7
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
		t.Logf("SUCCESS: Position [0,1] decoded correctly as %d", decoded[1])
	}
}

// TestMultiplePositions tests the same value at different positions
func TestMultiplePositions(t *testing.T) {
	testValue := int32(-127)

	positions := []struct {
		row, col int
		name     string
	}{
		{0, 0, "[0,0]_first"},
		{0, 1, "[0,1]_error_prone"},
		{0, 2, "[0,2]"},
		{0, 3, "[0,3]_error_prone"},
		{0, 4, "[0,4]"},
		{3, 0, "[3,0]_perfect_row"},
		{3, 1, "[3,1]_perfect_row"},
	}

	for _, pos := range positions {
		t.Run(pos.name, func(t *testing.T) {
			width, height := 5, 5
			input := make([]int32, 25)

			idx := pos.row*width + pos.col
			input[idx] = testValue

			maxBitplane := 7
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

			if decoded[idx] != testValue {
				t.Errorf("%s: expected=%d, got=%d, diff=%d",
					pos.name, testValue, decoded[idx], decoded[idx]-testValue)
			} else {
				t.Logf("%s: PASS ✓", pos.name)
			}
		})
	}
}
