package nearlossless

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/codec"
)

// TestCodecInterface verifies that Codec implements codec.Codec
func TestCodecInterface(t *testing.T) {
	var _ codec.Codec = (*Codec)(nil)
}

// TestCodecRegistration tests that the codec is registered in the global registry
func TestCodecRegistration(t *testing.T) {
	// Test retrieval by UID
	c, err := codec.Get("1.2.840.10008.1.2.4.81")
	if err != nil {
		t.Fatalf("Get() by UID failed: %v", err)
	}
	if c == nil {
		t.Fatal("Get() by UID returned nil codec")
	}
	if c.UID() != "1.2.840.10008.1.2.4.81" {
		t.Errorf("Get() by UID returned wrong codec: UID=%s", c.UID())
	}

	// Test retrieval by name
	c2, err := codec.Get("JPEG-LS Near-Lossless")
	if err != nil {
		t.Fatalf("Get() by name failed: %v", err)
	}
	if c2 == nil {
		t.Fatal("Get() by name returned nil codec")
	}
	if c2.Name() != "JPEG-LS Near-Lossless" {
		t.Errorf("Get() by name returned wrong codec: Name=%s", c2.Name())
	}
}

// TestCodecUID tests the UID method
func TestCodecUID(t *testing.T) {
	c := NewCodec()
	expectedUID := "1.2.840.10008.1.2.4.81"
	if c.UID() != expectedUID {
		t.Errorf("UID() = %s, want %s", c.UID(), expectedUID)
	}
}

// TestCodecName tests the Name method
func TestCodecName(t *testing.T) {
	c := NewCodec()
	expectedName := "JPEG-LS Near-Lossless"
	if c.Name() != expectedName {
		t.Errorf("Name() = %s, want %s", c.Name(), expectedName)
	}
}

// TestOptionsValidate tests the Options.Validate method
func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Options
		wantErr bool
	}{
		{"Valid NEAR=0", &Options{NEAR: 0}, false},
		{"Valid NEAR=3", &Options{NEAR: 3}, false},
		{"Valid NEAR=255", &Options{NEAR: 255}, false},
		{"Invalid NEAR=-1", &Options{NEAR: -1}, true},
		{"Invalid NEAR=256", &Options{NEAR: 256}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCodecEncode tests encoding through the Codec interface
func TestCodecEncode(t *testing.T) {
	c := NewCodec()

	// Create test image
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	tests := []struct {
		name    string
		params  codec.EncodeParams
		wantErr bool
	}{
		{
			name: "Valid NEAR=0 (lossless)",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      width,
				Height:     height,
				Components: 1,
				BitDepth:   8,
				Options:    &codec.BaseOptions{NearLossless: 0},
			},
			wantErr: false,
		},
		{
			name: "Valid NEAR=3",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      width,
				Height:     height,
				Components: 1,
				BitDepth:   8,
				Options:    &codec.BaseOptions{NearLossless: 3},
			},
			wantErr: false,
		},
		{
			name: "Valid with custom Options",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      width,
				Height:     height,
				Components: 1,
				BitDepth:   8,
				Options:    &Options{NEAR: 5},
			},
			wantErr: false,
		},
		{
			name: "Valid without options (defaults to 0)",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      width,
				Height:     height,
				Components: 1,
				BitDepth:   8,
			},
			wantErr: false,
		},
		{
			name: "Invalid width",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      0,
				Height:     height,
				Components: 1,
				BitDepth:   8,
			},
			wantErr: true,
		},
		{
			name: "Invalid components",
			params: codec.EncodeParams{
				PixelData:  pixelData,
				Width:      width,
				Height:     height,
				Components: 2,
				BitDepth:   8,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := c.Encode(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(encoded) == 0 {
				t.Error("Encode() returned empty data")
			}
		})
	}
}

// TestCodecDecode tests decoding through the Codec interface
func TestCodecDecode(t *testing.T) {
	c := NewCodec()

	// Create test image
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Keep a copy of original data for comparison
	originalPixelData := make([]byte, len(pixelData))
	copy(originalPixelData, pixelData)

	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
		Options:    &codec.BaseOptions{NearLossless: 3},
	}

	encoded, err := c.Encode(params)
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	// Test decode
	result, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() failed: %v", err)
	}

	// Verify dimensions
	if result.Width != width {
		t.Errorf("Decode() width = %d, want %d", result.Width, width)
	}
	if result.Height != height {
		t.Errorf("Decode() height = %d, want %d", result.Height, height)
	}
	if result.Components != 1 {
		t.Errorf("Decode() components = %d, want 1", result.Components)
	}
	if result.BitDepth != 8 {
		t.Errorf("Decode() bitDepth = %d, want 8", result.BitDepth)
	}

	// Verify error bound (NEAR=3) - compare against original data
	maxError := 0
	for i := range originalPixelData {
		diff := int(result.PixelData[i]) - int(originalPixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}
	if maxError > 3 {
		t.Errorf("Decode() max error = %d, want <= 3", maxError)
	}

	t.Logf("Decode() max error = %d (NEAR=3)", maxError)
}

// TestCodecDecodeInvalid tests decoding with invalid input
func TestCodecDecodeInvalid(t *testing.T) {
	c := NewCodec()

	tests := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid data", []byte{0x00, 0x01, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Decode(tt.data)
			if err == nil {
				t.Error("Decode() expected error, got nil")
			}
		})
	}
}

// TestCodecRoundTrip tests encoding and decoding round-trip
func TestCodecRoundTrip(t *testing.T) {
	c := NewCodec()

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		near       int
	}{
		{"Grayscale NEAR=0", 32, 32, 1, 0},
		{"Grayscale NEAR=1", 32, 32, 1, 1},
		{"Grayscale NEAR=3", 32, 32, 1, 3},
		{"Grayscale NEAR=5", 32, 32, 1, 5},
		{"RGB NEAR=3", 16, 16, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test image
			size := tt.width * tt.height * tt.components
			pixelData := make([]byte, size)
			for i := range pixelData {
				pixelData[i] = byte((i * 7) % 256)
			}

			// Encode
			params := codec.EncodeParams{
				PixelData:  pixelData,
				Width:      tt.width,
				Height:     tt.height,
				Components: tt.components,
				BitDepth:   8,
				Options:    &codec.BaseOptions{NearLossless: tt.near},
			}

			encoded, err := c.Encode(params)
			if err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			// Decode
			result, err := c.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			// Verify error bound
			maxError := 0
			for i := range pixelData {
				diff := int(result.PixelData[i]) - int(pixelData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				if diff > tt.near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, tt.near)
				}
			}

			t.Logf("Max error=%d (NEAR=%d)", maxError, tt.near)

			// For lossless (NEAR=0), verify perfect reconstruction
			if tt.near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}

