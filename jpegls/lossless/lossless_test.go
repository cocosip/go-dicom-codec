package lossless

import (
	"testing"
)

// TestEncodeDecode8BitGrayscale tests 8-bit grayscale encoding and decoding
func TestEncodeDecode8BitGrayscale(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1

	// Create test image with gradient
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			pixelData[idx] = byte((x + y*2) % 256)
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(encoded))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height || c != components || bd != bitDepth {
		t.Errorf("Dimension mismatch: got %dx%d, %d components, %d-bit; want %dx%d, %d components, %d-bit",
			w, h, c, bd, width, height, components, bitDepth)
	}

	// Verify lossless: all pixels must match exactly
	if len(decoded) != len(pixelData) {
		t.Fatalf("Decoded data length mismatch: got %d, want %d", len(decoded), len(pixelData))
	}

	errors := 0
	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			errors++
			if errors <= 10 { // Show first 10 errors
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decoded[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("Perfect lossless reconstruction! (0 errors)")
	}
}

// TestEncodeDecode8BitRGB tests 8-bit RGB encoding and decoding
func TestEncodeDecode8BitRGB(t *testing.T) {
	width, height := 32, 32
	bitDepth := 8
	components := 3

	// Create test RGB image
	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * components
			pixelData[idx+0] = byte((x * 8) % 256)      // R
			pixelData[idx+1] = byte((y * 8) % 256)      // G
			pixelData[idx+2] = byte(((x + y) * 4) % 256) // B
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(pixelData))
	t.Logf("RGB Compressed size: %d bytes", len(encoded))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height || c != components || bd != bitDepth {
		t.Errorf("Dimension mismatch")
	}

	// Verify lossless
	errors := 0
	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("RGB: Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("RGB: Perfect lossless reconstruction!")
	}
}

// TestEncodeDecode12Bit tests 12-bit encoding and decoding
func TestEncodeDecode12Bit(t *testing.T) {
	width, height := 32, 32
	bitDepth := 12
	components := 1
	maxVal := (1 << bitDepth) - 1 // 4095

	// Create test image (16-bit storage, but values in 12-bit range)
	pixelData := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 2
			val := ((x + y*2) * 4) % (maxVal + 1)
			pixelData[idx] = byte(val & 0xFF)
			pixelData[idx+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("12-bit Original size: %d bytes", len(pixelData))
	t.Logf("12-bit Compressed size: %d bytes", len(encoded))
	t.Logf("12-bit Compression ratio: %.2fx", float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height || c != components || bd != bitDepth {
		t.Errorf("Dimension mismatch")
	}

	// Verify lossless
	errors := 0
	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("12-bit: Total pixel errors: %d / %d", errors, len(pixelData))
	} else {
		t.Logf("12-bit: Perfect lossless reconstruction!")
	}
}

// TestInvalidParameters tests error handling for invalid parameters
func TestInvalidParameters(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		wantError  bool
	}{
		{"Invalid width", 0, 64, 1, 8, true},
		{"Invalid height", 64, 0, 1, 8, true},
		{"Invalid components", 64, 64, 2, 8, true},
		{"Invalid bit depth low", 64, 64, 1, 1, true},
		{"Invalid bit depth high", 64, 64, 1, 17, true},
		{"Valid 8-bit", 64, 64, 1, 8, false},
		{"Valid 16-bit", 64, 64, 1, 16, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height * tt.components
			if tt.bitDepth > 8 {
				size *= 2
			}
			pixelData := make([]byte, size)

			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth)
			if (err != nil) != tt.wantError {
				t.Errorf("Encode() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestFlatRegion tests encoding of flat regions (all same value)
func TestFlatRegion(t *testing.T) {
	width, height := 32, 32
	bitDepth := 8
	components := 1

	// Create flat image (all pixels = 128)
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = 128
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Flat region - Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	// Flat regions should compress very well
	compressionRatio := float64(len(pixelData)) / float64(len(encoded))
	if compressionRatio < 2.0 {
		t.Logf("Warning: Flat region compression ratio is low: %.2fx (expected > 2.0)", compressionRatio)
	}

	// Decode and verify
	decoded, _, _, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			t.Errorf("Flat region: pixel %d mismatch", i)
			break
		}
	}
}

// TestGradientImage tests encoding of gradient images
func TestGradientImage(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1

	// Create smooth gradient
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			// Smooth gradient
			pixelData[idx] = byte(((x * 256 / width) + (y * 256 / height)) / 2)
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Gradient - Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	// Decode and verify
	decoded, _, _, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	errors := 0
	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Gradient: %d pixel errors", errors)
	} else {
		t.Logf("Gradient: Perfect lossless reconstruction!")
	}
}
