package main

import (
	"fmt"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

func main() {
	// Test 8x8 with 1 level DWT
	width, height := 8, 8
	
	// Create test data
	data := make([]int32, width*height)
	for i := 0; i < width*height; i++ {
		data[i] = int32(i % 8)
	}
	
	fmt.Println("Original:", data[:16])
	
	// Make a copy for forward transform
	encoded := make([]int32, len(data))
	copy(encoded, data)
	
	// Apply forward multilevel DWT (1 level)
	wavelet.ForwardMultilevel(encoded, width, height, 1)
	fmt.Println("After Forward DWT:", encoded[:16])
	
	// Apply inverse multilevel DWT (1 level)
	decoded := make([]int32, len(encoded))
	copy(decoded, encoded)
	wavelet.InverseMultilevel(decoded, width, height, 1)
	
	fmt.Println("After Inverse DWT:", decoded[:16])
	
	// Check if reconstruction is perfect
	errors := 0
	for i := 0; i < len(data); i++ {
		if data[i] != decoded[i] {
			errors++
			if errors <= 5 {
				fmt.Printf("  [%d] expected %d, got %d (diff=%d)\n", i, data[i], decoded[i], decoded[i]-data[i])
			}
		}
	}
	
	if errors == 0 {
		fmt.Println("✓ Perfect reconstruction!")
	} else {
		fmt.Printf("✗ %d errors out of %d\n", errors, len(data))
	}
}
