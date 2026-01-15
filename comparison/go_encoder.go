// Go JPEG2000 encoder for raw pixel data comparison
// Usage: go run go_encoder.go <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]
// Example: go run go_encoder.go input.bin output_go.j2k 512 512 3 8 0

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	if len(os.Args) < 7 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s input.bin output.j2k 512 512 3 8 0\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]
	width, _ := strconv.Atoi(os.Args[3])
	height, _ := strconv.Atoi(os.Args[4])
	numComponents, _ := strconv.Atoi(os.Args[5])
	bitDepth, _ := strconv.Atoi(os.Args[6])
	isSigned := false
	if len(os.Args) > 7 {
		isSigned = os.Args[7] != "0"
	}

	fmt.Println("Go JPEG2000 Encoder Configuration:")
	fmt.Printf("  Input: %s\n", inputFile)
	fmt.Printf("  Output: %s\n", outputFile)
	fmt.Printf("  Dimensions: %dx%d\n", width, height)
	fmt.Printf("  Components: %d\n", numComponents)
	fmt.Printf("  Bit depth: %d\n", bitDepth)
	fmt.Printf("  Signed: %v\n", isSigned)

	// Read raw pixel data
	rawData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input file: %v\n", err)
		os.Exit(1)
	}

	bytesPerSample := 1
	if bitDepth > 8 {
		bytesPerSample = 2
	}
	expectedSize := width * height * numComponents * bytesPerSample

	fmt.Printf("  File size: %d bytes\n", len(rawData))
	fmt.Printf("  Expected size: %d bytes\n", expectedSize)

	if len(rawData) < expectedSize {
		fmt.Fprintf(os.Stderr, "Warning: File size (%d) is smaller than expected (%d)\n", len(rawData), expectedSize)
	}

	// Setup encoder parameters
	params := &jpeg2000.EncodeParams{
		Width:           width,
		Height:          height,
		Components:      numComponents,
		BitDepth:        bitDepth,
		IsSigned:        isSigned,
		NumLevels:       5,              // 5 decomposition levels
		Lossless:        true,           // lossless compression
		CodeBlockWidth:  64,
		CodeBlockHeight: 64,
		NumLayers:       1,
		ProgressionOrder: 0,              // LRCP
		TileWidth:       0,               // single tile
		TileHeight:      0,               // single tile
	}

	// Create encoder
	encoder := jpeg2000.NewEncoder(params)

	// Encode
	fmt.Println("\nEncoding...")
	j2kData, err := encoder.Encode(rawData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode: %v\n", err)
		os.Exit(1)
	}

	// Write output
	err = os.WriteFile(outputFile, j2kData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Encoding completed successfully!\n")
	fmt.Printf("Output file: %s\n", outputFile)
	fmt.Printf("Output size: %d bytes\n", len(j2kData))
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(rawData))/float64(len(j2kData)))
}
