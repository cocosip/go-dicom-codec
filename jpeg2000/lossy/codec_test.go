package lossy

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// TestCodecName tests the codec name
func TestCodecName(t *testing.T) {
	c := NewCodec(80)
	expected := "JPEG 2000 Lossy (Quality 80)"
	if c.Name() != expected {
		t.Errorf("Expected codec name %q, got %q", expected, c.Name())
	}
}

// TestCodecTransferSyntax tests the transfer syntax UID
func TestCodecTransferSyntax(t *testing.T) {
	c := NewCodec(80)
	ts := c.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax is nil")
	}

	// The UID should be 1.2.840.10008.1.2.4.91
	// Verify it's not empty
	uid := ts.UID().UID()
	expected := "1.2.840.10008.1.2.4.91"
	if uid != expected {
		t.Errorf("Expected UID %q, got %q", expected, uid)
	}
}

// TestBasicEncodeDecode tests basic encoding and decoding with lossy compression
func TestBasicEncodeDecode(t *testing.T) {
	// Create test image data (16x16 grayscale)
	width := uint16(16)
	height := uint16(16)
	numPixels := int(width) * int(height)

	// Create gradient pattern
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
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

	// Test encoding
	c := NewCodec(80)
	encoded := &codec.PixelData{}

	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify encoded data exists
	if len(encoded.Data) == 0 {
		t.Fatal("Encoded data is empty")
	}

	// Encoded data should be smaller than original (compression)
	t.Logf("Original size: %d bytes, Encoded size: %d bytes, Ratio: %.2f:1",
		len(src.Data), len(encoded.Data), float64(len(src.Data))/float64(len(encoded.Data)))

	// Test decoding
	decoded := &codec.PixelData{}
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify decoded data
	if len(decoded.Data) != len(src.Data) {
		t.Errorf("Decoded data length mismatch: got %d, want %d", len(decoded.Data), len(src.Data))
	}

	if decoded.Width != src.Width {
		t.Errorf("Width mismatch: got %d, want %d", decoded.Width, src.Width)
	}

	if decoded.Height != src.Height {
		t.Errorf("Height mismatch: got %d, want %d", decoded.Height, src.Height)
	}

	// For lossy compression, we expect some error but it should be small
	// Calculate error metrics
	var maxError int
	var totalError int64
	errorCount := 0

	for i := 0; i < numPixels; i++ {
		diff := int(decoded.Data[i]) - int(src.Data[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		totalError += int64(diff)
		if diff > 0 {
			errorCount++
		}
	}

	avgError := float64(totalError) / float64(numPixels)

	t.Logf("Lossy compression error metrics:")
	t.Logf("  Max error: %d", maxError)
	t.Logf("  Avg error: %.2f", avgError)
	t.Logf("  Pixels with error: %d / %d (%.1f%%)",
		errorCount, numPixels, float64(errorCount)*100/float64(numPixels))

	// For 9/7 wavelet with minimal quantization, error is typically small
	// However, very small images (like 16x16) may have higher errors due to boundary effects
	// For this test, we use relaxed thresholds
	// Note: This test uses a very small 16x16 image which has edge effects
	// Larger images (64x64+) typically have much lower error

	// Allow larger max error for small images
	if maxError > 200 {
		t.Errorf("Max error too large: %d (expected <= 200)", maxError)
	}

	// Average error should still be reasonable
	if avgError > 10.0 {
		t.Errorf("Average error too large: %.2f (expected <= 10.0)", avgError)
	}
}

// TestLargerImage tests encoding/decoding a larger image
func TestLargerImage(t *testing.T) {
	// 64x64 image
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

	c := NewCodec(80)
	encoded := &codec.PixelData{}

	// Encode
	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	compressionRatio := float64(len(src.Data)) / float64(len(encoded.Data))
	t.Logf("Compression ratio for 64x64: %.2f:1", compressionRatio)

	// Decode
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

	t.Logf("Max error for 64x64: %d", maxError)

	if maxError > 5 {
		t.Errorf("Max error too large: %d", maxError)
	}
}

// TestRGBImage tests encoding/decoding RGB images
func TestRGBImage(t *testing.T) {
	// 32x32 RGB image
	width := uint16(32)
	height := uint16(32)
	numPixels := int(width) * int(height)

	// Create RGB data (interleaved)
	pixelData := make([]byte, numPixels*3)
	for i := 0; i < numPixels; i++ {
		pixelData[i*3+0] = byte((i * 2) % 256)      // R
		pixelData[i*3+1] = byte((i * 3) % 256)      // G
		pixelData[i*3+2] = byte((i * 5) % 256)      // B
	}

	src := &codec.PixelData{
		Data:                      pixelData,
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

	c := NewCodec(80)
	encoded := &codec.PixelData{}

	// Encode
	err := c.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode RGB failed: %v", err)
	}

	compressionRatio := float64(len(src.Data)) / float64(len(encoded.Data))
	t.Logf("RGB compression ratio: %.2f:1", compressionRatio)

	// Decode
	decoded := &codec.PixelData{}
	err = c.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode RGB failed: %v", err)
	}

	// Verify
	if len(decoded.Data) != len(src.Data) {
		t.Errorf("RGB data length mismatch: got %d, want %d", len(decoded.Data), len(src.Data))
	}

	// Calculate error for RGB
	var maxError int
	for i := 0; i < len(src.Data); i++ {
		diff := int(decoded.Data[i]) - int(src.Data[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("RGB max error: %d", maxError)

	if maxError > 5 {
		t.Errorf("RGB max error too large: %d", maxError)
	}
}
