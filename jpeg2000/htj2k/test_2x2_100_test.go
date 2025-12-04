package htj2k

import (
	"testing"
)

// Test2x2With100 tests 2x2 block with value 100 (same as TestHTBlockSingleNonZero)
func Test2x2With100(t *testing.T) {
	width := 2
	height := 2

	// Single non-zero at [0]
	testCoeffs := []int32{100, 0, 0, 0}

	t.Logf("Input: %v", testCoeffs)

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes: %v", len(encoded), encoded)

	// Decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %v", decoded)

	// Compare
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("Mismatch at [%d]: expected %d, got %d", i, testCoeffs[i], decoded[i])
		}
	}
}

// Test2x2AllSame tests 2x2 block where all samples have same value
func Test2x2AllSame(t *testing.T) {
	width := 2
	height := 2

	testCoeffs := []int32{100, 100, 100, 100}

	t.Logf("Input: %v", testCoeffs)

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes: %v", len(encoded), encoded)

	// Decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %v", decoded)

	// Compare
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("Mismatch at [%d]: expected %d, got %d", i, testCoeffs[i], decoded[i])
		}
	}
}
