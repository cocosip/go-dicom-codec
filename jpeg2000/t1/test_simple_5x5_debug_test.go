package t1

import (
	"fmt"
	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
	"testing"
)

// Test 5x5 with detailed tracing
func TestSimple5x5Debug(t *testing.T) {
	width, height := 5, 5
	input := make([]int32, width*height)
	for i := 0; i < len(input); i++ {
		input[i] = int32(i%256) - 128
	}

	fmt.Printf("Input data:\n")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fmt.Printf("%4d ", input[y*width+x])
		}
		fmt.Println()
	}

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	fmt.Printf("\nMax bitplane: %d\n", maxBitplane)
	fmt.Printf("Num passes: %d\n\n", numPasses)

	// Enable MQ debug
	mqc.EnableEncoderDebug()
	mqc.EnableDecoderDebug()

	// Encode
	fmt.Println("=== ENCODING ===")
	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	fmt.Printf("\nEncoded %d bytes\n\n", len(encoded))

	// Decode
	fmt.Println("=== DECODING ===")
	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Compare
	fmt.Println("\n=== COMPARISON ===")
	fmt.Printf("Position | Expected | Got | Error\n")
	fmt.Printf("---------|----------|-----|------\n")
	errorCount := 0
	for i := range input {
		if input[i] != decoded[i] {
			y := i / width
			x := i % width
			fmt.Printf("(%d,%d)    | %4d     | %4d| YES\n", x, y, input[i], decoded[i])
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("5x5: %d/%d errors (%.1f%%)", errorCount, len(input), float64(errorCount)*100/float64(len(input)))
	}
}
