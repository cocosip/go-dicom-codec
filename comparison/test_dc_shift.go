// Test DC level shift behavior
package main

import (
	"fmt"
	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	width, height := 8, 8

	// Create simple test data: 0, 1, 2, 3...
	componentData := make([][]int32, 1)
	componentData[0] = make([]int32, width*height)
	for i := range componentData[0] {
		componentData[0][i] = int32(i)
	}

	fmt.Println("Original data (first 10 values):")
	for i := 0; i < 10; i++ {
		fmt.Printf("%d ", componentData[0][i])
	}
	fmt.Println()

	// Encode
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 0  // No decomposition for simplicity
	encoder := jpeg2000.NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	fmt.Printf("\nEncoded: %d bytes\n", len(encoded))

	// Decode
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	decoded, _ := decoder.GetComponentData(0)

	fmt.Println("\nDecoded data (first 10 values):")
	for i := 0; i < 10; i++ {
		fmt.Printf("%d ", decoded[i])
	}
	fmt.Println()

	fmt.Println("\nDifferences (first 10):")
	for i := 0; i < 10; i++ {
		diff := decoded[i] - componentData[0][i]
		fmt.Printf("Pos %d: original=%d decoded=%d diff=%d\n",
			i, componentData[0][i], decoded[i], diff)
	}

	// Check decoder parameters
	fmt.Printf("\nDecoder parameters:\n")
	fmt.Printf("  Width: %d\n", decoder.Width())
	fmt.Printf("  Height: %d\n", decoder.Height())
	fmt.Printf("  BitDepth: %d\n", decoder.BitDepth())
	fmt.Printf("  Components: %d\n", decoder.Components())
}
