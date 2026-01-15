// Simple test to verify encoder/decoder works correctly
// Usage: go run test_simple.go

package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	fmt.Println("Simple Encoder/Decoder Test")
	fmt.Println("=" + repeatString("=", 70))

	// Create simple test data: 16x16 grayscale image
	width := 16
	height := 16
	testData := make([]byte, width*height)

	// Fill with simple pattern
	for i := 0; i < len(testData); i++ {
		testData[i] = byte(i % 256)
	}

	fmt.Printf("\nTest data: %dx%d grayscale, 8-bit\n", width, height)
	fmt.Printf("First 20 values: ")
	for i := 0; i < 20; i++ {
		fmt.Printf("%d ", testData[i])
	}
	fmt.Println()

	// Encode
	params := &jpeg2000.EncodeParams{
		Width:           width,
		Height:          height,
		Components:      1,
		BitDepth:        8,
		IsSigned:        false,
		NumLevels:       3,
		Lossless:        true,
		CodeBlockWidth:  64,
		CodeBlockHeight: 64,
		NumLayers:       1,
	}

	encoder := jpeg2000.NewEncoder(params)
	encoded, err := encoder.Encode(testData)
	if err != nil {
		fmt.Printf("Encode failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nEncoded: %d bytes (ratio: %.2f:1)\n", len(encoded), float64(len(testData))/float64(len(encoded)))

	// Decode
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		fmt.Printf("Decode failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDecoded: %dx%d, %d components, %d-bit\n",
		decoder.Width(), decoder.Height(), decoder.Components(), decoder.BitDepth())

	componentData := decoder.GetImageData()
	fmt.Printf("Component 0 first 20 values: ")
	for i := 0; i < 20 && i < len(componentData[0]); i++ {
		fmt.Printf("%d ", componentData[0][i])
	}
	fmt.Println()

	// Convert back to bytes (interleaved)
	decoded := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		decoded[i] = byte(componentData[0][i])
	}

	// Compare
	fmt.Println("\nComparison:")
	if bytes.Equal(testData, decoded) {
		fmt.Println("  ✓✓✓ PERFECT MATCH! Encoder/Decoder working correctly!")
	} else {
		matches := 0
		for i := 0; i < len(testData); i++ {
			if testData[i] == decoded[i] {
				matches++
			}
		}
		fmt.Printf("  ✗ Mismatch: %d/%d bytes match (%.2f%%)\n",
			matches, len(testData), float64(matches)/float64(len(testData))*100)

		fmt.Println("\n  First 10 differences:")
		shown := 0
		for i := 0; i < len(testData) && shown < 10; i++ {
			if testData[i] != decoded[i] {
				fmt.Printf("    Byte %d: original=%d, decoded=%d (component value=%d)\n",
					i, testData[i], decoded[i], componentData[0][i])
				shown++
			}
		}
	}

	// Test 16-bit
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("Testing 16-bit encoding...")
	fmt.Println(repeatString("=", 70))

	testData16 := make([]byte, width*height*2)
	for i := 0; i < width*height; i++ {
		val := uint16(i * 100)
		testData16[i*2] = byte(val & 0xFF)
		testData16[i*2+1] = byte((val >> 8) & 0xFF)
	}

	fmt.Printf("\nTest data: %dx%d grayscale, 16-bit\n", width, height)
	fmt.Printf("First 10 values (as uint16): ")
	for i := 0; i < 20 && i+1 < len(testData16); i += 2 {
		val := uint16(testData16[i]) | (uint16(testData16[i+1]) << 8)
		fmt.Printf("%d ", val)
	}
	fmt.Println()

	params16 := &jpeg2000.EncodeParams{
		Width:           width,
		Height:          height,
		Components:      1,
		BitDepth:        16,
		IsSigned:        false,
		NumLevels:       3,
		Lossless:        true,
		CodeBlockWidth:  64,
		CodeBlockHeight: 64,
		NumLayers:       1,
	}

	encoder16 := jpeg2000.NewEncoder(params16)
	encoded16, err := encoder16.Encode(testData16)
	if err != nil {
		fmt.Printf("Encode failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nEncoded: %d bytes (ratio: %.2f:1)\n", len(encoded16), float64(len(testData16))/float64(len(encoded16)))

	decoder16 := jpeg2000.NewDecoder()
	err = decoder16.Decode(encoded16)
	if err != nil {
		fmt.Printf("Decode failed: %v\n", err)
		os.Exit(1)
	}

	componentData16 := decoder16.GetImageData()
	fmt.Printf("\nComponent 0 first 10 values: ")
	for i := 0; i < 10 && i < len(componentData16[0]); i++ {
		fmt.Printf("%d ", componentData16[0][i])
	}
	fmt.Println()

	// Convert back
	decoded16 := make([]byte, width*height*2)
	for i := 0; i < width*height; i++ {
		val := uint16(componentData16[0][i])
		decoded16[i*2] = byte(val & 0xFF)
		decoded16[i*2+1] = byte((val >> 8) & 0xFF)
	}

	fmt.Println("\nComparison:")
	if bytes.Equal(testData16, decoded16) {
		fmt.Println("  ✓✓✓ PERFECT MATCH! 16-bit encoding working correctly!")
	} else {
		matches := 0
		for i := 0; i < len(testData16); i++ {
			if testData16[i] == decoded16[i] {
				matches++
			}
		}
		fmt.Printf("  ✗ Mismatch: %d/%d bytes match (%.2f%%)\n",
			matches, len(testData16), float64(matches)/float64(len(testData16))*100)

		fmt.Println("\n  First 5 value differences:")
		shown := 0
		for i := 0; i < width*height && shown < 5; i++ {
			origVal := uint16(testData16[i*2]) | (uint16(testData16[i*2+1]) << 8)
			decVal := uint16(decoded16[i*2]) | (uint16(decoded16[i*2+1]) << 8)
			if origVal != decVal {
				fmt.Printf("    Value %d: original=%d, decoded=%d (component=%d)\n",
					i, origVal, decVal, componentData16[0][i])
				shown++
			}
		}
	}
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
