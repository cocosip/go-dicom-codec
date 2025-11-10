package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*JPEGLSLosslessCodec)(nil)

// JPEGLSLosslessCodec implements the external codec.Codec interface for JPEG-LS Lossless
type JPEGLSLosslessCodec struct {
	transferSyntax *transfer.TransferSyntax
}

// NewJPEGLSLosslessCodec creates a new JPEG-LS Lossless codec
func NewJPEGLSLosslessCodec() *JPEGLSLosslessCodec {
	return &JPEGLSLosslessCodec{
		transferSyntax: transfer.JPEGLSLossless,
	}
}

// Name returns the codec name
func (c *JPEGLSLosslessCodec) Name() string {
	return "JPEG-LS Lossless"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *JPEGLSLosslessCodec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG-LS Lossless format
func (c *JPEGLSLosslessCodec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Validate bit depth (JPEG-LS supports 2-16 bits)
	if src.BitsStored < 2 || src.BitsStored > 16 {
		return fmt.Errorf("JPEG-LS supports 2-16 bit depth, got %d bits", src.BitsStored)
	}

	// Encode using the JPEG-LS encoder
	jpegData, err := Encode(
		src.Data,
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
	)
	if err != nil {
		return fmt.Errorf("JPEG-LS Lossless encode failed: %w", err)
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

// Decode decodes JPEG-LS Lossless data to uncompressed pixel data
func (c *JPEGLSLosslessCodec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Decode using the JPEG-LS decoder
	pixelData, width, height, components, bitDepth, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG-LS Lossless decode failed: %w", err)
	}

	// Verify dimensions match if specified
	if src.Width > 0 && width != int(src.Width) {
		return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, src.Width)
	}
	if src.Height > 0 && height != int(src.Height) {
		return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, src.Height)
	}

	// Set destination data
	dst.Data = pixelData
	dst.Width = uint16(width)
	dst.Height = uint16(height)
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = uint16((bitDepth-1)/8+1) * 8
	dst.BitsStored = uint16(bitDepth)
	dst.HighBit = uint16(bitDepth - 1)
	dst.SamplesPerPixel = uint16(components)
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = 0 // Always interleaved after decode
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID()

	return nil
}

// RegisterJPEGLSLosslessCodec registers the JPEG-LS Lossless codec with the global registry
func RegisterJPEGLSLosslessCodec() {
	registry := codec.GetGlobalRegistry()
	jpegLSCodec := NewJPEGLSLosslessCodec()
	registry.RegisterCodec(transfer.JPEGLSLossless, jpegLSCodec)
}

func init() {
	RegisterJPEGLSLosslessCodec()
}
