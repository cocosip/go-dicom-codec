package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/codec"
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
)

func main() {
	fmt.Println("=== Codec Registry Example ===\n")

	// List all registered codecs
	listCodecs()

	fmt.Println("\n=== Using JPEG Baseline via Codec Interface ===")
	baselineExample()

	fmt.Println("\n=== Using JPEG Lossless SV1 via Codec Interface ===")
	losslessExample()
}

func listCodecs() {
	codecs := codec.List()
	fmt.Printf("Registered codecs: %d\n", len(codecs))
	for _, c := range codecs {
		fmt.Printf("  - %s (UID: %s)\n", c.Name(), c.UID())
	}
}

func baselineExample() {
	// Get codec by UID (DICOM Transfer Syntax UID)
	c, err := codec.Get("1.2.840.10008.1.2.4.50")
	if err != nil {
		log.Fatalf("Failed to get codec: %v", err)
	}

	fmt.Printf("Using codec: %s (UID: %s)\n", c.Name(), c.UID())

	// Create test image (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 3) % 256)
	}

	// Encode using codec interface
	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
		Options:    nil, // Use defaults
	}

	compressed, err := c.Encode(params)
	if err != nil {
		log.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Original size: %d bytes\n", len(pixelData))
	fmt.Printf("Compressed size: %d bytes\n", len(compressed))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(pixelData))/float64(len(compressed)))

	// Decode
	result, err := c.Decode(compressed)
	if err != nil {
		log.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, %d components, %d-bit\n",
		result.Width, result.Height, result.Components, result.BitDepth)
}

func losslessExample() {
	// Get codec by name
	c, err := codec.Get("jpeg-lossless-sv1")
	if err != nil {
		log.Fatalf("Failed to get codec: %v", err)
	}

	fmt.Printf("Using codec: %s (UID: %s)\n", c.Name(), c.UID())

	// Create test image (32x32 grayscale)
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Encode
	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
		Options:    nil,
	}

	compressed, err := c.Encode(params)
	if err != nil {
		log.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Original size: %d bytes\n", len(pixelData))
	fmt.Printf("Compressed size: %d bytes\n", len(compressed))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(pixelData))/float64(len(compressed)))

	// Decode
	result, err := c.Decode(compressed)
	if err != nil {
		log.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, %d components, %d-bit\n",
		result.Width, result.Height, result.Components, result.BitDepth)

	// Verify perfect reconstruction (lossless)
	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != result.PixelData[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Printf("✓ Perfect reconstruction: all %d pixels match!\n", len(pixelData))
	} else {
		fmt.Printf("✗ Found %d errors\n", errors)
	}
}
