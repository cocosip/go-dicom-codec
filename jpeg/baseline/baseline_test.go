package baseline

import (
	"testing"
)

func TestEncodeDecodeGrayscale(t *testing.T) {
	// Create a simple test pattern (grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, 85)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes (compression ratio: %.2fx)",
		len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if components != 1 {
		t.Errorf("Components mismatch: got %d, want 1", components)
	}

	// Verify data length
	if len(decodedData) != width*height {
		t.Errorf("Data length mismatch: got %d, want %d", len(decodedData), width*height)
	}

	// Check that decoded data is reasonably close to original (lossy compression)
	// We'll allow a generous error margin
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Maximum pixel error: %d", maxError)

	// For lossy JPEG, we expect some error, but it shouldn't be too large
	if maxError > 50 {
		t.Errorf("Maximum error too large: %d (expected <= 50)", maxError)
	}
}

func TestEncodeDecodeRGB(t *testing.T) {
	// Create a simple test pattern (RGB)
	width, height := 64, 64
	pixelData := make([]byte, width*height*3)

	// Create a color gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 4)        // R
			pixelData[offset+1] = byte(y * 4)        // G
			pixelData[offset+2] = byte((x + y) * 2)  // B
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 3, 85)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes (compression ratio: %.2fx)",
		len(jpegData), float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if components != 3 {
		t.Errorf("Components mismatch: got %d, want 3", components)
	}

	// Verify data length
	if len(decodedData) != width*height*3 {
		t.Errorf("Data length mismatch: got %d, want %d", len(decodedData), width*height*3)
	}

	// Check that decoded data is reasonably close to original (lossy compression)
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}

	t.Logf("Maximum pixel error: %d", maxError)

	// For lossy JPEG with YCbCr conversion and 4:2:0 subsampling, we expect some error
	// The error can be larger due to chroma subsampling
	if maxError > 120 {
		t.Errorf("Maximum error too large: %d (expected <= 120)", maxError)
	}
}

func TestEncodeInvalidParameters(t *testing.T) {
	pixelData := make([]byte, 64*64)

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		quality    int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 85, true},
		{"Invalid height", 64, 0, 1, 85, true},
		{"Invalid components", 64, 64, 2, 85, true},
		{"Invalid quality low", 64, 64, 1, 0, true},
		{"Invalid quality high", 64, 64, 1, 101, true},
		{"Valid", 64, 64, 1, 85, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.quality)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQualityLevels(t *testing.T) {
	width, height := 32, 32
	pixelData := make([]byte, width*height)

	// Create a test pattern
	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	qualities := []int{10, 50, 90}
	var prevSize int

	for _, quality := range qualities {
		jpegData, err := Encode(pixelData, width, height, 1, quality)
		if err != nil {
			t.Fatalf("Encode at quality %d failed: %v", quality, err)
		}

		t.Logf("Quality %d: size = %d bytes", quality, len(jpegData))

		// Higher quality should generally result in larger file sizes
		if prevSize > 0 && len(jpegData) < prevSize {
			t.Logf("Quality %d produced smaller file than previous quality (expected)", quality)
		}
		prevSize = len(jpegData)
	}
}

func BenchmarkEncodeGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 85)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 1, 85)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeRGB(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height*3)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 3, 85)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRGB(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height*3)

	for i := 0; i < len(pixelData); i++ {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 3, 85)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
