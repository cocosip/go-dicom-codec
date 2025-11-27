package lossy_test

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Example demonstrates the recommended type-safe way to use parameters
func Example_typeSafeParameters() {
	// Create test image data
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

	// Method 1: Type-safe parameters (RECOMMENDED)
	c := lossy.NewCodec(80)
	params := lossy.NewLossyParameters().
		WithQuality(95).
		WithNumLevels(5)

	dst := &codec.PixelData{}
	err := c.Encode(src, dst, params)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	fmt.Printf("Encoded with quality %d\n", params.Quality)
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(src.Data))/float64(len(dst.Data)))

	// Output will vary slightly but should be close to:
	// Encoded with quality 95
	// Compression ratio: ~3.2:1
}

// Example_legacyParameters demonstrates backward compatibility
func Example_legacyParameters() {
	// Create test image (same as above)
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))

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

	// Method 2: Generic parameters (for backward compatibility)
	c := lossy.NewCodec(80)

	// Using any codec.Parameters implementation
	params := lossy.NewLossyParameters()
	params.SetParameter("quality", 90)
	params.SetParameter("numLevels", 3)

	dst := &codec.PixelData{}
	err := c.Encode(src, dst, params)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	// Can read back the parameters
	quality := params.GetParameter("quality").(int)
	fmt.Printf("Used quality: %d\n", quality)

	// Output:
	// Used quality: 90
}

// Example_directFieldAccess shows direct field access
func Example_directFieldAccess() {
	// Most convenient and type-safe way
	params := lossy.NewLossyParameters()

	// Direct field access with IDE autocomplete
	params.Quality = 85
	params.NumLevels = 4

	// Validate before use
	params.Validate()

	fmt.Printf("Quality: %d, Levels: %d\n", params.Quality, params.NumLevels)

	// Output:
	// Quality: 85, Levels: 4
}

// Example_comparisonWithOldWay demonstrates the improvement
func Example_comparisonWithOldWay() {
	fmt.Println("=== OLD WAY (string-based) ===")
	// Problems:
	// 1. Easy to mistype parameter name
	// 2. No IDE autocomplete
	// 3. Type assertion needed
	// 4. No compile-time checking

	oldParams := lossy.NewLossyParameters()
	oldParams.SetParameter("quality", 95) // What if you type "qualty"?
	quality := oldParams.GetParameter("quality").(int) // Needs type assertion

	fmt.Printf("Quality: %d\n", quality)

	fmt.Println("\n=== NEW WAY (type-safe) ===")
	// Benefits:
	// 1. Compile-time checking
	// 2. IDE autocomplete works
	// 3. No type assertion needed
	// 4. Self-documenting

	newParams := lossy.NewLossyParameters().
		WithQuality(95). // Chaining supported
		WithNumLevels(5)

	// Or direct field access:
	newParams.Quality = 95 // Even simpler!
	newParams.NumLevels = 5

	fmt.Printf("Quality: %d, Levels: %d\n", newParams.Quality, newParams.NumLevels)

	// Output:
	// === OLD WAY (string-based) ===
	// Quality: 95
	//
	// === NEW WAY (type-safe) ===
	// Quality: 95, Levels: 5
}
