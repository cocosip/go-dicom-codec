package t1

import (
	"testing"
)

// TestUniformSingle tests a single uniform value
func TestUniformSingle(t *testing.T) {
	width, height := 1, 1
	data := []int32{-128}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Input: %v, maxBitplane: %d", data, maxBitplane)

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

	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d", i, decoded[i], data[i])
		}
	}
}
