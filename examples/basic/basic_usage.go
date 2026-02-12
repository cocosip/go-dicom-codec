// Package main demonstrates basic usage of the codecs.
package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
)

func main() {
	// Example 1: Encode and decode a grayscale image
	grayscaleExample()

	// Example 2: Encode and decode an RGB image
	rgbExample()
}

func grayscaleExample() {
	fmt.Println("=== Grayscale Image Example ===")

	// Create a simple grayscale gradient image (64x64)
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a diagonal gradient
			pixelData[y*width+x] = byte((x + y) * 2)
		}
	}

	// Encode to JPEG
	jpegData, err := baseline.Encode(pixelData, width, height, 1, 85)
	if err != nil {
		log.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Original size: %d bytes\n", len(pixelData))
	fmt.Printf("Compressed size: %d bytes\n", len(jpegData))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(pixelData))/float64(len(jpegData)))

	// Decode from JPEG
	decodedData, w, h, components, err := baseline.Decode(jpegData)
	if err != nil {
		log.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, %d components\n", w, h, components)

	// Calculate maximum error
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}
	fmt.Printf("Maximum pixel error: %d\n\n", maxError)
}

func rgbExample() {
	fmt.Println("=== RGB Image Example ===")

	// Create a simple RGB color gradient image (64x64)
	width, height := 64, 64
	pixelData := make([]byte, width*height*3)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 4)       // R
			pixelData[offset+1] = byte(y * 4)       // G
			pixelData[offset+2] = byte((x + y) * 2) // B
		}
	}

	// Encode to JPEG with different quality levels
	qualities := []int{50, 75, 90}

	for _, quality := range qualities {
		jpegData, err := baseline.Encode(pixelData, width, height, 3, quality)
		if err != nil {
			log.Fatalf("Encode failed: %v", err)
		}

		fmt.Printf("Quality %d: %d bytes (ratio: %.2fx)\n",
			quality, len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

		// Decode
		decodedData, w, h, components, err := baseline.Decode(jpegData)
		if err != nil {
			log.Fatalf("Decode failed: %v", err)
		}

		// Calculate maximum error
		maxError := 0
		for i := 0; i < len(pixelData); i++ {
			diff := int(pixelData[i]) - int(decodedData[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
		}
		fmt.Printf("  Decoded: %dx%d, %d components, max error: %d\n", w, h, components, maxError)
	}
}
