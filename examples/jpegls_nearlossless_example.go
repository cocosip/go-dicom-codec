package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/codec"
	_ "github.com/cocosip/go-dicom-codec/jpegls/nearlossless"
)

func main() {
	fmt.Println("=== JPEG-LS Near-Lossless Example ===\n")

	// Get codec by UID
	c, err := codec.Get("1.2.840.10008.1.2.4.81")
	if err != nil {
		log.Fatalf("Failed to get codec: %v", err)
	}

	fmt.Printf("Using codec: %s (UID: %s)\n\n", c.Name(), c.UID())

	// Create test image (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Test different NEAR values
	nearValues := []int{0, 1, 3, 5}

	for _, near := range nearValues {
		fmt.Printf("--- NEAR=%d ---\n", near)

		// Encode with specified NEAR value
		params := codec.EncodeParams{
			PixelData:  pixelData,
			Width:      width,
			Height:     height,
			Components: 1,
			BitDepth:   8,
			Options:    &codec.BaseOptions{NearLossless: near},
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

		// Verify error bound
		maxError := 0
		errorCount := 0
		for i := 0; i < len(pixelData); i++ {
			diff := int(result.PixelData[i]) - int(pixelData[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > 0 {
				errorCount++
			}
			if diff > maxError {
				maxError = diff
			}
		}

		fmt.Printf("Max error: %d (limit: %d)\n", maxError, near)
		fmt.Printf("Pixels with errors: %d/%d (%.1f%%)\n",
			errorCount, len(pixelData), 100.0*float64(errorCount)/float64(len(pixelData)))

		if near == 0 {
			if maxError == 0 {
				fmt.Printf("✓ Perfect lossless reconstruction!\n")
			} else {
				fmt.Printf("✗ Lossless mode has errors!\n")
			}
		} else {
			if maxError <= near {
				fmt.Printf("✓ Error bound satisfied (max error %d ≤ NEAR %d)\n", maxError, near)
			} else {
				fmt.Printf("✗ Error bound violated (max error %d > NEAR %d)\n", maxError, near)
			}
		}

		fmt.Println()
	}

	fmt.Println("=== RGB Image Example ===\n")
	rgbExample(c)
}

func rgbExample(c codec.Codec) {
	// Create RGB test image (32x32)
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * components
			pixelData[idx+0] = byte((x * 8) % 256)   // R
			pixelData[idx+1] = byte((y * 8) % 256)   // G
			pixelData[idx+2] = byte(((x + y) * 4) % 256) // B
		}
	}

	near := 3
	fmt.Printf("RGB Image (32x32), NEAR=%d\n", near)

	// Encode
	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   8,
		Options:    &codec.BaseOptions{NearLossless: near},
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

	// Verify error bound
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(result.PixelData[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	fmt.Printf("Max error: %d (limit: %d)\n", maxError, near)
	if maxError <= near {
		fmt.Printf("✓ RGB encoding successful!\n")
	}
}
