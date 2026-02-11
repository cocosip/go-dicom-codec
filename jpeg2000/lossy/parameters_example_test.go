package lossy_test

import (
	"fmt"
	"log"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
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

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	src.AddFrame(pixelData)

	// Method 1: Type-safe parameters (RECOMMENDED)
	c := lossy.NewCodecWithRate(80)
	params := lossy.NewLossyParameters().
		WithRate(95).
		WithNumLevels(5)

	dst := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(src, dst, params)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	srcData, _ := src.GetFrame(0)
	dstData, _ := dst.GetFrame(0)
	fmt.Printf("Encoded with rate %d\n", params.Rate)
	fmt.Printf("Compression ratio: %.2f:1\n", float64(len(srcData))/float64(len(dstData)))

	// Output will vary slightly but should be close to:
	// Encoded with rate 95
	// Compression ratio: ~3.2:1
}

// Example_legacyParameters demonstrates backward compatibility
func Example_legacyParameters() {
	// Create test image (same as above)
	width := uint16(64)
	height := uint16(64)
	pixelData := make([]byte, int(width)*int(height))

	frameInfo := &imagetypes.FrameInfo{
		Width:                     width,
		Height:                    height,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	src.AddFrame(pixelData)

	// Method 2: Generic parameters (for backward compatibility)
	c := lossy.NewCodecWithRate(80)

	// Using any codec.Parameters implementation
	params := lossy.NewLossyParameters()
	params.SetParameter("rate", 90)
	params.SetParameter("numLevels", 3)

	dst := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(src, dst, params)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	// Can read back the parameters
	rate := params.GetParameter("rate").(int)
	fmt.Printf("Used rate: %d\n", rate)

	// Output:
	// Used rate: 90
}

// Example_directFieldAccess shows direct field access
func Example_directFieldAccess() {
	// Most convenient and type-safe way
	params := lossy.NewLossyParameters()

	// Direct field access with IDE autocomplete
	params.Rate = 85
	params.NumLevels = 4

	// Validate before use
	params.Validate()

	fmt.Printf("Rate: %d, Levels: %d\n", params.Rate, params.NumLevels)

	// Output:
	// Rate: 85, Levels: 4
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
	oldParams.SetParameter("rate", 95)           // What if you type "qualty"?
	rate := oldParams.GetParameter("rate").(int) // Needs type assertion

	fmt.Printf("Rate: %d\n", rate)

	fmt.Println("\n=== NEW WAY (type-safe) ===")
	// Benefits:
	// 1. Compile-time checking
	// 2. IDE autocomplete works
	// 3. No type assertion needed
	// 4. Self-documenting

	newParams := lossy.NewLossyParameters().
		WithRate(95). // Chaining supported
		WithNumLevels(5)

	// Or direct field access:
	newParams.Rate = 95 // Even simpler!
	newParams.NumLevels = 5

	fmt.Printf("Rate: %d, Levels: %d\n", newParams.Rate, newParams.NumLevels)

	// Output:
	// === OLD WAY (string-based) ===
	// Rate: 95
	//
	// === NEW WAY (type-safe) ===
	// Rate: 95, Levels: 5
}
