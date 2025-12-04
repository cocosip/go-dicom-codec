package htj2k

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// TestHTJ2KLosslessRoundTrip tests HTJ2K lossless encoding and decoding
func TestHTJ2KLosslessRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		width  uint16
		height uint16
	}{
		{"16x16", 16, 16},
		{"64x64", 64, 64},
		{"128x128", 128, 128},
		{"256x256", 256, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test image (grayscale gradient)
			size := int(tt.width * tt.height)
			testData := make([]byte, size)
			for i := 0; i < size; i++ {
				testData[i] = byte(i % 256)
			}

			// Create source PixelData
			src := &codec.PixelData{
				Data:                      testData,
				Width:                     tt.width,
				Height:                    tt.height,
				NumberOfFrames:            1,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: "MONOCHROME2",
			}

			// Create HTJ2K lossless codec
			htCodec := NewLosslessCodec()

			// Encode
			encoded := &codec.PixelData{}
			err := htCodec.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			t.Logf("Original size: %d bytes", len(testData))
			t.Logf("Encoded size: %d bytes", len(encoded.Data))
			t.Logf("Compression ratio: %.2f:1", float64(len(testData))/float64(len(encoded.Data)))

			// Decode
			decoded := &codec.PixelData{
				Width:               tt.width,
				Height:              tt.height,
				BitsAllocated:       8,
				BitsStored:          8,
				SamplesPerPixel:     1,
				PixelRepresentation: 0,
			}
			err = htCodec.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify perfect reconstruction (lossless)
			if len(decoded.Data) != len(testData) {
				t.Fatalf("Decoded data size mismatch: got %d, want %d", len(decoded.Data), len(testData))
			}

			errors := 0
			maxError := 0
			for i := 0; i < len(testData); i++ {
				diff := int(testData[i]) - int(decoded.Data[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					errors++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			t.Logf("Pixel errors: %d/%d", errors, len(testData))
			t.Logf("Max error: %d", maxError)

			// For lossless, we expect perfect reconstruction
			if errors > 0 {
				t.Errorf("Lossless mode should have 0 errors, got %d errors with max error %d", errors, maxError)
			}
		})
	}
}

// TestHTJ2KLosslessRPCLRoundTrip tests HTJ2K lossless RPCL encoding
func TestHTJ2KLosslessRPCLRoundTrip(t *testing.T) {
	// Create test image
	width := uint16(64)
	height := uint16(64)
	size := int(width * height)
	testData := make([]byte, size)
	for i := 0; i < size; i++ {
		testData[i] = byte(i % 256)
	}

	src := &codec.PixelData{
		Data:                      testData,
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

	// Create HTJ2K lossless RPCL codec
	htCodec := NewLosslessRPCLCodec()

	// Encode
	encoded := &codec.PixelData{}
	err := htCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("RPCL Compression ratio: %.2f:1", float64(len(testData))/float64(len(encoded.Data)))

	// Decode
	decoded := &codec.PixelData{
		Width:               width,
		Height:              height,
		BitsAllocated:       8,
		BitsStored:          8,
		SamplesPerPixel:     1,
		PixelRepresentation: 0,
	}
	err = htCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(testData); i++ {
		if testData[i] != decoded.Data[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Lossless RPCL mode should have 0 errors, got %d", errors)
	}
}

// TestHTJ2KLossyRoundTrip tests HTJ2K lossy encoding and decoding
func TestHTJ2KLossyRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		quality int
		width   uint16
		height  uint16
	}{
		{"64x64_Q50", 50, 64, 64},
		{"64x64_Q80", 80, 64, 64},
		{"128x128_Q70", 70, 128, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test image
			size := int(tt.width * tt.height)
			testData := make([]byte, size)
			for i := 0; i < size; i++ {
				testData[i] = byte(i % 256)
			}

			src := &codec.PixelData{
				Data:                      testData,
				Width:                     tt.width,
				Height:                    tt.height,
				NumberOfFrames:            1,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: "MONOCHROME2",
			}

			// Create HTJ2K lossy codec
			htCodec := NewCodec(tt.quality)

			// Encode
			encoded := &codec.PixelData{}
			err := htCodec.Encode(src, encoded, nil)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			t.Logf("Quality %d - Compression ratio: %.2f:1", tt.quality, float64(len(testData))/float64(len(encoded.Data)))

			// Decode
			decoded := &codec.PixelData{
				Width:               tt.width,
				Height:              tt.height,
				BitsAllocated:       8,
				BitsStored:          8,
				SamplesPerPixel:     1,
				PixelRepresentation: 0,
			}
			err = htCodec.Decode(encoded, decoded, nil)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Calculate error metrics
			var sumSquaredError int64
			maxError := 0
			for i := 0; i < len(testData); i++ {
				diff := int(testData[i]) - int(decoded.Data[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				sumSquaredError += int64(diff * diff)
			}

			mse := float64(sumSquaredError) / float64(len(testData))
			psnr := 10 * (20 - (0.5 * float64(mse)))

			t.Logf("Max error: %d", maxError)
			t.Logf("MSE: %.2f", mse)
			t.Logf("PSNR: %.2f dB", psnr)

			// For lossy, check reasonable error bounds
			if maxError > 50 {
				t.Errorf("Max error too high: %d (expected < 50)", maxError)
			}
		})
	}
}

// TestHTJ2KRGBRoundTrip tests HTJ2K with RGB images
func TestHTJ2KRGBRoundTrip(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	size := int(width * height * 3) // RGB

	// Create RGB test image
	testData := make([]byte, size)
	for i := 0; i < int(width*height); i++ {
		testData[i*3+0] = byte(i % 256)        // R
		testData[i*3+1] = byte((i * 2) % 256)  // G
		testData[i*3+2] = byte((i * 3) % 256)  // B
	}

	src := &codec.PixelData{
		Data:                      testData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           3,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}

	// Test lossless
	t.Run("RGB_Lossless", func(t *testing.T) {
		htCodec := NewLosslessCodec()

		encoded := &codec.PixelData{}
		err := htCodec.Encode(src, encoded, nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		t.Logf("RGB Compression ratio: %.2f:1", float64(len(testData))/float64(len(encoded.Data)))

		decoded := &codec.PixelData{
			Width:               width,
			Height:              height,
			BitsAllocated:       8,
			BitsStored:          8,
			SamplesPerPixel:     3,
			PixelRepresentation: 0,
		}
		err = htCodec.Decode(encoded, decoded, nil)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		// Verify perfect reconstruction
		errors := 0
		for i := 0; i < len(testData); i++ {
			if testData[i] != decoded.Data[i] {
				errors++
			}
		}

		t.Logf("RGB errors: %d/%d", errors, len(testData))
		if errors > 0 {
			t.Errorf("RGB lossless should have 0 errors, got %d", errors)
		}
	})
}

// TestHTJ2K12BitRoundTrip tests HTJ2K with 12-bit images
func TestHTJ2K12BitRoundTrip(t *testing.T) {
	width := uint16(64)
	height := uint16(64)
	size := int(width * height * 2) // 12-bit stored as uint16

	// Create 12-bit test image (stored as uint16)
	testData := make([]byte, size)
	for i := 0; i < int(width*height); i++ {
		val := uint16(i % 4096) // 12-bit range: 0-4095
		testData[i*2] = byte(val & 0xFF)
		testData[i*2+1] = byte((val >> 8) & 0xFF)
	}

	src := &codec.PixelData{
		Data:                      testData,
		Width:                     width,
		Height:                    height,
		NumberOfFrames:            1,
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	htCodec := NewLosslessCodec()

	// Encode
	encoded := &codec.PixelData{}
	err := htCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("12-bit Compression ratio: %.2f:1", float64(len(testData))/float64(len(encoded.Data)))

	// Decode
	decoded := &codec.PixelData{
		Width:               width,
		Height:              height,
		BitsAllocated:       16,
		BitsStored:          12,
		SamplesPerPixel:     1,
		PixelRepresentation: 0,
	}
	err = htCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	maxError := 0
	for i := 0; i < int(width*height); i++ {
		origVal := uint16(testData[i*2]) | (uint16(testData[i*2+1]) << 8)
		decVal := uint16(decoded.Data[i*2]) | (uint16(decoded.Data[i*2+1]) << 8)

		diff := int(origVal) - int(decVal)
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			errors++
			if diff > maxError {
				maxError = diff
			}
		}
	}

	t.Logf("12-bit errors: %d/%d", errors, int(width*height))
	t.Logf("Max error: %d", maxError)

	if errors > 0 {
		t.Errorf("12-bit lossless should have 0 errors, got %d errors with max %d", errors, maxError)
	}
}
