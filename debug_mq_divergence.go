package main

import (
	"fmt"
	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

func main() {
	// Test 5x5 gradient pattern (first failing case)
	width, height := 5, 5
	data := make([]int32, width*height)
	for i := range data {
		data[i] = int32(i%256 - 128)
	}

	fmt.Println("=== Testing 5x5 Gradient Pattern ===")
	fmt.Printf("Input data: %v\n\n", data)

	// Enable MQ debug
	mqc.EnableEncoderDebug()
	mqc.EnableDecoderDebug()

	// Encode
	fmt.Println("--- ENCODING ---")
	encoder := t1.NewT1Encoder(width, height, 1, 8)
	encoded := encoder.Encode(data)
	fmt.Printf("\nEncoded %d bytes\n", len(encoded))

	// Decode
	fmt.Println("\n--- DECODING ---")
	decoder := t1.NewT1Decoder(encoded, width, height, 1, 8)
	decoded := decoder.Decode()

	// Compare
	fmt.Println("\n--- COMPARISON ---")
	errors := 0
	for i := range data {
		if data[i] != decoded[i] {
			if errors < 10 {
				fmt.Printf("Position %d: expected %d, got %d\n", i, data[i], decoded[i])
			}
			errors++
		}
	}
	fmt.Printf("\nTotal errors: %d/%d (%.1f%%)\n", errors, len(data), float64(errors)*100/float64(len(data)))
}
