package main

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
	"github.com/cocosip/go-dicom-codec/jpeg/lossless"
	"github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	fmt.Println("=== DICOM JPEG Codecs - Complete Example ===\n")

	// Register all codecs
	fmt.Println("Registering all JPEG codecs...")
	baseline.RegisterBaselineCodec(85)          // Quality 85
	lossless14sv1.RegisterLosslessSV1Codec()    // Predictor 1 only
	lossless.RegisterLosslessCodec(4)           // Predictor 4
	fmt.Println("✓ All codecs registered\n")

	// Create test image data (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	fmt.Printf("Test image: %dx%d grayscale, %d bytes\n\n", width, height, len(pixelData))

	// Prepare source PixelData
	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     uint16(width),
		Height:                    uint16(height),
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
		TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
	}

	// Get global registry
	registry := codec.GetGlobalRegistry()

	// Test JPEG Baseline (Lossy)
	fmt.Println("--- JPEG Baseline (Process 1) - Lossy ---")
	testCodec(registry, transfer.JPEGBaseline8Bit, src, true)

	// Test JPEG Lossless SV1
	fmt.Println("\n--- JPEG Lossless SV1 (Predictor 1) - Lossless ---")
	testCodec(registry, transfer.JPEGLosslessSV1, src, false)

	// Test JPEG Lossless (All Predictors)
	fmt.Println("\n--- JPEG Lossless (Predictor 4) - Lossless ---")
	testCodec(registry, transfer.JPEGLossless, src, false)

	// Comparison
	fmt.Println("\n=== Compression Comparison ===")
	compareCodecs(registry, src)

	// RGB Example
	fmt.Println("\n=== RGB Image Example ===")
	testRGBImage(registry)
}

func testCodec(registry *codec.CodecRegistry, ts *transfer.TransferSyntax, src *codec.PixelData, isLossy bool) {
	// Get codec from registry
	c, exists := registry.GetCodec(ts)
	if !exists {
		fmt.Printf("✗ Codec not found for %s\n", ts.UID().UID())
		return
	}

	fmt.Printf("Codec: %s\n", c.Name())
	fmt.Printf("Transfer Syntax: %s\n", ts.UID().UID())

	// Encode
	encoded := &codec.PixelData{}
	err := c.Encode(src, encoded, nil)
	if err != nil {
		fmt.Printf("✗ Encode failed: %v\n", err)
		return
	}

	ratio := float64(len(src.Data)) / float64(len(encoded.Data))
	fmt.Printf("Compressed: %d bytes (%.2fx)\n", len(encoded.Data), ratio)

	// Decode
	decoded := &codec.PixelData{}
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		fmt.Printf("✗ Decode failed: %v\n", err)
		return
	}

	// Verify
	if isLossy {
		// For lossy, check quality
		maxDiff := 0
		totalDiff := 0
		for i := 0; i < len(src.Data); i++ {
			diff := int(src.Data[i]) - int(decoded.Data[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxDiff {
				maxDiff = diff
			}
			totalDiff += diff
		}
		avgDiff := float64(totalDiff) / float64(len(src.Data))
		fmt.Printf("Quality: Max diff=%d, Avg diff=%.2f\n", maxDiff, avgDiff)
		fmt.Println("✓ Lossy compression completed")
	} else {
		// For lossless, check perfect reconstruction
		errors := 0
		for i := 0; i < len(src.Data); i++ {
			if decoded.Data[i] != src.Data[i] {
				errors++
			}
		}
		if errors == 0 {
			fmt.Printf("✓ Perfect reconstruction: all %d pixels match\n", len(src.Data))
		} else {
			fmt.Printf("✗ Errors: %d pixels differ\n", errors)
		}
	}
}

func compareCodecs(registry *codec.CodecRegistry, src *codec.PixelData) {
	codecs := []struct {
		name string
		ts   *transfer.TransferSyntax
	}{
		{"JPEG Baseline", transfer.JPEGBaseline8Bit},
		{"JPEG Lossless SV1", transfer.JPEGLosslessSV1},
		{"JPEG Lossless", transfer.JPEGLossless},
	}

	fmt.Printf("%-20s %12s %10s %12s\n", "Codec", "Size (bytes)", "Ratio", "Type")
	fmt.Println("─────────────────────────────────────────────────────────")

	for _, entry := range codecs {
		c, exists := registry.GetCodec(entry.ts)
		if !exists {
			continue
		}

		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("%-20s %12s %10s %12s\n", entry.name, "ERROR", "-", "-")
			continue
		}

		ratio := float64(len(src.Data)) / float64(len(encoded.Data))
		codecType := "Lossless"
		if entry.ts == transfer.JPEGBaseline8Bit {
			codecType = "Lossy"
		}

		fmt.Printf("%-20s %12d %9.2fx %12s\n", entry.name, len(encoded.Data), ratio, codecType)
	}
}

func testRGBImage(registry *codec.CodecRegistry) {
	// Create RGB test data (32x32)
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     uint16(width),
		Height:                    uint16(height),
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(components),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
		TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
	}

	fmt.Printf("RGB image: %dx%d, %d bytes\n\n", width, height, len(pixelData))

	// Test with Baseline (best for RGB photos)
	c, exists := registry.GetCodec(transfer.JPEGBaseline8Bit)
	if exists {
		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("RGB Baseline encode failed: %v\n", err)
		} else {
			ratio := float64(len(src.Data)) / float64(len(encoded.Data))
			fmt.Printf("JPEG Baseline: %d bytes (%.2fx compression)\n", len(encoded.Data), ratio)

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				fmt.Printf("RGB Baseline decode failed: %v\n", err)
			} else {
				fmt.Println("✓ RGB lossy compression successful")
			}
		}
	}

	// Test with Lossless SV1
	c, exists = registry.GetCodec(transfer.JPEGLosslessSV1)
	if exists {
		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			fmt.Printf("RGB Lossless SV1 encode failed: %v\n", err)
		} else {
			ratio := float64(len(src.Data)) / float64(len(encoded.Data))
			fmt.Printf("JPEG Lossless SV1: %d bytes (%.2fx compression)\n", len(encoded.Data), ratio)

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				fmt.Printf("RGB Lossless SV1 decode failed: %v\n", err)
			} else {
				// Check if lossless
				errors := 0
				for i := 0; i < len(src.Data); i++ {
					if decoded.Data[i] != src.Data[i] {
						errors++
					}
				}
				if errors == 0 {
					fmt.Println("✓ RGB perfect lossless reconstruction")
				} else {
					fmt.Printf("RGB had %d pixel differences\n", errors)
				}
			}
		}
	}
}
