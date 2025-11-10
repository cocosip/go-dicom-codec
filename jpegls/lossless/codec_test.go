package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

// TestCodecInterface tests the Codec interface implementation
func TestCodecInterface(t *testing.T) {
	c := NewLosslessCodec(8)

	// Test UID
	expectedUID := transfer.JPEGLSLossless.UID().UID()
	if c.UID() != expectedUID {
		t.Errorf("UID mismatch: got %s, want %s", c.UID(), expectedUID)
	}

	// Test Name
	name := c.Name()
	t.Logf("Codec name: %s", name)
	if name == "" {
		t.Error("Name should not be empty")
	}
}

// TestCodecEncodeDecode8Bit tests codec encode/decode with 8-bit data
func TestCodecEncodeDecode8Bit(t *testing.T) {
	c := NewLosslessCodec(8)

	width, height := 64, 64
	components := 1

	// Create test frame
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   8,
	}

	// Encode
	encoded, err := c.Encode(params)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encoded))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decodedFrame, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if decodedFrame.Width != width || decodedFrame.Height != height {
		t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
			decodedFrame.Width, decodedFrame.Height, width, height)
	}

	if decodedFrame.Components != components {
		t.Errorf("Components mismatch: got %d, want %d", decodedFrame.Components, components)
	}

	// Verify lossless
	errors := 0
	for i := range pixelData {
		if i >= len(decodedFrame.PixelData) {
			break
		}
		if decodedFrame.PixelData[i] != pixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedFrame.PixelData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("Perfect lossless compression (0 errors)")
	}
}

// TestCodecEncodeDecode12Bit tests codec with 12-bit data
func TestCodecEncodeDecode12Bit(t *testing.T) {
	c := NewLosslessCodec(12)

	width, height := 32, 32
	components := 1
	maxVal := 4095

	// Create 12-bit test data (stored in 16-bit)
	pixelData := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 2
			val := ((x + y*2) * 16) % (maxVal + 1)
			pixelData[idx] = byte(val & 0xFF)
			pixelData[idx+1] = byte((val >> 8) & 0xFF)
		}
	}

	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   12,
	}

	// Encode
	encoded, err := c.Encode(params)
	if err != nil {
		t.Fatalf("12-bit Encode failed: %v", err)
	}

	t.Logf("12-bit Original size: %d bytes", len(pixelData))
	t.Logf("12-bit Compressed size: %d bytes", len(encoded))
	t.Logf("12-bit Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decodedFrame, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("12-bit Decode failed: %v", err)
	}

	// Verify lossless
	errors := 0
	for i := range pixelData {
		if i >= len(decodedFrame.PixelData) {
			break
		}
		if decodedFrame.PixelData[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("12-bit: Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("12-bit: Perfect lossless compression!")
	}
}

// TestCodecRGB tests codec with RGB data
func TestCodecRGB(t *testing.T) {
	c := NewLosslessCodec(8)

	width, height := 32, 32
	components := 3

	// Create RGB test data
	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * components
			pixelData[idx+0] = byte(x * 8)
			pixelData[idx+1] = byte(y * 8)
			pixelData[idx+2] = byte((x + y) * 4)
		}
	}

	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: components,
		BitDepth:   8,
	}

	// Encode
	encoded, err := c.Encode(params)
	if err != nil {
		t.Fatalf("RGB Encode failed: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(encoded))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decodedFrame, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("RGB Decode failed: %v", err)
	}

	// Verify lossless
	errors := 0
	for i := range pixelData {
		if i >= len(decodedFrame.PixelData) {
			break
		}
		if decodedFrame.PixelData[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("RGB: Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("RGB: Perfect lossless compression!")
	}
}

// TestCodecRegistry tests codec registration
func TestCodecRegistry(t *testing.T) {
	// Register codec (should already be registered in init())
	RegisterLosslessCodec(8)

	// Get codec from registry
	c, err := codec.Get(transfer.JPEGLSLossless.UID().UID())
	if err != nil {
		t.Fatalf("Failed to get codec from registry: %v", err)
	}

	if c == nil {
		t.Fatal("Got nil codec from registry")
	}

	name := c.Name()
	t.Logf("Retrieved codec name: %s", name)

	// Test encode/decode through registry codec
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
	}

	encoded, err := c.Encode(params)
	if err != nil {
		t.Fatalf("Registry codec encode failed: %v", err)
	}

	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Registry codec decode failed: %v", err)
	}

	// Verify
	errors := 0
	for i := range pixelData {
		if decoded.PixelData[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Registry codec: %d pixel errors", errors)
	} else {
		t.Logf("Registry codec test passed: %dx%d image", width, height)
	}
}
