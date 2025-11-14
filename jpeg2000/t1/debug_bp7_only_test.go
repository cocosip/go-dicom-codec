package t1

import (
	"testing"
)

// TestBP7Only tests encoding/decoding only BP7 (3 passes)
func TestBP7Only(t *testing.T) {
	width, height := 2, 3
	data := []int32{-128, -128, -128, -128, -128, -128}

	maxBitplane := 7
	numPasses := 3 // Only BP7: SPP, MRP, CP

	t.Logf("Input: %v, maxBitplane: %d, numPasses: %d", data, maxBitplane, numPasses)

	// Encode
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes: %v", len(encoded), encoded)

	// Decode
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()
	t.Logf("Output: %v", decoded)

	errors := 0
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: got %d, want %d (error: %d)", i, decoded[i], data[i], decoded[i]-data[i])
			errors++
		}
	}

	if errors == 0 {
		t.Log("✓ Perfect reconstruction")
	} else {
		t.Logf("✗ %d/%d values incorrect", errors, len(data))
	}
}
