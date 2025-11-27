package lossy_test

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// ExampleCodec_Encode demonstrates basic lossy encoding
func ExampleCodec_Encode() {
	// Create a simple 16x16 grayscale test image
	width := uint16(16)
	height := uint16(16)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Prepare source pixel data
	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Create codec with default quality (80)
	c := lossy.NewCodec(80)

	// Encode
	dst := &codec.PixelData{}
	err := c.Encode(src, dst, nil)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	fmt.Printf("Original size: %d bytes\n", len(src.Data))
	fmt.Printf("Encoded size: %d bytes\n", len(dst.Data))
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(src.Data))/float64(len(dst.Data)))
}

// ExampleCodec_Encode_withQualityParameter demonstrates encoding with custom quality
func ExampleCodec_Encode_withQualityParameter() {
	// Create test image
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Test different quality levels
	qualities := []int{100, 80, 50}

	for _, quality := range qualities {
		// Create codec with specific quality
		c := lossy.NewCodec(quality)

		// Alternatively, you can override quality via parameters:
		// c := lossy.NewCodec(80) // default
		// params := codec.NewParameters()
		// params.SetParameter("quality", quality)
		// err := c.Encode(src, dst, params)

		dst := &codec.PixelData{}
		err := c.Encode(src, dst, nil)
		if err != nil {
			log.Fatalf("Encoding failed: %v", err)
		}

		// Decode to check quality
		decoded := &codec.PixelData{}
		err = c.Decode(dst, decoded, nil)
		if err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}

		// Calculate error
		var maxError int
		for i := range src.Data {
			diff := int(decoded.Data[i]) - int(src.Data[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
		}

		compressionRatio := float64(len(src.Data)) / float64(len(dst.Data))
		fmt.Printf("Quality %d: Compression %.2f:1, Max error: %d\n",
			quality, compressionRatio, maxError)
	}

	// Expected approximate results (may vary slightly):
	// Quality 100: Compression ~3:1, Max error: ≤1
	// Quality 80: Compression ~3-4:1, Max error: ≤3
	// Quality 50: Compression ~5-6:1, Max error: ~12-15
}

// ExampleCodec_Decode demonstrates basic lossy decoding
func ExampleCodec_Decode() {
	// Assume we have encoded data (in real use, this comes from DICOM file)
	// For this example, we'll encode first
	width := uint16(16)
	height := uint16(16)
	pixelData := make([]byte, int(width)*int(height))
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Encode
	c := lossy.NewCodec(80)
	encoded := &codec.PixelData{}
	_ = c.Encode(src, encoded, nil)

	// Decode
	decoded := &codec.PixelData{}
	err := c.Decode(encoded, decoded, nil)
	if err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}

	fmt.Printf("Decoded width: %d\n", decoded.Width)
	fmt.Printf("Decoded height: %d\n", decoded.Height)
	fmt.Printf("Decoded data size: %d bytes\n", len(decoded.Data))
}
