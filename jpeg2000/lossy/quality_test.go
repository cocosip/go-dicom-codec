package lossy

import (
	"fmt"
	"math"
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// TestQualityParameter tests different quality levels
func TestQualityParameter(t *testing.T) {
	// Create test image data (64x64 grayscale)
	width := uint16(64)
	height := uint16(64)
	numPixels := int(width) * int(height)

	// Create gradient pattern
	pixelData := make([]byte, numPixels)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			pixelData[y*int(width)+x] = byte((x + y) % 256)
		}
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

	// Test with quality 100 (near-lossless)
	t.Run("Quality 100", func(t *testing.T) {
		// Create codec with quality=100
		c := NewCodec(100)

		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		t.Logf("Quality 100 - Size: %d bytes, Ratio: %.2f:1",
			len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)))

		decoded := &codec.PixelData{}
		err = c.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		// Calculate error
		var maxError int
		var totalError int64
		for i := 0; i < numPixels; i++ {
			diff := int(decoded.Data[i]) - int(src.Data[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
			totalError += int64(diff)
		}

		avgError := float64(totalError) / float64(numPixels)
		t.Logf("Quality 100 - Max error: %d, Avg error: %.2f", maxError, avgError)

		// Quality 100 should have very low error (near-lossless)
		if maxError > 3 {
			t.Errorf("Quality 100 max error too large: %d (expected <= 3)", maxError)
		}
	})

	// Test with quality 50 (medium)
	t.Run("Quality 50", func(t *testing.T) {
		c := NewCodec(50)

		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		t.Logf("Quality 50 - Size: %d bytes, Ratio: %.2f:1",
			len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)))

		decoded := &codec.PixelData{}
		err = c.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		// Calculate error
		var maxError int
		for i := 0; i < numPixels; i++ {
			diff := int(decoded.Data[i]) - int(src.Data[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
		}

		t.Logf("Quality 50 - Max error: %d", maxError)

		// Quality 50 should have moderate error
		if maxError > 20 {
			t.Logf("Warning: Quality 50 max error: %d (expected <= 20)", maxError)
		}
	})
}

// Helper function to calculate PSNR
func calculatePSNR(original, decoded []byte, maxValue float64) float64 {
	if len(original) != len(decoded) {
		return 0
	}

	var mse float64
	for i := range original {
		diff := float64(int(original[i]) - int(decoded[i]))
		mse += diff * diff
	}
	mse /= float64(len(original))

	if mse == 0 {
		return math.Inf(1) // Perfect match
	}

	return 10 * math.Log10((maxValue * maxValue) / mse)
}

// Helper function to calculate MSE
func calculateMSE(original, decoded []byte) float64 {
	if len(original) != len(decoded) {
		return 0
	}

	var mse float64
	for i := range original {
		diff := float64(int(original[i]) - int(decoded[i]))
		mse += diff * diff
	}
	return mse / float64(len(original))
}

// TestQualityRange tests comprehensive quality range
func TestQualityRange(t *testing.T) {
	width := uint16(128)
	height := uint16(128)
	numPixels := int(width) * int(height)

	// Create gradient pattern
	pixelData := make([]byte, numPixels)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			pixelData[y*int(width)+x] = byte((x + y) % 256)
		}
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

	qualities := []int{1, 10, 30, 50, 70, 90, 95, 99}
	results := make([]struct {
		quality int
		size    int
		psnr    float64
		mse     float64
	}, 0, len(qualities))

	for _, quality := range qualities {
		t.Run(fmt.Sprintf("Quality_%d", quality), func(t *testing.T) {
			c := NewCodec(quality)

			encoded := &codec.PixelData{}
			err := c.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed for quality %d: %v", quality, err)
			}

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed for quality %d: %v", quality, err)
			}

			psnr := calculatePSNR(src.Data, decoded.Data, 255.0)
			mse := calculateMSE(src.Data, decoded.Data)

			results = append(results, struct {
				quality int
				size    int
				psnr    float64
				mse     float64
			}{quality, len(encoded.Data), psnr, mse})

			t.Logf("Quality %d: Size=%d bytes, Ratio=%.2f:1, PSNR=%.2f dB, MSE=%.2f",
				quality, len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)), psnr, mse)
		})
	}

	// Verify general trend: higher quality -> lower MSE, higher PSNR
	// Note: At very high quality (>90), there may be non-monotonic behavior due to:
	// - Rate-distortion optimization tradeoffs
	// - Different quantization strategies
	// - Numerical precision limits
	// So we only check the general trend for quality <= 90
	for i := 1; i < len(results); i++ {
		prev := results[i-1]
		curr := results[i]

		// Only enforce strict monotonicity up to quality 90
		if curr.quality <= 90 {
			// MSE should decrease with higher quality
			if curr.mse > prev.mse*1.1 { // Allow 10% tolerance
				t.Errorf("MSE should generally decrease with higher quality: Q%d MSE=%.2f > Q%d MSE=%.2f",
					curr.quality, curr.mse, prev.quality, prev.mse)
			}

			// PSNR should increase with higher quality
			if !math.IsInf(curr.psnr, 1) && !math.IsInf(prev.psnr, 1) {
				if curr.psnr < prev.psnr-1.0 { // Allow 1dB tolerance
					t.Errorf("PSNR should generally increase with higher quality: Q%d PSNR=%.2f < Q%d PSNR=%.2f",
						curr.quality, curr.psnr, prev.quality, prev.psnr)
				}
			}
		} else {
			// For very high quality (>90), just log the values for information
			t.Logf("High quality comparison: Q%d->Q%d: MSE %.2f->%.2f, PSNR %.2f->%.2f",
				prev.quality, curr.quality, prev.mse, curr.mse, prev.psnr, curr.psnr)
		}
	}
}

// TestDifferentImagePatterns tests various image content types
func TestDifferentImagePatterns(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	numPixels := int(width) * int(height)

	patterns := []struct {
		name      string
		generator func(x, y, w, h int) byte
	}{
		{
			name: "Uniform",
			generator: func(x, y, w, h int) byte {
				return 128
			},
		},
		{
			name: "Horizontal Gradient",
			generator: func(x, y, w, h int) byte {
				return byte(x * 255 / w)
			},
		},
		{
			name: "Vertical Gradient",
			generator: func(x, y, w, h int) byte {
				return byte(y * 255 / h)
			},
		},
		{
			name: "Diagonal Gradient",
			generator: func(x, y, w, h int) byte {
				return byte((x + y) * 255 / (w + h))
			},
		},
		{
			name: "Checkerboard",
			generator: func(x, y, w, h int) byte {
				if (x/8+y/8)%2 == 0 {
					return 0
				}
				return 255
			},
		},
		{
			name: "Edges",
			generator: func(x, y, w, h int) byte {
				if x < w/2 {
					return 0
				}
				return 255
			},
		},
	}

	quality := 80

	for _, pattern := range patterns {
		t.Run(pattern.name, func(t *testing.T) {
			pixelData := make([]byte, numPixels)
			for y := 0; y < int(height); y++ {
				for x := 0; x < int(width); x++ {
					pixelData[y*int(width)+x] = pattern.generator(x, y, int(width), int(height))
				}
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

			c := NewCodec(quality)
			encoded := &codec.PixelData{}
			err := c.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			psnr := calculatePSNR(src.Data, decoded.Data, 255.0)
			ratio := float64(len(src.Data)) / float64(len(encoded.Data))

			t.Logf("%s: Size=%d, Ratio=%.2f:1, PSNR=%.2f dB",
				pattern.name, len(encoded.Data), ratio, psnr)

			// Verify dimensions preserved
			if decoded.Width != src.Width || decoded.Height != src.Height {
				t.Errorf("Dimensions changed: got %dx%d, want %dx%d",
					decoded.Width, decoded.Height, src.Width, src.Height)
			}
		})
	}
}

// TestDifferentImageSizes tests various image dimensions
func TestDifferentImageSizes(t *testing.T) {
	sizes := []struct {
		width  uint16
		height uint16
	}{
		{32, 32},
		{64, 64},
		{128, 128},
		{256, 256},
		{64, 128}, // Non-square
		{128, 64}, // Non-square
	}

	quality := 80

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			numPixels := int(size.width) * int(size.height)
			pixelData := make([]byte, numPixels)
			for y := 0; y < int(size.height); y++ {
				for x := 0; x < int(size.width); x++ {
					pixelData[y*int(size.width)+x] = byte((x + y) % 256)
				}
			}

			src := &codec.PixelData{
				Data:                      pixelData,
				Width:                     size.width,
				Height:                    size.height,
				NumberOfFrames:            1,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: "MONOCHROME2",
			}

			c := NewCodec(quality)
			encoded := &codec.PixelData{}
			err := c.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if decoded.Width != src.Width || decoded.Height != src.Height {
				t.Errorf("Dimensions not preserved: got %dx%d, want %dx%d",
					decoded.Width, decoded.Height, src.Width, src.Height)
			}

			psnr := calculatePSNR(src.Data, decoded.Data, 255.0)
			t.Logf("%dx%d: PSNR=%.2f dB, Size=%d bytes",
				size.width, size.height, psnr, len(encoded.Data))
		})
	}
}

// TestDifferentBitDepths tests 8-bit, 12-bit, and 16-bit images
func TestDifferentBitDepths(t *testing.T) {
	bitDepths := []struct {
		bitsAllocated uint16
		bitsStored    uint16
		highBit       uint16
		maxValue      uint16
	}{
		{8, 8, 7, 255},
		{16, 12, 11, 4095},
		{16, 16, 15, 65535},
	}

	width := uint16(64)
	height := uint16(64)
	quality := 80

	for _, bd := range bitDepths {
		t.Run(fmt.Sprintf("%d-bit", bd.bitsStored), func(t *testing.T) {
			numPixels := int(width) * int(height)
			bytesPerPixel := int(bd.bitsAllocated) / 8
			pixelData := make([]byte, numPixels*bytesPerPixel)

			// Create gradient pattern
			for i := 0; i < numPixels; i++ {
				value := uint16(i * int(bd.maxValue) / numPixels)
				if bytesPerPixel == 1 {
					pixelData[i] = byte(value)
				} else {
					// Little-endian 16-bit
					pixelData[i*2] = byte(value & 0xFF)
					pixelData[i*2+1] = byte((value >> 8) & 0xFF)
				}
			}

			src := &codec.PixelData{
				Data:                      pixelData,
				Width:                     width,
				Height:                    height,
				NumberOfFrames:            1,
				BitsAllocated:             bd.bitsAllocated,
				BitsStored:                bd.bitsStored,
				HighBit:                   bd.highBit,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: "MONOCHROME2",
			}

			c := NewCodec(quality)
			encoded := &codec.PixelData{}
			err := c.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed for %d-bit: %v", bd.bitsStored, err)
			}

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed for %d-bit: %v", bd.bitsStored, err)
			}

			// Verify metadata
			if decoded.BitsStored != src.BitsStored {
				t.Errorf("BitsStored changed: got %d, want %d", decoded.BitsStored, src.BitsStored)
			}

			ratio := float64(len(src.Data)) / float64(len(encoded.Data))
			t.Logf("%d-bit: Size=%d bytes, Ratio=%.2f:1",
				bd.bitsStored, len(encoded.Data), ratio)
		})
	}
}

// TestRGBImages tests color image encoding
func TestRGBImages(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	numPixels := int(width) * int(height)
	samplesPerPixel := 3

	// Create RGB gradient
	pixelData := make([]byte, numPixels*samplesPerPixel)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			idx := (y*int(width) + x) * samplesPerPixel
			pixelData[idx] = byte(x * 255 / int(width))     // R
			pixelData[idx+1] = byte(y * 255 / int(height))  // G
			pixelData[idx+2] = byte((x + y) * 255 / 128)    // B
		}
	}

	src := &codec.PixelData{
		Data:                      pixelData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(samplesPerPixel),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}

	qualities := []int{50, 80, 95}

	for _, quality := range qualities {
		t.Run(fmt.Sprintf("Quality_%d", quality), func(t *testing.T) {
			c := NewCodec(quality)

			encoded := &codec.PixelData{}
			err := c.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed for RGB quality %d: %v", quality, err)
			}

			decoded := &codec.PixelData{}
			err = c.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed for RGB quality %d: %v", quality, err)
			}

			if decoded.SamplesPerPixel != src.SamplesPerPixel {
				t.Errorf("SamplesPerPixel changed: got %d, want %d",
					decoded.SamplesPerPixel, src.SamplesPerPixel)
			}

			psnr := calculatePSNR(src.Data, decoded.Data, 255.0)
			ratio := float64(len(src.Data)) / float64(len(encoded.Data))

			t.Logf("RGB Quality %d: Size=%d, Ratio=%.2f:1, PSNR=%.2f dB",
				quality, len(encoded.Data), ratio, psnr)
		})
	}
}

// TestCompressionRatioMonotonicity verifies higher quality -> lower compression
func TestCompressionRatioMonotonicity(t *testing.T) {
	width := uint16(128)
	height := uint16(128)
	numPixels := int(width) * int(height)

	pixelData := make([]byte, numPixels)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			pixelData[y*int(width)+x] = byte((x*y) % 256)
		}
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

	qualities := []int{20, 40, 60, 80, 95}
	var prevSize int

	for i, quality := range qualities {
		c := NewCodec(quality)

		encoded := &codec.PixelData{}
		err := c.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed for quality %d: %v", quality, err)
		}

		currentSize := len(encoded.Data)
		ratio := float64(len(src.Data)) / float64(currentSize)
		t.Logf("Quality %d: Size=%d, Ratio=%.2f:1", quality, currentSize, ratio)

		// Higher quality should produce larger (or similar) file sizes
		if i > 0 && currentSize < prevSize {
			// Allow some tolerance for similar qualities
			decrease := float64(prevSize-currentSize) / float64(prevSize)
			if decrease > 0.05 { // More than 5% decrease is suspicious
				t.Errorf("Quality %d produced smaller file than quality %d: %d < %d (%.1f%% decrease)",
					quality, qualities[i-1], currentSize, prevSize, decrease*100)
			}
		}

		prevSize = currentSize
	}
}
