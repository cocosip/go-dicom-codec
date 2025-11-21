package t1

import (
	"fmt"
	"testing"
)

func TestDumpBitstream(t *testing.T) {
	t.Log("Testing 5x5 gradient - dumping bitstream")

	// Create 5x5 gradient
	width, height := 5, 5
	coeffs := make([]int32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			coeffs[y*width+x] = int32((y*width + x) - 12) // -12 to 12
		}
	}

	t.Log("Coefficients:")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fmt.Printf("%4d ", coeffs[y*width+x])
		}
		fmt.Println()
	}

	// Encode
	numPasses := 24 // 8 bitplanes * 3 passes
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(coeffs, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("\nBitstream length: %d bytes", len(encoded))
	t.Log("Bitstream (hex):")
	for i, b := range encoded {
		if i%16 == 0 {
			fmt.Printf("\n%04x: ", i)
		}
		fmt.Printf("%02x ", b)
	}
	fmt.Println()

	t.Log("\nBitstream (binary):")
	for i, b := range encoded {
		if i%8 == 0 {
			fmt.Printf("\n%04x: ", i)
		}
		fmt.Printf("%08b ", b)
	}
	fmt.Println()

	// Decode
	maxBitplane := 7
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded data
	decoded := dec.data

	t.Log("\nDecoded coefficients:")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fmt.Printf("%4d ", decoded[y*width+x])
		}
		fmt.Println()
	}

	// Compare
	t.Log("\nErrors:")
	hasError := false
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if coeffs[idx] != decoded[idx] {
				t.Logf("  pos(%d,%d): expected=%d got=%d diff=%d",
					x, y, coeffs[idx], decoded[idx], coeffs[idx]-decoded[idx])
				hasError = true
			}
		}
	}

	if !hasError {
		t.Log("  No errors!")
	}
}
