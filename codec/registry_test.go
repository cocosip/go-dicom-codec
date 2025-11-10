package codec_test

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/codec"
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
)

func TestCodecRegistry(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantFound bool
		wantUID   string
		wantName  string
	}{
		{
			name:      "Get baseline by UID",
			key:       "1.2.840.10008.1.2.4.50",
			wantFound: true,
			wantUID:   "1.2.840.10008.1.2.4.50",
			wantName:  "jpeg-baseline",
		},
		{
			name:      "Get baseline by name",
			key:       "jpeg-baseline",
			wantFound: true,
			wantUID:   "1.2.840.10008.1.2.4.50",
			wantName:  "jpeg-baseline",
		},
		{
			name:      "Get lossless SV1 by UID",
			key:       "1.2.840.10008.1.2.4.70",
			wantFound: true,
			wantUID:   "1.2.840.10008.1.2.4.70",
			wantName:  "jpeg-lossless-sv1",
		},
		{
			name:      "Get lossless SV1 by name",
			key:       "jpeg-lossless-sv1",
			wantFound: true,
			wantUID:   "1.2.840.10008.1.2.4.70",
			wantName:  "jpeg-lossless-sv1",
		},
		{
			name:      "Get non-existent codec",
			key:       "non-existent",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := codec.Get(tt.key)

			if tt.wantFound {
				if err != nil {
					t.Errorf("Get(%q) unexpected error: %v", tt.key, err)
					return
				}
				if c == nil {
					t.Errorf("Get(%q) returned nil codec", tt.key)
					return
				}
				if c.UID() != tt.wantUID {
					t.Errorf("Get(%q).UID() = %q, want %q", tt.key, c.UID(), tt.wantUID)
				}
				if c.Name() != tt.wantName {
					t.Errorf("Get(%q).Name() = %q, want %q", tt.key, c.Name(), tt.wantName)
				}
			} else {
				if err == nil {
					t.Errorf("Get(%q) expected error, got nil", tt.key)
				}
				if err != codec.ErrCodecNotFound {
					t.Errorf("Get(%q) error = %v, want %v", tt.key, err, codec.ErrCodecNotFound)
				}
			}
		})
	}
}

func TestListCodecs(t *testing.T) {
	codecs := codec.List()

	if len(codecs) < 2 {
		t.Errorf("List() returned %d codecs, want at least 2", len(codecs))
	}

	// Verify we have both codecs
	foundBaseline := false
	foundLossless := false

	for _, c := range codecs {
		switch c.UID() {
		case "1.2.840.10008.1.2.4.50":
			foundBaseline = true
			if c.Name() != "jpeg-baseline" {
				t.Errorf("Baseline codec name = %q, want %q", c.Name(), "jpeg-baseline")
			}
		case "1.2.840.10008.1.2.4.70":
			foundLossless = true
			if c.Name() != "jpeg-lossless-sv1" {
				t.Errorf("Lossless SV1 codec name = %q, want %q", c.Name(), "jpeg-lossless-sv1")
			}
		}
	}

	if !foundBaseline {
		t.Error("List() did not include JPEG Baseline codec")
	}
	if !foundLossless {
		t.Error("List() did not include JPEG Lossless SV1 codec")
	}
}

func TestBaselineCodecEncodeDecode(t *testing.T) {
	// Get codec by UID
	c, err := codec.Get("1.2.840.10008.1.2.4.50")
	if err != nil {
		t.Fatalf("Failed to get baseline codec: %v", err)
	}

	// Create test image (64x64 grayscale)
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Encode
	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
		Options:    nil, // Use default quality
	}

	compressed, err := c.Encode(params)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Compressed size: %d bytes", len(compressed))

	// Decode
	result, err := c.Decode(compressed)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if result.Width != width {
		t.Errorf("Width = %d, want %d", result.Width, width)
	}
	if result.Height != height {
		t.Errorf("Height = %d, want %d", result.Height, height)
	}
	if result.Components != 1 {
		t.Errorf("Components = %d, want 1", result.Components)
	}
	if result.BitDepth != 8 {
		t.Errorf("BitDepth = %d, want 8", result.BitDepth)
	}
}

func TestLosslessSV1CodecEncodeDecode(t *testing.T) {
	// Get codec by name
	c, err := codec.Get("jpeg-lossless-sv1")
	if err != nil {
		t.Fatalf("Failed to get lossless SV1 codec: %v", err)
	}

	// Create test image (32x32 grayscale)
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Encode
	params := codec.EncodeParams{
		PixelData:  pixelData,
		Width:      width,
		Height:     height,
		Components: 1,
		BitDepth:   8,
		Options:    nil,
	}

	compressed, err := c.Encode(params)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Compressed size: %d bytes", len(compressed))

	// Decode
	result, err := c.Decode(compressed)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if result.Width != width {
		t.Errorf("Width = %d, want %d", result.Width, width)
	}
	if result.Height != height {
		t.Errorf("Height = %d, want %d", result.Height, height)
	}
	if result.Components != 1 {
		t.Errorf("Components = %d, want 1", result.Components)
	}
	if result.BitDepth != 8 {
		t.Errorf("BitDepth = %d, want 8", result.BitDepth)
	}

	// Verify perfect reconstruction (lossless)
	if len(result.PixelData) != len(pixelData) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(result.PixelData), len(pixelData))
	}

	errors := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != result.PixelData[i] {
			errors++
			if errors <= 5 {
				t.Errorf("Pixel %d mismatch: got %d, want %d", i, result.PixelData[i], pixelData[i])
			}
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", len(pixelData))
	}
}
