package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
)

// TestCodecInterface verifies the codec implements the interface
func TestCodecInterface(t *testing.T) {
	var _ codec.Codec = (*Codec)(nil)
}

// TestCodecCreation tests codec creation
func TestCodecCreation(t *testing.T) {
	c := NewCodec()
	if c == nil {
		t.Fatal("NewCodec returned nil")
	}
}

// TestCodecName tests the codec name
func TestCodecName(t *testing.T) {
	c := NewCodec()
	name := c.Name()

	expected := "JPEG 2000 Lossless"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

// TestCodecTransferSyntax tests the transfer syntax
func TestCodecTransferSyntax(t *testing.T) {
	c := NewCodec()
	ts := c.TransferSyntax()

	if ts == nil {
		t.Fatal("TransferSyntax() returned nil")
	}

	// The UID should be 1.2.840.10008.1.2.4.90
	uid := ts.UID().UID()
	expected := "1.2.840.10008.1.2.4.90"
	if uid != expected {
		t.Errorf("Transfer Syntax UID = %s, want %s", uid, expected)
	}
}

// TestDecodeNilInputs tests decode with nil inputs
func TestDecodeNilInputs(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}

	tests := []struct {
		name string
		src  imagetypes.PixelData
		dst  imagetypes.PixelData
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, codecHelpers.NewTestPixelData(frameInfo)},
		{"Dst nil", codecHelpers.NewTestPixelData(frameInfo), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Decode(tt.src, tt.dst, nil)
			if err == nil {
				t.Error("Expected error for nil input, got nil")
			}
		})
	}
}

// TestDecodeEmptyData tests decode with empty data
func TestDecodeEmptyData(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Decode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

// TestDecodeInvalidData tests decode with invalid JPEG 2000 data
func TestDecodeInvalidData(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	src.AddFrame([]byte{0x00, 0x01, 0x02, 0x03}) // Invalid JPEG 2000 data
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Decode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for invalid data, got nil")
	}
}

// TestEncodeNotImplemented tests that encoding returns error
func TestEncodeNotImplemented(t *testing.T) {
	c := NewCodec()

	// Create simple test data
	pixelData := make([]byte, 64*64)
	// Fill with simple pattern
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	src.AddFrame(pixelData)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Encode(src, dst, nil)
	if err != nil {
		t.Errorf("Encoding failed: %v", err)
	}

	// Verify output
	dstData, _ := dst.GetFrame(0)
	if len(dstData) == 0 {
		t.Error("Encoded data is empty")
	}
	if dst.GetFrameInfo().Width != src.GetFrameInfo().Width {
		t.Errorf("Width mismatch: got %d, want %d", dst.GetFrameInfo().Width, src.GetFrameInfo().Width)
	}
	if dst.GetFrameInfo().Height != src.GetFrameInfo().Height {
		t.Errorf("Height mismatch: got %d, want %d", dst.GetFrameInfo().Height, src.GetFrameInfo().Height)
	}
}

// TestEncodeNilInputs tests encode with nil inputs
func TestEncodeNilInputs(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	dstPixel := codecHelpers.NewTestPixelData(frameInfo)
	dstPixel.AddFrame([]byte{1})

	tests := []struct {
		name string
		src  imagetypes.PixelData
		dst  imagetypes.PixelData
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, codecHelpers.NewTestPixelData(frameInfo)},
		{"Dst nil", dstPixel, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Encode(tt.src, tt.dst, nil)
			if err == nil {
				t.Error("Expected error for nil input, got nil")
			}
		})
	}
}

// TestEncodeEmptyData tests encode with empty data
func TestEncodeEmptyData(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	err := c.Encode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}
