package nearlossless

import (
	"fmt"
	"testing"
)

// TestSimpleFlat tests encoding/decoding of a completely flat image
func TestSimpleFlat(t *testing.T) {
	// Create a 4x4 image with all same values
	width, height := 4, 4
	pixelData := make([]byte, width*height)

	// Fill with constant value
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = 100
	}

	near := 0 // Start with lossless

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, 8, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Flat 4x4 image, NEAR=%d, encoded size=%d bytes\n", near, len(encoded))

	// Decode
	decoded, w, h, _, _, actualNear, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, NEAR=%d\n", w, h, actualNear)

	// Verify all pixels are 100
	for i := 0; i < len(pixelData); i++ {
		if decoded[i] != pixelData[i] {
			t.Errorf("Pixel %d: got %d, want %d", i, decoded[i], pixelData[i])
		}
	}
}

// TestSimpleFlatNear3 tests with NEAR=3
func TestSimpleFlatNear3(t *testing.T) {
	// Create a 4x4 image with values within NEAR range
	width, height := 4, 4
	pixelData := make([]byte, width*height)

	// Fill with values that vary within NEAR=3
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(100 + (i % 4)) // 100, 101, 102, 103, 100, 101...
	}

	near := 3

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, 8, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Near-flat 4x4 image, NEAR=%d, encoded size=%d bytes\n", near, len(encoded))

	// Decode
	decoded, w, h, _, _, actualNear, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, NEAR=%d\n", w, h, actualNear)

	// Verify errors are within NEAR
	for i := 0; i < len(pixelData); i++ {
		diff := int(decoded[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > near {
			t.Errorf("Pixel %d: error=%d exceeds NEAR=%d (orig=%d, decoded=%d)",
				i, diff, near, pixelData[i], decoded[i])
		}
	}
}

// TestSingleLine tests a single line to isolate issues
func TestSingleLine(t *testing.T) {
	width, height := 8, 1
	pixelData := make([]byte, width*height)

	// First 4 pixels same, then different
	pixelData[0] = 100
	pixelData[1] = 100
	pixelData[2] = 100
	pixelData[3] = 100
	pixelData[4] = 110
	pixelData[5] = 120
	pixelData[6] = 130
	pixelData[7] = 140

	near := 0

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, 8, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Single line 8x1 image, NEAR=%d, encoded size=%d bytes\n", near, len(encoded))
	fmt.Printf("Input:  %v\n", pixelData)

	// Decode
	decoded, w, h, _, _, actualNear, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, NEAR=%d\n", w, h, actualNear)
	fmt.Printf("Output: %v\n", decoded)

	// Verify
	for i := 0; i < len(pixelData); i++ {
		if decoded[i] != pixelData[i] {
			t.Errorf("Pixel %d: got %d, want %d", i, decoded[i], pixelData[i])
		}
	}
}
