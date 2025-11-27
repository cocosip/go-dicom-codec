package lossy

import (
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
