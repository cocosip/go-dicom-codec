// Package main provides "无损编解码器"示例用法。
package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
)

func main() {
	// Example: Lossless compression with perfect reconstruction
	losslessExample()
}

func losslessExample() {
	fmt.Println("=== JPEG Lossless First-Order Prediction Example ===")

	// Create a test image (8-bit grayscale)
	width, height := 128, 128
	bitDepth := 8
	pixelData := make([]byte, width*height)

	// Create a complex pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Complex pattern with fine details
			val := (x*x + y*y) % 256
			pixelData[y*width+x] = byte(val)
		}
	}

	fmt.Printf("Original image: %dx%d, %d-bit\n", width, height, bitDepth)
	fmt.Printf("Original size: %d bytes\n", len(pixelData))

	// Encode
	jpegData, err := lossless14sv1.Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		log.Fatalf("Encode failed: %v", err)
	}

	fmt.Printf("Compressed size: %d bytes\n", len(jpegData))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, bits, err := lossless14sv1.Decode(jpegData)
	if err != nil {
		log.Fatalf("Decode failed: %v", err)
	}

	fmt.Printf("Decoded: %dx%d, %d components, %d-bit\n", w, h, components, bits)

	// Verify perfect reconstruction
	if len(decodedData) != len(pixelData) {
		log.Fatalf("Size mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Printf("✓ Perfect reconstruction: all %d pixels match exactly!\n", len(pixelData))
		fmt.Println("✓ Lossless compression verified!")
	} else {
		fmt.Printf("✗ Found %d pixel errors (unexpected for lossless)\n", errors)
	}

	fmt.Println("\n=== RGB Lossless Example ===")

	// RGB image
	rgbWidth, rgbHeight := 64, 64
	rgbData := make([]byte, rgbWidth*rgbHeight*3)

	for y := 0; y < rgbHeight; y++ {
		for x := 0; x < rgbWidth; x++ {
			offset := (y*rgbWidth + x) * 3
			rgbData[offset+0] = byte((x * 4) % 256)
			rgbData[offset+1] = byte((y * 4) % 256)
			rgbData[offset+2] = byte(((x + y) * 2) % 256)
		}
	}

	fmt.Printf("Original RGB image: %dx%d\n", rgbWidth, rgbHeight)
	fmt.Printf("Original size: %d bytes\n", len(rgbData))

	// Encode RGB
	rgbJpeg, err := lossless14sv1.Encode(rgbData, rgbWidth, rgbHeight, 3, 8)
	if err != nil {
		log.Fatalf("RGB encode failed: %v", err)
	}

	fmt.Printf("Compressed size: %d bytes\n", len(rgbJpeg))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(rgbData))/float64(len(rgbJpeg)))

	// Decode RGB
	decodedRGB, _, _, _, _, err := lossless14sv1.Decode(rgbJpeg)
	if err != nil {
		log.Fatalf("RGB decode failed: %v", err)
	}

	// Verify
	rgbErrors := 0
	for i := 0; i < len(rgbData); i++ {
		if rgbData[i] != decodedRGB[i] {
			rgbErrors++
		}
	}

	if rgbErrors == 0 {
		fmt.Printf("✓ Perfect RGB reconstruction: all %d bytes match!\n", len(rgbData))
	} else {
		fmt.Printf("✗ Found %d RGB errors\n", rgbErrors)
	}
}
