package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
)

// TestCodecRegistration verifies the codec is registered in the global registry
func TestCodecRegistration(t *testing.T) {
	registry := codec.GetGlobalRegistry()

	// Get the codec from registry
	retrievedCodec, exists := registry.GetCodec(transfer.JPEG2000Lossless)
	if !exists {
		t.Fatal("JPEG 2000 Lossless codec not found in global registry")
	}

	// Verify it's the correct codec
	if retrievedCodec.Name() != "JPEG 2000 Lossless" {
		t.Errorf("Expected codec name 'JPEG 2000 Lossless', got '%s'", retrievedCodec.Name())
	}

	// Verify transfer syntax
	ts := retrievedCodec.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax is nil")
	}

	expectedUID := "1.2.840.10008.1.2.4.90"
	if ts.UID().UID() != expectedUID {
		t.Errorf("Expected UID %s, got %s", expectedUID, ts.UID().UID())
	}
}

// TestCodecInterfaceCompliance verifies the codec implements all required methods
func TestCodecInterfaceCompliance(t *testing.T) {
	c := NewCodec()

	// Test Name method
	name := c.Name()
	if name == "" {
		t.Error("Name() returned empty string")
	}

	// Test TransferSyntax method
	ts := c.TransferSyntax()
	if ts == nil {
		t.Error("TransferSyntax() returned nil")
	}

	// Test Encode method exists and works
	frameInfo := &imagetypes.FrameInfo{
		Width:           1,
		Height:          1,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	src.AddFrame([]byte{1, 2, 3})
	dst := codecHelpers.NewTestPixelData(frameInfo)
	err := c.Encode(src, dst, nil)
	// Encoding should work now
	if err != nil {
		t.Errorf("Encode failed: %v", err)
	}
	dstData, _ := dst.GetFrame(0)
	if len(dstData) == 0 {
		t.Error("Encoded data is empty")
	}

	// Test Decode method exists
	emptyDst := codecHelpers.NewTestPixelData(frameInfo)
	err = c.Decode(codecHelpers.NewTestPixelData(frameInfo), emptyDst, nil)
	// We expect an error (empty data)
	if err == nil {
		t.Error("Decode should return error for empty data")
	}
}

// TestCodecMetadata verifies codec metadata is correct
func TestCodecMetadata(t *testing.T) {
	c := NewCodec()

	tests := []struct {
		name     string
		getValue func() interface{}
		expected interface{}
	}{
		{
			name:     "Codec Name",
			getValue: func() interface{} { return c.Name() },
			expected: "JPEG 2000 Lossless",
		},
		{
			name:     "Transfer Syntax UID",
			getValue: func() interface{} { return c.TransferSyntax().UID().UID() },
			expected: "1.2.840.10008.1.2.4.90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.getValue()
			if got != tt.expected {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// TestDecodeErrorHandling tests various error conditions for decode
func TestDecodeErrorHandling(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}

	srcWithData := codecHelpers.NewTestPixelData(frameInfo)
	srcWithData.AddFrame([]byte{1})

	srcWithInvalidData := codecHelpers.NewTestPixelData(frameInfo)
	srcWithInvalidData.AddFrame([]byte{0x00, 0x01, 0x02})

	tests := []struct {
		name        string
		src         imagetypes.PixelData
		dst         imagetypes.PixelData
		expectError bool
		errorContains string
	}{
		{
			name:          "Nil source",
			src:           nil,
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "cannot be nil",
		},
		{
			name:          "Nil destination",
			src:           srcWithData,
			dst:           nil,
			expectError:   true,
			errorContains: "cannot be nil",
		},
		{
			name:          "Empty data",
			src:           codecHelpers.NewTestPixelData(frameInfo),
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "empty",
		},
		{
			name:          "Invalid JPEG 2000 data",
			src:           srcWithInvalidData,
			dst:           codecHelpers.NewTestPixelData(frameInfo),
			expectError:   true,
			errorContains: "decode failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Decode(tt.src, tt.dst, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				// Simple substring check
				errMsg := err.Error()
				found := false
				// Check if error message contains the expected substring
				for i := 0; i <= len(errMsg)-len(tt.errorContains); i++ {
					if errMsg[i:i+len(tt.errorContains)] == tt.errorContains {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Error message '%s' does not contain '%s'", errMsg, tt.errorContains)
				}
			}
		})
	}
}

// TestEncodeErrorHandling tests various error conditions for encode
func TestEncodeErrorHandling(t *testing.T) {
	c := NewCodec()

	frameInfo := &imagetypes.FrameInfo{
		Width:           64,
		Height:          64,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}

	srcWithData := codecHelpers.NewTestPixelData(frameInfo)
	srcWithData.AddFrame([]byte{1})

	frameInfoSmall := &imagetypes.FrameInfo{
		Width:           8,
		Height:          8,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	srcValid := codecHelpers.NewTestPixelData(frameInfoSmall)
	srcValid.AddFrame(make([]byte, 64))

	tests := []struct {
		name        string
		src         imagetypes.PixelData
		dst         imagetypes.PixelData
		expectError bool
	}{
		{
			name:        "Nil source",
			src:         nil,
			dst:         codecHelpers.NewTestPixelData(frameInfo),
			expectError: true,
		},
		{
			name:        "Nil destination",
			src:         srcWithData,
			dst:         nil,
			expectError: true,
		},
		{
			name:        "Empty data",
			src:         codecHelpers.NewTestPixelData(frameInfo),
			dst:         codecHelpers.NewTestPixelData(frameInfo),
			expectError: true,
		},
		{
			name:        "Valid data (encoding works)",
			src:         srcValid,
			dst:         codecHelpers.NewTestPixelData(frameInfoSmall),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Encode(tt.src, tt.dst, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
