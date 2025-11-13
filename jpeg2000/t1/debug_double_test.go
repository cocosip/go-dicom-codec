package t1

import (
	"testing"
)

// TestDebugDoubleValue tests why value 1 becomes 3
func TestDebugDoubleValue(t *testing.T) {
	width, height := 2, 2
	// Simple case: one value = 1 at position (0,0)
	data := []int32{1, 0, 0, 0}

	// Encode
	enc := NewT1Encoder(width, height, 0)

	// Value 1 = binary 1, so maxBitplane = 0
	maxBitplane := 0
	numPasses := (maxBitplane + 1) * 3 // 3 passes for 1 bitplane

	t.Logf("Input: %v, maxBitplane: %d, numPasses: %d", data, maxBitplane, numPasses)

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

	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decoded[i], data[i])
		}
	}
}

// TestDebugValue2 tests value 2
func TestDebugValue2(t *testing.T) {
	width, height := 2, 2
	data := []int32{0, 2, 0, 0}

	enc := NewT1Encoder(width, height, 0)

	// Value 2 = binary 10, maxBitplane = 1
	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3 // 6 passes

	t.Logf("Input: %v, maxBitplane: %d, numPasses: %d", data, maxBitplane, numPasses)

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
