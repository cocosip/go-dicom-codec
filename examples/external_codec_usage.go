package main

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/lossless"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	fmt.Println("=== JPEG Lossless Codec Usage Example (External Interface) ===\n")

	// Example 1: Direct codec usage
	fmt.Println("Example 1: Direct codec usage")
	directUsage()
	fmt.Println()

	// Example 2: Registry-based usage
	fmt.Println("Example 2: Registry-based usage")
	registryUsage()
	fmt.Println()

	// Example 3: Using parameters
	fmt.Println("Example 3: Using parameters to specify predictor")
	parametersUsage()
	fmt.Println()
}

func directUsage() {
	// Create test image data (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	// Create source PixelData
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

	// Create codec with predictor 4 (Ra + Rb - Rc)
	losslessCodec := lossless.NewLosslessCodec(4)
	fmt.Printf("Codec: %s\n", losslessCodec.Name())

	// Encode
	encoded := &codec.PixelData{}
	err := losslessCodec.Encode(src, encoded, nil)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	fmt.Printf("Original size: %d bytes\n", len(src.Data))
	fmt.Printf("Compressed size: %d bytes\n", len(encoded.Data))
	fmt.Printf("Compression ratio: %.2fx\n", float64(len(src.Data))/float64(len(encoded.Data)))

	// Decode
	decoded := &codec.PixelData{}
	err = losslessCodec.Decode(encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify lossless reconstruction
	errors := 0
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Printf("✓ Perfect lossless reconstruction: all %d pixels match\n", len(src.Data))
	} else {
		fmt.Printf("✗ Reconstruction errors: %d pixels differ\n", errors)
	}
}

func registryUsage() {
	// Register codec with the global registry
	lossless.RegisterLosslessCodec(1) // Register with predictor 1

	// Get codec from registry
	registry := codec.GetGlobalRegistry()
	retrievedCodec, exists := registry.GetCodec(transfer.JPEGLossless)
	if !exists {
		fmt.Println("Codec not found in registry")
		return
	}

	fmt.Printf("Retrieved codec: %s\n", retrievedCodec.Name())

	// Create test data
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

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

	// Encode using retrieved codec
	encoded := &codec.PixelData{}
	err := retrievedCodec.Encode(src, encoded, nil)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	fmt.Printf("Compressed size: %d bytes (%.2fx)\n",
		len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)))

	// Decode
	decoded := &codec.PixelData{}
	err = retrievedCodec.Decode(encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify
	errors := 0
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Println("✓ Registry codec works perfectly")
	} else {
		fmt.Printf("✗ Errors: %d\n", errors)
	}
}

func parametersUsage() {
	// Create codec with auto-select (predictor 0)
	losslessCodec := lossless.NewLosslessCodec(0)

	// Create test data
	width, height := 48, 48
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x*3 + y*5) % 256)
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
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
		TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
	}

	// Create parameters and override predictor
	params := codec.NewBaseParameters()
	params.SetParameter("predictor", 5) // Use predictor 5

	// Encode with parameters
	encoded := &codec.PixelData{}
	err := losslessCodec.Encode(src, encoded, params)
	if err != nil {
		fmt.Printf("Encode error: %v\n", err)
		return
	}

	fmt.Printf("Codec default: %s\n", losslessCodec.Name())
	fmt.Printf("Using predictor from parameters: 5 (Ra + ((Rb - Rc) >> 1))\n")
	fmt.Printf("Compressed size: %d bytes (%.2fx)\n",
		len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)))

	// Decode
	decoded := &codec.PixelData{}
	err = losslessCodec.Decode(encoded, decoded, nil)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// Verify
	errors := 0
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			errors++
		}
	}

	if errors == 0 {
		fmt.Println("✓ Parameters override works correctly")
	} else {
		fmt.Printf("✗ Errors: %d\n", errors)
	}
}
