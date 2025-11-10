package lossless

import (
    "testing"

    "github.com/cocosip/go-dicom/pkg/dicom/transfer"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// TestCodecInterface tests the Codec interface implementation for JPEGLSLosslessCodec
func TestCodecInterface(t *testing.T) {
	c := NewJPEGLSLosslessCodec()

	// Test UID
	expectedUID := transfer.JPEGLSLossless.UID().UID()
	if c.TransferSyntax().UID().UID() != expectedUID {
		t.Errorf("UID mismatch: got %s, want %s", c.TransferSyntax().UID().UID(), expectedUID)
	}

	// Test Name
	name := c.Name()
	t.Logf("Codec name: %s", name)
	if name == "" {
		t.Error("Name should not be empty")
	}
}

// TestCodecEncodeDecode8Bit tests codec encode/decode with 8-bit data
func TestCodecEncodeDecode8Bit(t *testing.T) {
    c := NewJPEGLSLosslessCodec()

    width, height := 64, 64
    // Create test frame (grayscale)
    pixelData := make([]byte, width*height)
    for i := range pixelData {
        pixelData[i] = byte(i % 256)
    }

    src := &codec.PixelData{
        Data:                      pixelData,
        Width:                     uint16(width),
        Height:                    uint16(height),
        NumberOfFrames:            1,
        BitsAllocated:             8,
        BitsStored:                8,
        HighBit:                   7,
        SamplesPerPixel:           1,
        PixelRepresentation:       0,
        PlanarConfiguration:       0,
        PhotometricInterpretation: "MONOCHROME2",
        TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
    }

    encoded := &codec.PixelData{}
    if err := c.Encode(src, encoded, nil); err != nil {
        t.Fatalf("Encode failed: %v", err)
    }

    decoded := &codec.PixelData{}
    if err := c.Decode(encoded, decoded, nil); err != nil {
        t.Fatalf("Decode failed: %v", err)
    }

    if int(decoded.Width) != width || int(decoded.Height) != height {
        t.Errorf("Dimension mismatch: got %dx%d, want %dx%d", decoded.Width, decoded.Height, width, height)
    }
    if decoded.SamplesPerPixel != 1 {
        t.Errorf("Components mismatch: got %d, want %d", decoded.SamplesPerPixel, 1)
    }

    errors := 0
    for i := 0; i < len(src.Data); i++ {
        if decoded.Data[i] != src.Data[i] {
            errors++
            if errors <= 5 {
                t.Errorf("Pixel %d mismatch: got %d, want %d", i, decoded.Data[i], src.Data[i])
            }
        }
    }
    if errors > 0 {
        t.Errorf("Total pixel errors: %d / %d", errors, len(src.Data))
    }
}

// TestCodecEncodeDecode12Bit tests codec with 12-bit data
func TestCodecEncodeDecode12Bit(t *testing.T) {
    c := NewJPEGLSLosslessCodec()

    width, height := 32, 32
    maxVal := 4095

    // Create 12-bit test data (stored in 16-bit little-endian)
    pixelData := make([]byte, width*height*2)
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            idx := (y*width + x) * 2
            val := ((x + y*2) * 16) % (maxVal + 1)
            pixelData[idx] = byte(val & 0xFF)
            pixelData[idx+1] = byte((val >> 8) & 0xFF)
        }
    }

    src := &codec.PixelData{
        Data:                      pixelData,
        Width:                     uint16(width),
        Height:                    uint16(height),
        NumberOfFrames:            1,
        BitsAllocated:             16,
        BitsStored:                12,
        HighBit:                   11,
        SamplesPerPixel:           1,
        PixelRepresentation:       0,
        PlanarConfiguration:       0,
        PhotometricInterpretation: "MONOCHROME2",
        TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
    }

    encoded := &codec.PixelData{}
    if err := c.Encode(src, encoded, nil); err != nil {
        t.Fatalf("12-bit Encode failed: %v", err)
    }

    decoded := &codec.PixelData{}
    if err := c.Decode(encoded, decoded, nil); err != nil {
        t.Fatalf("12-bit Decode failed: %v", err)
    }

    errors := 0
    for i := 0; i < len(src.Data); i++ {
        if decoded.Data[i] != src.Data[i] {
            errors++
        }
    }
    if errors > 0 {
        t.Errorf("12-bit: Total pixel errors: %d / %d", errors, len(src.Data))
    }
}

// TestCodecRGB tests codec with RGB data
func TestCodecRGB(t *testing.T) {
    c := NewJPEGLSLosslessCodec()

    width, height := 32, 32
    components := 3

    // Create RGB test data
    pixelData := make([]byte, width*height*components)
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            idx := (y*width + x) * components
            pixelData[idx+0] = byte(x * 8)
            pixelData[idx+1] = byte(y * 8)
            pixelData[idx+2] = byte((x + y) * 4)
        }
    }

    src := &codec.PixelData{
        Data:                      pixelData,
        Width:                     uint16(width),
        Height:                    uint16(height),
        NumberOfFrames:            1,
        BitsAllocated:             8,
        BitsStored:                8,
        HighBit:                   7,
        SamplesPerPixel:           uint16(components),
        PixelRepresentation:       0,
        PlanarConfiguration:       0,
        PhotometricInterpretation: "RGB",
        TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
    }

    encoded := &codec.PixelData{}
    if err := c.Encode(src, encoded, nil); err != nil {
        t.Fatalf("RGB Encode failed: %v", err)
    }

    decoded := &codec.PixelData{}
    if err := c.Decode(encoded, decoded, nil); err != nil {
        t.Fatalf("RGB Decode failed: %v", err)
    }

    errors := 0
    for i := 0; i < len(src.Data); i++ {
        if decoded.Data[i] != src.Data[i] {
            errors++
        }
    }
    if errors > 0 {
        t.Errorf("RGB: Total pixel errors: %d / %d", errors, len(src.Data))
    }
}

// TestCodecRegistry tests codec registration
func TestCodecRegistry(t *testing.T) {
    // Ensure codec registered
    RegisterJPEGLSLosslessCodec()

    // Get from global registry
    registry := codec.GetGlobalRegistry()
    retrievedCodec, exists := registry.GetCodec(transfer.JPEGLSLossless)
    if !exists {
        t.Fatal("Codec not found in registry")
    }

    // Test encode/decode through registry codec
    width, height := 32, 32
    pixelData := make([]byte, width*height)
    for i := range pixelData {
        pixelData[i] = byte(i % 256)
    }

    src := &codec.PixelData{
        Data:                      pixelData,
        Width:                     uint16(width),
        Height:                    uint16(height),
        NumberOfFrames:            1,
        BitsAllocated:             8,
        BitsStored:                8,
        HighBit:                   7,
        SamplesPerPixel:           1,
        PixelRepresentation:       0,
        PlanarConfiguration:       0,
        PhotometricInterpretation: "MONOCHROME2",
        TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
    }

    encoded := &codec.PixelData{}
    err := retrievedCodec.Encode(src, encoded, nil)
    if err != nil {
        t.Fatalf("Registry encode failed: %v", err)
    }

    decoded := &codec.PixelData{}
    err = retrievedCodec.Decode(encoded, decoded, nil)
    if err != nil {
        t.Fatalf("Registry decode failed: %v", err)
    }

    for i := 0; i < len(src.Data); i++ {
        if decoded.Data[i] != src.Data[i] {
            t.Errorf("Pixel %d mismatch in registry test", i)
            break
        }
    }
}
