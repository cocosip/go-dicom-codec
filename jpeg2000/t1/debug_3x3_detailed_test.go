package t1

import (
	"testing"
)

// Test3x3UniformDetailed tests 3x3 uniform with detailed output
func Test3x3UniformDetailed(t *testing.T) {
	width, height := 3, 3
	data := []int32{
		-128, -128, -128,
		-128, -128, -128,
		-128, -128, -128,
	}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Input: 3x3 uniform, all values = -128")
	t.Logf("MaxBitplane: %d, NumPasses: %d", maxBitplane, numPasses)

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

	// Check all coefficients
	t.Log("\nResults:")
	errors := 0
	for i := 0; i < 9; i++ {
		status := "✓"
		if decoded[i] != data[i] {
			status = "✗"
			errors++
		}
		row := i / 3
		col := i % 3
		t.Logf("  [%d,%d] (idx=%d): expected=%d, got=%d %s (error=%d)",
			row, col, i, data[i], decoded[i], status, decoded[i]-data[i])
	}

	if errors > 0 {
		t.Errorf("Failed: %d/9 values incorrect", errors)
	} else {
		t.Log("✓ Perfect reconstruction")
	}
}

// Test3x3FirstColumn tests just first column (3 coefficients)
func Test3x3FirstColumn(t *testing.T) {
	width, height := 1, 3
	data := []int32{-128, -128, -128}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Input: 1x3 (first column), all values = -128")

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

	// Check
	errors := 0
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Index %d: expected=%d, got=%d (error=%d)",
				i, data[i], decoded[i], decoded[i]-data[i])
			errors++
		}
	}

	if errors == 0 {
		t.Log("✓ Perfect reconstruction")
	}
}

// Test3x3PartialGroup tests if the issue is with partial row groups
func Test3x3PartialGroup(t *testing.T) {
	// 3 rows means we have a partial group (not divisible by 4)
	// Column 0: rows 0,1,2 (3 coefficients - partial group)

	width, height := 1, 3
	data := []int32{-128, -128, -128}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Testing partial row group (3 rows, need 4 for full RL group)")

	// Encode
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	// Verify
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Partial group failed at index %d: expected=%d, got=%d",
				i, data[i], decoded[i])
		}
	}
}
