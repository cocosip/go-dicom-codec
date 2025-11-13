package t1

import (
	"testing"
)

// TestDiagonalPattern tests the diagonal pattern that's failing
func TestDiagonalPattern(t *testing.T) {
	width, height := 4, 4
	// Pattern: 1, 0, 0, 0,  0, 2, 0, 0,  0, 0, 4, 0,  0, 0, 0, 8
	data := []int32{1, 0, 0, 0, 0, 2, 0, 0, 0, 0, 4, 0, 0, 0, 0, 8}

	// Max value is 8, so maxBitplane = 3
	maxBitplane := 3
	numPasses := (maxBitplane + 1) * 3 // 12 passes

	t.Logf("Input: %v", data)
	t.Logf("maxBitplane: %d, numPasses: %d", maxBitplane, numPasses)

	// Encode
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	// Decode
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()
	t.Logf("Output: %v", decoded)

	// Check each value
	errors := 0
	for i := range data {
		if decoded[i] != data[i] {
			t.Logf("Mismatch at index %d: got %d, want %d", i, decoded[i], data[i])
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Total mismatches: %d / %d", errors, len(data))
	}
}

// TestSimpler tests just two diagonal values
func TestSimpler(t *testing.T) {
	width, height := 2, 2
	// Just two values: 1 at (0,0) and 2 at (1,1)
	data := []int32{1, 0, 0, 2}

	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Input: %v, maxBitplane: %d", data, maxBitplane)

	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()
	t.Logf("Output: %v", decoded)

	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decoded[i], data[i])
		}
	}
}
