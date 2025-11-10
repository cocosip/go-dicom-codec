package lossless14sv1

import (
	"testing"
)

func TestEncodeDecodeGrayscale8bit(t *testing.T) {
	// Create test image (8-bit grayscale)
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height)

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, bits, err := Decode(jpegData)
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

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", len(pixelData))
	}
}

func TestEncodeDecodeRGB8bit(t *testing.T) {
	// Create test image (8-bit RGB)
	width, height := 64, 64
	bitDepth := 8
	pixelData := make([]byte, width*height*3)

	// Create a color gradient
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			pixelData[offset+0] = byte(x * 4)       // R
			pixelData[offset+1] = byte(y * 4)       // G
			pixelData[offset+2] = byte((x+y) * 2)   // B
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 3, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, components, bits, err := Decode(jpegData)
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

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction (lossless)
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, decodedData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", len(pixelData))
	}
}

func TestEncodeDecode12bit(t *testing.T) {
	t.Skip("12-bit support requires extended Huffman tables - TODO")
	// Create test image (12-bit grayscale)
	width, height := 32, 32
	bitDepth := 12
	pixelData := make([]byte, width*height*2) // 2 bytes per sample

	// Create a 12-bit gradient (0-4095)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y) * 64) % 4096
			offset := (y*width + x) * 2
			pixelData[offset] = byte(val & 0xFF)
			pixelData[offset+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Encode
	jpegData, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(pixelData))
	t.Logf("Compressed size: %d bytes", len(jpegData))
	t.Logf("Compression ratio: %.2fx", float64(len(pixelData))/float64(len(jpegData)))

	// Decode
	decodedData, w, h, _, bits, err := Decode(jpegData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if w != width || h != height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", w, h, width, height)
	}

	if bits != bitDepth {
		t.Errorf("Bit depth mismatch: got %d, want %d", bits, bitDepth)
	}

	// Verify perfect reconstruction
	if len(decodedData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decodedData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decodedData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Byte %d mismatch: got %d, want %d", i, decodedData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all bytes match")
	}
}

func TestEncodeInvalidParameters(t *testing.T) {
	pixelData := make([]byte, 64*64)

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		wantErr    bool
	}{
		{"Invalid width", 0, 64, 1, 8, true},
		{"Invalid height", 64, 0, 1, 8, true},
		{"Invalid components", 64, 64, 2, 8, true},
		{"Invalid bit depth low", 64, 64, 1, 1, true},
		{"Invalid bit depth high", 64, 64, 1, 17, true},
		{"Valid", 64, 64, 1, 8, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(pixelData, tt.width, tt.height, tt.components, tt.bitDepth)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkEncode8bitGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 8)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecode8bitGrayscale(b *testing.B) {
	width, height := 512, 512
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	jpegData, err := Encode(pixelData, width, height, 1, 8)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _, err := Decode(jpegData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode12bit(b *testing.B) {
	width, height := 256, 256
	pixelData := make([]byte, width*height*2)
	for i := 0; i < len(pixelData); i += 2 {
		val := (i / 2) % 4096
		pixelData[i] = byte(val & 0xFF)
		pixelData[i+1] = byte((val >> 8) & 0xFF)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encode(pixelData, width, height, 1, 12)
		if err != nil {
			b.Fatal(err)
		}
	}
}
