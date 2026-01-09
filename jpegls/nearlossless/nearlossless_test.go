package nearlossless

import (
	"testing"

	jpegcommon "github.com/cocosip/go-dicom-codec/jpegls/common"
)

// TestEncodeDecodeNEAR0 tests with NEAR=0 (should be lossless)
func TestEncodeDecodeNEAR0(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1
	near := 0 // Lossless

	// Create test image
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			pixelData[idx] = byte((x + y*2) % 256)
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("NEAR=0 Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, n, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height || c != components || bd != bitDepth || n != near {
		t.Errorf("Dimension mismatch")
	}

	// Verify lossless (NEAR=0)
	errors := 0
	maxError := 0
	for i := range pixelData {
		diff := jpegcommon.Abs(int(decoded[i]) - int(pixelData[i]))
		if diff > 0 {
			errors++
			if diff > maxError {
				maxError = diff
			}
		}
	}

	if errors > 0 {
		t.Errorf("NEAR=0: Expected lossless, but got %d errors, max error=%d", errors, maxError)
	} else {
		t.Logf("NEAR=0: Perfect lossless! (0 errors)")
	}
}

// TestEncodeDecodeNEAR3 tests with NEAR=3
func TestEncodeDecodeNEAR3(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1
	near := 3

	// Create test image
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			pixelData[idx] = byte((x + y*2) % 256)
		}
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, components, bitDepth, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("NEAR=3 Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	// Decode
	decoded, w, h, c, bd, n, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height || c != components || bd != bitDepth || n != near {
		t.Errorf("Dimension mismatch")
	}

	// Verify error bound
	errors := 0
	maxError := 0
	totalError := 0
	for i := range pixelData {
		diff := jpegcommon.Abs(int(decoded[i]) - int(pixelData[i]))
		if diff > 0 {
			errors++
			totalError += diff
			if diff > maxError {
				maxError = diff
			}
		}

		if diff > near {
			t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, near)
		}
	}

	avgError := 0.0
	if errors > 0 {
		avgError = float64(totalError) / float64(errors)
	}

	t.Logf("NEAR=3: Max error=%d (limit=%d), Avg error=%.2f, %d pixels with errors",
		maxError, near, avgError, errors)

	if maxError > near {
		t.Errorf("NEAR=3: Max error %d exceeds limit %d", maxError, near)
	}
}

// TestEncodeDecodeNEAR7 tests with NEAR=7
func TestEncodeDecodeNEAR7(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1
	near := 7

	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			pixelData[idx] = byte((x*3 + y*5) % 256)
		}
	}

	encoded, err := Encode(pixelData, width, height, components, bitDepth, near)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("NEAR=7 Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	decoded, _, _, _, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify error bound
	maxError := 0
	for i := range pixelData {
		diff := jpegcommon.Abs(int(decoded[i]) - int(pixelData[i]))
		if diff > maxError {
			maxError = diff
		}
		if diff > near {
			t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, near)
		}
	}

	t.Logf("NEAR=7: Max error=%d (limit=%d)", maxError, near)
}

// TestEncodeDecodeRGB tests RGB with NEAR=3
func TestEncodeDecodeRGB(t *testing.T) {
	width, height := 32, 32
	bitDepth := 8
	components := 3
	near := 3

	pixelData := make([]byte, width*height*components)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * components
			pixelData[idx+0] = byte((x * 8) % 256)
			pixelData[idx+1] = byte((y * 8) % 256)
			pixelData[idx+2] = byte(((x + y) * 4) % 256)
		}
	}

	encoded, err := Encode(pixelData, width, height, components, bitDepth, near)
	if err != nil {
		t.Fatalf("RGB Encode failed: %v", err)
	}

	t.Logf("RGB NEAR=3 Original: %d bytes, Compressed: %d bytes (%.2fx)",
		len(pixelData), len(encoded), float64(len(pixelData))/float64(len(encoded)))

	decoded, _, _, _, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("RGB Decode failed: %v", err)
	}

	// Verify error bound
	maxError := 0
	for i := range pixelData {
		diff := jpegcommon.Abs(int(decoded[i]) - int(pixelData[i]))
		if diff > maxError {
			maxError = diff
		}
		if diff > near {
			t.Errorf("RGB Pixel %d: error=%d exceeds NEAR=%d", i, diff, near)
		}
	}

	t.Logf("RGB NEAR=3: Max error=%d (limit=%d)", maxError, near)
}

// TestCompressionRatioComparison compares compression ratios for different NEAR values
func TestCompressionRatioComparison(t *testing.T) {
	width, height := 64, 64
	bitDepth := 8
	components := 1

	// Create test image
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			pixelData[idx] = byte((x + y*2) % 256)
		}
	}

	nearValues := []int{0, 1, 3, 5, 7, 10}

	t.Logf("Compression ratio comparison:")
	for _, near := range nearValues {
		encoded, err := Encode(pixelData, width, height, components, bitDepth, near)
		if err != nil {
			t.Fatalf("Encode NEAR=%d failed: %v", near, err)
		}

		ratio := float64(len(pixelData)) / float64(len(encoded))
		t.Logf("  NEAR=%2d: %d bytes (%.2fx compression)", near, len(encoded), ratio)
	}
}

// TestInvalidParameters tests error handling
func TestInvalidParameters(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		near       int
		wantError  bool
	}{
		{"Invalid width", 0, 64, 1, 8, 3, true},
		{"Invalid height", 64, 0, 1, 8, 3, true},
		{"Invalid components", 64, 64, 2, 8, 3, true},
		{"Invalid bit depth low", 64, 64, 1, 1, 3, true},
		{"Invalid bit depth high", 64, 64, 1, 17, 3, true},
		{"Invalid NEAR negative", 64, 64, 1, 8, -1, true},
		{"Invalid NEAR too high", 64, 64, 1, 8, 256, true},
		{"Valid NEAR=0", 64, 64, 1, 8, 0, false},
		{"Valid NEAR=7", 64, 64, 1, 8, 7, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height * tt.components
			if tt.bitDepth > 8 {
				size *= 2
			}
			pixelData := make([]byte, size)

			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth, tt.near)
			if (err != nil) != tt.wantError {
				t.Errorf("Encode() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestEncodeDecodeSigned16BitNear0(t *testing.T) {
	width, height := 4, 4
	bitDepth := 16
	components := 1
	near := 0
	values := []int16{-32768, -1024, -1, 0, 1, 42, 4096, 16000}

	pixelData := make([]byte, width*height*components*2)
	for i := 0; i < width*height*components; i++ {
		v := values[i%len(values)]
		u := uint16(v)
		pixelData[2*i] = byte(u)
		pixelData[2*i+1] = byte(u >> 8)
	}

	encoded, err := EncodeWithPixelRep(pixelData, width, height, components, bitDepth, near, true)
	if err != nil {
		t.Fatalf("EncodeWithPixelRep failed: %v", err)
	}

	decoded, w, h, c, bd, n, err := DecodeWithPixelRep(encoded, true)
	if err != nil {
		t.Fatalf("DecodeWithPixelRep failed: %v", err)
	}
	if w != width || h != height || c != components || bd != bitDepth || n != near {
		t.Fatalf("decoded info mismatch: %dx%d c=%d bd=%d near=%d", w, h, c, bd, n)
	}

	if len(decoded) != len(pixelData) {
		t.Fatalf("length mismatch: got %d want %d", len(decoded), len(pixelData))
	}

	for i := range pixelData {
		if decoded[i] != pixelData[i] {
			t.Fatalf("pixel mismatch at %d: got %d want %d", i, decoded[i], pixelData[i])
		}
	}
}
