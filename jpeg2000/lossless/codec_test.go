package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/codec"
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

	tests := []struct {
		name string
		src  *codec.PixelData
		dst  *codec.PixelData
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, &codec.PixelData{}},
		{"Dst nil", &codec.PixelData{}, nil},
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

	src := &codec.PixelData{Data: []byte{}}
	dst := &codec.PixelData{}

	err := c.Decode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

// TestDecodeInvalidData tests decode with invalid JPEG 2000 data
func TestDecodeInvalidData(t *testing.T) {
	c := NewCodec()

	src := &codec.PixelData{
		Data: []byte{0x00, 0x01, 0x02, 0x03}, // Invalid JPEG 2000 data
	}
	dst := &codec.PixelData{}

	err := c.Decode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for invalid data, got nil")
	}
}

// TestEncodeNotImplemented tests that encoding returns error
func TestEncodeNotImplemented(t *testing.T) {
	c := NewCodec()

	// Create simple test data
	src := &codec.PixelData{
		Data:            make([]byte, 64*64),
		Width:           64,
		Height:          64,
		SamplesPerPixel: 1,
		BitsStored:      8,
		BitsAllocated:   8,
		HighBit:         7,
	}
	// Fill with simple pattern
	for i := range src.Data {
		src.Data[i] = byte(i % 256)
	}
	dst := &codec.PixelData{}

	err := c.Encode(src, dst, nil)
	if err != nil {
		t.Errorf("Encoding failed: %v", err)
	}

	// Verify output
	if len(dst.Data) == 0 {
		t.Error("Encoded data is empty")
	}
	if dst.Width != src.Width {
		t.Errorf("Width mismatch: got %d, want %d", dst.Width, src.Width)
	}
	if dst.Height != src.Height {
		t.Errorf("Height mismatch: got %d, want %d", dst.Height, src.Height)
	}
}

// TestEncodeNilInputs tests encode with nil inputs
func TestEncodeNilInputs(t *testing.T) {
	c := NewCodec()

	tests := []struct {
		name string
		src  *codec.PixelData
		dst  *codec.PixelData
	}{
		{"Both nil", nil, nil},
		{"Src nil", nil, &codec.PixelData{}},
		{"Dst nil", &codec.PixelData{Data: []byte{1}}, nil},
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

	src := &codec.PixelData{Data: []byte{}}
	dst := &codec.PixelData{}

	err := c.Encode(src, dst, nil)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}
