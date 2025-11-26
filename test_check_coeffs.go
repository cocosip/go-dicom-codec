package main

import (
	"fmt"
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

func main() {
	width, height := 8, 8
	
	// Original data
	original := make([]int32, width*height)
	for i := 0; i < width*height; i++ {
		original[i] = int32(i % 8)
	}
	
	// Apply DWT
	dwt := make([]int32, len(original))
	copy(dwt, original)
	wavelet.ForwardMultilevel(dwt, width, height, 1)
	
	fmt.Println("After DWT (first 16):", dwt[:16])
	
	// Now encode the ORIGINAL data with jpeg2000
	componentData := make([][]int32, 1)
	componentData[0] = make([]int32, len(original))
	copy(componentData[0], original)
	
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 1
	encoder := jpeg2000.NewEncoder(params)
	
	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}
	
	fmt.Printf("Encoded %d bytes\n", len(encoded))
	
	// Decode
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}
	
	decoded, _ := decoder.GetComponentData(0)
	fmt.Println("Decoded (first 16):", decoded[:16])
	
	// Compare
	errors := 0
	for i := 0; i < len(original) && i < 16; i++ {
		if original[i] != decoded[i] {
			errors++
			fmt.Printf("  [%d] orig=%d decoded=%d diff=%d\n", 
				i, original[i], decoded[i], decoded[i]-original[i])
		}
	}
	
	if errors == 0 {
		fmt.Println("First 16 pixels: PERFECT!")
	}
}
