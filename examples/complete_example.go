package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  go-dicom-codec Complete Demo")
	fmt.Println("========================================\n")

	// Demo 1: Direct package usage
	directPackageUsage()

	// Demo 2: Codec interface usage
	codecInterfaceUsage()

	// Demo 3: Registry features
	registryFeatures()

	// Demo 4: Quality comparison
	qualityComparison()
}

func directPackageUsage() {
	fmt.Println("=== Demo 1: Direct Package Usage ===\n")

	// Create test image
	width, height := 128, 128
	grayscale := createTestImage(width, height, 1)

	// Use baseline package directly
	fmt.Println("Using jpeg/baseline package directly:")
	encoded, err := baseline.Encode(grayscale, width, height, 1, 85)
	if err != nil {
		log.Fatal(err)
	}

	decoded, w, h, comp, err := baseline.Decode(encoded)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  Original: %d bytes\n", len(grayscale))
	fmt.Printf("  Encoded: %d bytes (%.2fx compression)\n",
		len(encoded), float64(len(grayscale))/float64(len(encoded)))
	fmt.Printf("  Decoded: %dx%d, %d components\n", w, h, comp)
	fmt.Printf("  Max error: %d\n\n", maxError(grayscale, decoded))
}

func codecInterfaceUsage() {
	fmt.Println("=== Demo 2: Codec Interface Usage ===\n")

	// Create test image
	width, height := 128, 128
	rgb := createTestImage(width, height, 3)

	// Get codec by UID (DICOM Transfer Syntax)
	c, err := codec.Get("1.2.840.10008.1.2.4.50")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Using codec: %s (UID: %s)\n", c.Name(), c.UID())

	// Encode with custom options
	opts := &baseline.Options{
		BaseOptions: codec.BaseOptions{Quality: 90},
	}

	params := codec.EncodeParams{
		PixelData:  rgb,
		Width:      width,
		Height:     height,
		Components: 3,
		BitDepth:   8,
		Options:    opts,
	}

	encoded, err := c.Encode(params)
	if err != nil {
		log.Fatal(err)
	}

	result, err := c.Decode(encoded)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  Encoded: %d bytes\n", len(encoded))
	fmt.Printf("  Decoded: %dx%d, %d components, %d-bit\n",
		result.Width, result.Height, result.Components, result.BitDepth)
	fmt.Printf("  Max error: %d\n\n", maxError(rgb, result.PixelData))
}

func registryFeatures() {
	fmt.Println("=== Demo 3: Registry Features ===\n")

	// List all codecs
	codecs := codec.List()
	fmt.Printf("Registered codecs: %d\n", len(codecs))
	for i, c := range codecs {
		fmt.Printf("  %d. %s\n", i+1, c.Name())
		fmt.Printf("     UID: %s\n", c.UID())
	}

	fmt.Println("\nAccessing codecs by different methods:")

	// By UID
	c1, _ := codec.Get("1.2.840.10008.1.2.4.50")
	fmt.Printf("  By UID '1.2.840.10008.1.2.4.50': %s\n", c1.Name())

	// By name
	c2, _ := codec.Get("jpeg-lossless-sv1")
	fmt.Printf("  By name 'jpeg-lossless-sv1': %s\n", c2.Name())
	fmt.Println()
}

func qualityComparison() {
	fmt.Println("=== Demo 4: Quality vs Compression Comparison ===\n")

	width, height := 256, 256
	img := createTestImage(width, height, 1)

	c, _ := codec.Get("jpeg-baseline")

	qualities := []int{50, 70, 85, 95}

	fmt.Println("Quality | Size (bytes) | Compression | Max Error")
	fmt.Println("--------|--------------|-------------|----------")

	for _, q := range qualities {
		opts := &baseline.Options{
			BaseOptions: codec.BaseOptions{Quality: q},
		}

		params := codec.EncodeParams{
			PixelData:  img,
			Width:      width,
			Height:     height,
			Components: 1,
			BitDepth:   8,
			Options:    opts,
		}

		encoded, _ := c.Encode(params)
		result, _ := c.Decode(encoded)

		ratio := float64(len(img)) / float64(len(encoded))
		err := maxError(img, result.PixelData)

		fmt.Printf("  %3d   | %12d | %9.2fx | %9d\n", q, len(encoded), ratio, err)
	}

	// Compare with lossless
	fmt.Println("\nLossless comparison:")
	lossless, _ := codec.Get("jpeg-lossless-sv1")

	params := codec.EncodeParams{
		PixelData:  img,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
	}

	encoded, _ := lossless.Encode(params)
	result, _ := lossless.Decode(encoded)

	ratio := float64(len(img)) / float64(len(encoded))
	err := maxError(img, result.PixelData)

	fmt.Printf("Lossless| %12d | %9.2fx | %9d (perfect)\n", len(encoded), ratio, err)
	fmt.Println()
}

// Helper functions

func createTestImage(width, height, components int) []byte {
	size := width * height * components
	data := make([]byte, size)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for c := 0; c < components; c++ {
				// Create interesting patterns
				val := (x*x + y*y + c*1000) % 256
				data[(y*width+x)*components+c] = byte(val)
			}
		}
	}

	return data
}

func maxError(original, decoded []byte) int {
	if len(original) != len(decoded) {
		return -1
	}

	maxErr := 0
	for i := 0; i < len(original); i++ {
		diff := int(original[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxErr {
			maxErr = diff
		}
	}

	return maxErr
}
