package nearlossless

import (
    "strings"
    "testing"

    "github.com/cocosip/go-dicom/pkg/dicom/transfer"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// TestCodecInterface verifies that Codec implements codec.Codec
func TestCodecInterface(t *testing.T) {
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

    cases := []struct{
        name string
        near int
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
            encoded := &codec.PixelData{}
            err := c.Encode(src, encoded, params)
            if (err != nil) != tc.wantErr {
                t.Fatalf("Encode error=%v, wantErr=%v", err, tc.wantErr)
            }
            if !tc.wantErr && len(encoded.Data) == 0 {
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

    // Base src
    baseSrc := &codec.PixelData{
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

    tests := []struct {
        name    string
        mutate  func(*codec.PixelData) codec.Parameters
        wantErr bool
    }{
        {
            name: "Valid NEAR=0 (lossless)",
            mutate: func(src *codec.PixelData) codec.Parameters {
                p := codec.NewBaseParameters(); p.SetParameter("near", 0); return p
            },
            wantErr: false,
        },
        {
            name: "Valid NEAR=3",
            mutate: func(src *codec.PixelData) codec.Parameters {
                p := codec.NewBaseParameters(); p.SetParameter("near", 3); return p
            },
            wantErr: false,
        },
        {
            name: "Invalid width",
            mutate: func(src *codec.PixelData) codec.Parameters {
                src.Width = 0; return nil
            },
            wantErr: true,
        },
        {
            name: "Invalid components",
            mutate: func(src *codec.PixelData) codec.Parameters {
                src.SamplesPerPixel = 2; return nil
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // copy src to avoid mutation leakage
            src := *baseSrc
            params := tt.mutate(&src)
            encoded := &codec.PixelData{}
            err := c.Encode(&src, encoded, params)
            if (err != nil) != tt.wantErr {
                t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && len(encoded.Data) == 0 {
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

    params := codec.NewBaseParameters()
    params.SetParameter("near", 3)

    encoded := &codec.PixelData{}
    if err := c.Encode(src, encoded, params); err != nil {
        t.Fatalf("Encode() failed: %v", err)
    }

    decoded := &codec.PixelData{}
    if err := c.Decode(encoded, decoded, nil); err != nil {
        t.Fatalf("Decode() failed: %v", err)
    }

    if int(decoded.Width) != width || int(decoded.Height) != height {
        t.Errorf("Decoded dimensions mismatch")
    }
    if decoded.SamplesPerPixel != 1 || decoded.BitsStored != 8 {
        t.Errorf("Decoded metadata mismatch")
    }

    // Verify error bound (NEAR=3)
    maxError := 0
    for i := 0; i < len(src.Data); i++ {
        diff := int(decoded.Data[i]) - int(src.Data[i])
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
            srcEnc := &codec.PixelData{Data: tt.data}
            dst := &codec.PixelData{}
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

            src := &codec.PixelData{
                Data:                      pixelData,
                Width:                     uint16(tt.width),
                Height:                    uint16(tt.height),
                NumberOfFrames:            1,
                BitsAllocated:             8,
                BitsStored:                8,
                HighBit:                   7,
                SamplesPerPixel:           uint16(tt.components),
                PixelRepresentation:       0,
                PlanarConfiguration:       0,
                PhotometricInterpretation: map[int]string{1: "MONOCHROME2", 3: "RGB"}[tt.components],
                TransferSyntaxUID:         transfer.ExplicitVRLittleEndian.UID().UID(),
            }

            params := codec.NewBaseParameters()
            params.SetParameter("near", tt.near)

            encoded := &codec.PixelData{}
            if err := c.Encode(src, encoded, params); err != nil {
                t.Fatalf("Encode() failed: %v", err)
            }

            decoded := &codec.PixelData{}
            if err := c.Decode(encoded, decoded, nil); err != nil {
                t.Fatalf("Decode() failed: %v", err)
            }

            maxError := 0
            for i := 0; i < len(src.Data); i++ {
                diff := int(decoded.Data[i]) - int(src.Data[i])
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
