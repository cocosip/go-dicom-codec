package lossless14sv1

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// LosslessSV1Codec implements the external codec.Codec interface for JPEG Lossless SV1
// SV1 (Selection Value 1) means it only uses predictor 1 (left pixel)
type LosslessSV1Codec struct {
	transferSyntax *transfer.TransferSyntax
}

// NewLosslessSV1Codec creates a new JPEG Lossless SV1 codec
func NewLosslessSV1Codec() *LosslessSV1Codec {
	return &LosslessSV1Codec{
		transferSyntax: transfer.JPEGLosslessSV1,
	}
}

// Name returns the codec name
func (c *LosslessSV1Codec) Name() string {
	return "JPEG Lossless SV1 (Predictor 1)"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *LosslessSV1Codec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG Lossless SV1 format
func (c *LosslessSV1Codec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Encode using the lossless SV1 encoder
	jpegData, err := Encode(
		src.Data,
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
	)
	if err != nil {
		return fmt.Errorf("JPEG Lossless SV1 encode failed: %w", err)
	}

	// Set destination data
	dst.Data = jpegData
	dst.Width = src.Width
	dst.Height = src.Height
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = src.BitsAllocated
	dst.BitsStored = src.BitsStored
	dst.HighBit = src.HighBit
	dst.SamplesPerPixel = src.SamplesPerPixel
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = c.transferSyntax.UID().UID()

	return nil
}

// Decode decodes JPEG Lossless SV1 data to uncompressed pixel data
func (c *LosslessSV1Codec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Decode using the lossless SV1 decoder
	pixelData, width, height, components, bitDepth, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG Lossless SV1 decode failed: %w", err)
	}

	// Verify dimensions match
	if width != int(src.Width) || height != int(src.Height) {
		return fmt.Errorf("decoded dimensions (%dx%d) don't match expected (%dx%d)",
			width, height, src.Width, src.Height)
	}

	if components != int(src.SamplesPerPixel) {
		return fmt.Errorf("decoded components (%d) don't match expected (%d)",
			components, src.SamplesPerPixel)
	}

	// Set destination data
	dst.Data = pixelData
	dst.Width = uint16(width)
	dst.Height = uint16(height)
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = uint16((bitDepth-1)/8 + 1) * 8
	dst.BitsStored = uint16(bitDepth)
	dst.HighBit = uint16(bitDepth - 1)
	dst.SamplesPerPixel = uint16(components)
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = 0 // Always interleaved after decode
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID()

	return nil
}

// RegisterLosslessSV1Codec registers the JPEG Lossless SV1 codec with the global registry
func RegisterLosslessSV1Codec() {
	registry := codec.GetGlobalRegistry()
	losslessSV1Codec := NewLosslessSV1Codec()
	registry.RegisterCodec(transfer.JPEGLosslessSV1, losslessSV1Codec)
}
