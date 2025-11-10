package lossless

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestLosslessCodecInterface(t *testing.T) {
	// Create codec
	losslessCodec := NewLosslessCodec(4) // Use predictor 4

	// Verify interface implementation
	var _ codec.Codec = losslessCodec

	// Test Name
	name := losslessCodec.Name()
	if name == "" {
		t.Error("Codec name should not be empty")
	}
	t.Logf("Codec name: %s", name)

	// Test TransferSyntax
	ts := losslessCodec.TransferSyntax()
	if ts == nil {
		t.Fatal("Transfer syntax should not be nil")
	}
	if ts.UID().UID() != transfer.JPEGLossless.UID().UID() {
		t.Errorf("Transfer syntax UID mismatch: got %s, want %s",
			ts.UID().UID(), transfer.JPEGLossless.UID().UID())
	}
}

func TestLosslessCodecEncodeDecode(t *testing.T) {
	// Create test pixel data
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y*2) % 256)
		}
	}

	// Create source PixelData
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

	// Create codec with predictor 4
	losslessCodec := NewLosslessCodec(4)

	// Encode
	encoded := &codec.PixelData{}
	err := losslessCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Original size: %d bytes", len(src.Data))
	t.Logf("Compressed size: %d bytes", len(encoded.Data))
	t.Logf("Compression ratio: %.2fx", float64(len(src.Data))/float64(len(encoded.Data)))

	// Verify encoded data is not empty
	if len(encoded.Data) == 0 {
		t.Fatal("Encoded data is empty")
	}

	// Verify encoded data is smaller than original (should be compressed)
	if len(encoded.Data) >= len(src.Data) {
		t.Logf("Warning: Encoded data (%d bytes) is not smaller than original (%d bytes)",
			len(encoded.Data), len(src.Data))
	}

	// Decode
	decoded := &codec.PixelData{}
	err = losslessCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	if decoded.Width != src.Width || decoded.Height != src.Height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d",
			decoded.Width, decoded.Height, src.Width, src.Height)
	}

	// Verify samples per pixel
	if decoded.SamplesPerPixel != src.SamplesPerPixel {
		t.Errorf("Samples per pixel mismatch: got %d, want %d",
			decoded.SamplesPerPixel, src.SamplesPerPixel)
	}

	// Verify perfect reconstruction (lossless)
	if len(decoded.Data) != len(src.Data) {
		t.Fatalf("Data length mismatch: got %d, want %d", len(decoded.Data), len(src.Data))
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
		t.Errorf("Total pixel errors: %d (lossless should have 0 errors)", errors)
	} else {
		t.Logf("Perfect reconstruction: all %d pixels match", len(src.Data))
	}
}

func TestLosslessCodecRGB(t *testing.T) {
	// Create RGB test data
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	// Create source PixelData
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

	// Create codec with predictor 4
	losslessCodec := NewLosslessCodec(4)

	// Encode
	encoded := &codec.PixelData{}
	err := losslessCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("RGB Original size: %d bytes", len(src.Data))
	t.Logf("RGB Compressed size: %d bytes", len(encoded.Data))

	// Decode
	decoded := &codec.PixelData{}
	err = losslessCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("RGB Total pixel errors: %d", errors)
	} else {
		t.Logf("Perfect RGB reconstruction")
	}
}

func TestLosslessCodecWithParameters(t *testing.T) {
	// Create test data
	width, height := 64, 64
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

	// Create codec with default predictor
	losslessCodec := NewLosslessCodec(0) // Auto-select

	// Create parameters and set predictor to 5
	params := codec.NewBaseParameters()
	params.SetParameter("predictor", 5)

	// Encode with parameters
	encoded := &codec.PixelData{}
	err := losslessCodec.Encode(src, encoded, params)
	if err != nil {
		t.Fatalf("Encode with parameters failed: %v", err)
	}

	// Decode
	decoded := &codec.PixelData{}
	err = losslessCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify perfect reconstruction
	errors := 0
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Total pixel errors with parameters: %d", errors)
	} else {
		t.Logf("Perfect reconstruction with predictor from parameters")
	}
}

func TestCodecRegistry(t *testing.T) {
	// Register codec
	RegisterLosslessCodec(4)

	// Get from global registry
	registry := codec.GetGlobalRegistry()
	retrievedCodec, exists := registry.GetCodec(transfer.JPEGLossless)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	if retrievedCodec == nil {
		t.Fatal("Retrieved codec is nil")
	}

	// Verify it's the correct codec
	name := retrievedCodec.Name()
	t.Logf("Retrieved codec name: %s", name)

	// Test with the retrieved codec
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
		t.Fatalf("Encode with retrieved codec failed: %v", err)
	}

	decoded := &codec.PixelData{}
	err = retrievedCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode with retrieved codec failed: %v", err)
	}

	// Verify reconstruction
	for i := 0; i < len(src.Data); i++ {
		if decoded.Data[i] != src.Data[i] {
			t.Errorf("Pixel %d mismatch with registry codec", i)
			break
		}
	}

	t.Logf("Registry codec test passed")
}
