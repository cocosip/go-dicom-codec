package main

import (
	"fmt"
	"math/rand"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

// This example demonstrates Region of Interest (ROI) encoding
// ROI allows encoding important regions with higher quality while
// compressing the background more aggressively

func main() {
	fmt.Println("=== JPEG 2000 ROI Example ===")

	// Create sample image (512x512)
	width, height := 512, 512
	pixelData := generateImageWithROI(width, height)

	// Example 1: Simple rectangle ROI
	fmt.Println("1. Simple Rectangle ROI")
	simpleROIExample(pixelData, width, height)
	fmt.Println()

	// Example 2: Multiple ROI regions
	fmt.Println("2. Multiple ROI Regions")
	multipleROIExample(pixelData, width, height)
	fmt.Println()

	// Example 3: Circular ROI using mask
	fmt.Println("3. Circular ROI (Mask-based)")
	circularROIExample(pixelData, width, height)
}

// simpleROIExample demonstrates basic rectangle ROI encoding
func simpleROIExample(pixelData []byte, width, height int) {
	// Define ROI: center 200x200 region
	roi := &jpeg2000.ROIParams{
		X0:     156, // (512-200)/2
		Y0:     156,
		Width:  200,
		Height: 200,
		Shift:  5, // Background shift (0-7, higher = more compression)
	}

	// Create encoding parameters
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.Lossless = false
	params.Rate = 60    // Low quality for background
	params.NumLevels = 5
	params.ROI = roi       // ROI gets higher quality

	// Encode
	encoder := jpeg2000.NewEncoder(params)
	compressed, err := encoder.Encode(pixelData)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	// Report results
	ratio := float64(len(pixelData)) / float64(len(compressed))
	fmt.Printf("   Image size: %dx%d\n", width, height)
	fmt.Printf("   ROI region: %dx%d at (%d, %d)\n",
		roi.Width, roi.Height, roi.X0, roi.Y0)
	fmt.Printf("   Background quality: %d\n", params.Rate)
	fmt.Printf("   ROI shift: %d (background compressed %dx more)\n",
		roi.Shift, 1<<roi.Shift)
	fmt.Printf("   Compression: %.2fx\n", ratio)
	fmt.Println("   鉁?ROI encoded with higher quality than background")

	// Decode and verify
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(compressed)
	if err != nil {
		fmt.Printf("   Decode ERROR: %v\n", err)
		return
	}

	fmt.Printf("   Decoded: %dx%d\n", decoder.Width(), decoder.Height())
}

// multipleROIExample demonstrates multiple ROI regions
func multipleROIExample(pixelData []byte, width, height int) {
	// Define multiple ROI regions (e.g., multiple lesions in medical image)
	roiConfig := &jpeg2000.ROIConfig{
		DefaultShift: 5,
		DefaultStyle: jpeg2000.ROIStyleMaxShift,
		ROIs: []jpeg2000.ROIRegion{
			{
				ID:    "lesion1",
				Rect:  &jpeg2000.ROIParams{X0: 100, Y0: 100, Width: 80, Height: 80},
				Shift: 6, // Higher shift = more background compression
			},
			{
				ID:    "lesion2",
				Rect:  &jpeg2000.ROIParams{X0: 300, Y0: 200, Width: 60, Height: 60},
				Shift: 5,
			},
			{
				ID:    "lesion3",
				Rect:  &jpeg2000.ROIParams{X0: 150, Y0: 350, Width: 70, Height: 70},
				Shift: 5,
			},
		},
	}

	// Create encoding parameters
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.Lossless = false
	params.Rate = 50      // Very low background quality
	params.NumLevels = 5
	params.ROIConfig = roiConfig

	// Encode
	encoder := jpeg2000.NewEncoder(params)
	compressed, err := encoder.Encode(pixelData)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	// Report results
	ratio := float64(len(pixelData)) / float64(len(compressed))
	fmt.Printf("   Number of ROI regions: %d\n", len(roiConfig.ROIs))
	fmt.Printf("   Background quality: %d\n", params.Rate)
	fmt.Printf("   Compression: %.2fx\n", ratio)

	for i, roi := range roiConfig.ROIs {
		fmt.Printf("   ROI #%d (%s): %dx%d at (%d,%d), shift=%d\n",
			i+1, roi.ID,
			roi.Rect.Width, roi.Rect.Height,
			roi.Rect.X0, roi.Rect.Y0,
			roi.Shift)
	}

	fmt.Println("   鉁?Multiple ROI regions encoded")
}

// circularROIExample demonstrates mask-based ROI (circular region)
func circularROIExample(pixelData []byte, width, height int) {
	// Create circular mask
	centerX, centerY, radius := width/2, height/2, 100

	mask := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx, dy := x-centerX, y-centerY
			if dx*dx+dy*dy <= radius*radius {
				mask[y*width+x] = true // ROI pixel
			}
		}
	}

	// Create ROI config with mask
	roiConfig := &jpeg2000.ROIConfig{
		ROIs: []jpeg2000.ROIRegion{
			{
				ID:         "circular_roi",
				Shape:      jpeg2000.ROIShapeMask,
				MaskWidth:  width,
				MaskHeight: height,
				MaskData:   mask,
				Shift:      5,
			},
		},
	}

	// Create encoding parameters
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.Lossless = false
	params.Rate = 60
	params.NumLevels = 5
	params.ROIConfig = roiConfig

	// Encode
	encoder := jpeg2000.NewEncoder(params)
	compressed, err := encoder.Encode(pixelData)
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		return
	}

	// Count ROI pixels
	roiPixels := 0
	for _, p := range mask {
		if p {
			roiPixels++
		}
	}

	// Report results
	ratio := float64(len(pixelData)) / float64(len(compressed))
	roiPercent := float64(roiPixels) / float64(width*height) * 100

	fmt.Printf("   Image size: %dx%d\n", width, height)
	fmt.Printf("   ROI shape: Circular (radius=%d pixels)\n", radius)
	fmt.Printf("   ROI coverage: %.1f%% of image\n", roiPercent)
	fmt.Printf("   Background quality: %d\n", params.Rate)
	fmt.Printf("   Compression: %.2fx\n", ratio)
	fmt.Println("   鉁?Circular ROI encoded using bitmap mask")

	// Note about polygon ROI
	fmt.Println("\n   馃挕 Tip: You can also use polygon ROI:")
	fmt.Println("      ROIShapePolygon with Polygon: []Point{{X,Y}, ...}")
}

// Helper functions

func generateImageWithROI(width, height int) []byte {
	// Generate image with distinct regions (simulating medical image with lesions)
	data := make([]byte, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Background: gradient
			value := uint8((x + y) * 255 / (width + height))

			// Add "lesion" regions (brighter spots)
			if isInCircle(x, y, 140, 140, 40) {
				value = 200 + uint8(rand.Intn(55)) // Bright region
			} else if isInCircle(x, y, 340, 240, 30) {
				value = 180 + uint8(rand.Intn(75))
			} else if isInCircle(x, y, 190, 390, 35) {
				value = 190 + uint8(rand.Intn(65))
			} else {
				// Add noise to background
				noise := int8(rand.Intn(20) - 10)
				value = clampUint8(int(value) + int(noise))
			}

			data[y*width+x] = value
		}
	}

	return data
}

func isInCircle(x, y, centerX, centerY, radius int) bool {
	dx, dy := x-centerX, y-centerY
	return dx*dx+dy*dy <= radius*radius
}

func clampUint8(val int) uint8 {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return uint8(val)
}
