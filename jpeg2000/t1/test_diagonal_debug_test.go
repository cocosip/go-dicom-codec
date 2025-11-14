package t1

import (
	"fmt"
	"testing"
)

// TestDiagonalDebug traces [1,0,0,2] with full position tracking
func TestDiagonalDebug(t *testing.T) {
	width, height := 2, 2
	data := []int32{1, 0, 0, 2}

	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Testing [1,0,0,2] - positions: (0,0)=1, (1,0)=0, (0,1)=0, (1,1)=2")

	// Encode
	enc := NewT1Encoder(width, height, 0)

	// Manually check what should be encoded
	t.Logf("\nExpected encoding:")
	t.Logf("  BP1: pos0 bit=0, pos1 bit=0, pos2 bit=0, pos3 bit=1 (pos3 becomes significant)")
	t.Logf("  BP0: pos0 bit=1 (becomes significant), pos1 bit=0, pos2 bit=0, pos3 bit=0 (refinement)")

	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("\nEncoded %d bytes: %v", len(encoded), encoded)

	// Decode with full tracking
	dec := NewT1Decoder(width, height, 0)

	// Add custom logging to track all positions
	t.Logf("\nDecoding:")
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	t.Logf("\nResults:")
	for i := 0; i < 4; i++ {
		x := i % width
		y := i / width
		status := "✓"
		if decoded[i] != data[i] {
			status = fmt.Sprintf("✗ (got %d)", decoded[i])
		}
		t.Logf("  Position %d (%d,%d): expected=%d %s", i, x, y, data[i], status)
	}

	// Fail if any incorrect
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Position %d: expected=%d, got=%d", i, data[i], decoded[i])
		}
	}
}
