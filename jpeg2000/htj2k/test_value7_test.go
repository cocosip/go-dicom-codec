package htj2k

import (
	"testing"
)

func TestValue7Single(t *testing.T) {
	width := 2
	height := 2
	testCoeffs := []int32{7, 0, 0, 0}

	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("[%d] expected %d, got %d", i, testCoeffs[i], decoded[i])
		}
	}
}

func TestValue7All(t *testing.T) {
	width := 2
	height := 2
	testCoeffs := []int32{7, 7, 7, 7}

	t.Logf("Input: %v", testCoeffs)

	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %v", encoded)

	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %v", decoded)

	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("[%d] expected %d, got %d", i, testCoeffs[i], decoded[i])
		}
	}
}
