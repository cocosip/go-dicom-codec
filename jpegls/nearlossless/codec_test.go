package nearlossless

import (
	"strings"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

// TestCodecInterface verifies that Codec implements codec.Codec
func TestCodecInterface(_ *testing.T) {
	var _ codec.Codec = (*JPEGLSNearLosslessCodec)(nil)
}

// TestCodecRegistration tests that the codec is registered in the global registry
func TestCodecRegistration(t *testing.T) {
	// Ensure codec is registered
	RegisterJPEGLSNearLosslessCodec(2)

	// Retrieve from global registry
	registry := codec.GetGlobalRegistry()
	c, exists := registry.GetCodec(transfer.JPEGLSNearLossless)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	// Basic checks
	if !strings.HasPrefix(c.Name(), "JPEG-LS Near-Lossless") {
		t.Errorf("Unexpected codec name: %s", c.Name())
	}
	if c.TransferSyntax().UID().UID() != transfer.JPEGLSNearLossless.UID().UID() {
		t.Errorf("Unexpected UID: %s", c.TransferSyntax().UID().UID())
	}
}

// TestCodecUID tests the UID method
func TestCodecUID(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)
	expectedUID := transfer.JPEGLSNearLossless.UID().UID()
	if c.TransferSyntax().UID().UID() != expectedUID {
		t.Errorf("UID() = %s, want %s", c.TransferSyntax().UID().UID(), expectedUID)
	}
}

// TestCodecName tests the Name method
func TestCodecName(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)
	if !strings.HasPrefix(c.Name(), "JPEG-LS Near-Lossless") {
		t.Errorf("Name() unexpected: %s", c.Name())
	}
}

// TestOptionsValidate tests the Options.Validate method
// Validate NEAR parameter handling via codec.Parameters
func TestParameterNearValues(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	cases := []struct {
		name    string
		near    int
		wantErr bool
	}{
		{"NEAR=0", 0, false},
		{"NEAR=3", 3, false},
		{"NEAR=255", 255, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := codec.NewBaseParameters()
			params.SetParameter("near", tc.near)
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Encode(src, encoded, params)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Encode error=%v, wantErr=%v", err, tc.wantErr)
			}
			frame, frameErr := encoded.GetFrame(0)
			if !tc.wantErr && (frameErr != nil || len(frame) == 0) {
				t.Error("encoded data is empty")
			}
		})
	}
}

// TestCodecEncode tests encoding through the Codec interface
func TestCodecEncode(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Base frameInfo
	baseFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	tests := []struct {
		name    string
		mutate  func(*imagetypes.FrameInfo) (*imagetypes.FrameInfo, codec.Parameters)
		wantErr bool
	}{
		{
			name: "Valid NEAR=0 (lossless)",
			mutate: func(fi *imagetypes.FrameInfo) (*imagetypes.FrameInfo, codec.Parameters) {
				p := codec.NewBaseParameters()
				p.SetParameter("near", 0)
				return fi, p
			},
			wantErr: false,
		},
		{
			name: "Valid NEAR=3",
			mutate: func(fi *imagetypes.FrameInfo) (*imagetypes.FrameInfo, codec.Parameters) {
				p := codec.NewBaseParameters()
				p.SetParameter("near", 3)
				return fi, p
			},
			wantErr: false,
		},
		{
			name: "Invalid width",
			mutate: func(fi *imagetypes.FrameInfo) (*imagetypes.FrameInfo, codec.Parameters) {
				newFi := *fi
				newFi.Width = 0
				return &newFi, nil
			},
			wantErr: true,
		},
		{
			name: "Invalid components",
			mutate: func(fi *imagetypes.FrameInfo) (*imagetypes.FrameInfo, codec.Parameters) {
				newFi := *fi
				newFi.SamplesPerPixel = 2
				return &newFi, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// copy frameInfo to avoid mutation leakage
			frameInfo, params := tt.mutate(baseFrameInfo)
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Encode(src, encoded, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			frame, frameErr := encoded.GetFrame(0)
			if !tt.wantErr && (frameErr != nil || len(frame) == 0) {
				t.Error("Encode() returned empty data")
			}
		})
	}
}

// TestCodecDecode tests decoding through the Codec interface
func TestCodecDecode(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Create source PixelData
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	params := codec.NewBaseParameters()
	params.SetParameter("near", 3)

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(src, encoded, params); err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() failed: %v", err)
	}

	decodedInfo := decoded.GetFrameInfo()
	if int(decodedInfo.Width) != width || int(decodedInfo.Height) != height {
		t.Errorf("Decoded dimensions mismatch")
	}
	if decodedInfo.SamplesPerPixel != 1 || decodedInfo.BitsStored != 8 {
		t.Errorf("Decoded metadata mismatch")
	}

	decodedFrame, err := decoded.GetFrame(0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	// Verify error bound (NEAR=3)
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(decodedFrame[i]) - int(pixelData[i])
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
}

// TestCodecDecodeInvalid tests decoding with invalid input
func TestCodecDecodeInvalid(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	tests := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid data", []byte{0x00, 0x01, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameInfo := &imagetypes.FrameInfo{
				Width:                     32,
				Height:                    32,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: "MONOCHROME2",
			}
			srcEnc := codecHelpers.NewTestPixelData(frameInfo)
			if err := srcEnc.AddFrame(tt.data); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			dst := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Decode(srcEnc, dst, nil)
			if err == nil {
				t.Error("Decode() expected error, got nil")
			}
		})
	}
}

// TestCodecRoundTrip tests encoding and decoding round-trip
func TestCodecRoundTrip(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

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
			size := tt.width * tt.height * tt.components
			pixelData := make([]byte, size)
			for i := range pixelData {
				pixelData[i] = byte((i * 7) % 256)
			}

			// Create source PixelData
			frameInfo := &imagetypes.FrameInfo{
				Width:                     uint16(tt.width),
				Height:                    uint16(tt.height),
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           uint16(tt.components),
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: map[int]string{1: "MONOCHROME2", 3: "RGB"}[tt.components],
			}
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := codec.NewBaseParameters()
			params.SetParameter("near", tt.near)

			encoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Encode(src, encoded, params); err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Decode(encoded, decoded, nil); err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			decodedFrame, err := decoded.GetFrame(0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}
			maxError := 0
			for i := 0; i < len(pixelData); i++ {
				diff := int(decodedFrame[i]) - int(pixelData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				if diff > tt.near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, tt.near)
					break
				}
			}

			if tt.near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}
