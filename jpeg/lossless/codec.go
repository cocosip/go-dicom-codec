package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// LosslessCodec implements the external codec.Codec interface for JPEG Lossless (Process 14)
type LosslessCodec struct {
	transferSyntax *transfer.TransferSyntax
	predictor      int // 0 for auto-select, 1-7 for specific predictor
}

// NewLosslessCodec creates a new JPEG Lossless codec
// predictor: 0 for auto-select, 1-7 for specific predictor
func NewLosslessCodec(predictor int) *LosslessCodec {
	return &LosslessCodec{
		transferSyntax: transfer.JPEGLossless,
		predictor:      predictor,
	}
}

// Name returns the codec name
func (c *LosslessCodec) Name() string {
	if c.predictor == 0 {
		return "JPEG Lossless (Auto Predictor)"
	}
	return fmt.Sprintf("JPEG Lossless (Predictor %d - %s)", c.predictor, PredictorName(c.predictor))
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *LosslessCodec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG Lossless format
func (c *LosslessCodec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Get predictor from parameters if provided, otherwise use codec's default
	predictor := c.predictor
	if params != nil {
		if p := params.GetParameter("predictor"); p != nil {
			if pInt, ok := p.(int); ok {
				predictor = pInt
			}
		}
	}

	// Encode using the lossless encoder
	jpegData, err := Encode(
		src.Data,
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		predictor,
	)
	if err != nil {
		return fmt.Errorf("JPEG Lossless encode failed: %w", err)
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

// Decode decodes JPEG Lossless data to uncompressed pixel data
func (c *LosslessCodec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Decode using the lossless decoder
	pixelData, width, height, components, bitDepth, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG Lossless decode failed: %w", err)
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

// RegisterLosslessCodec registers the JPEG Lossless codec with the global registry
func RegisterLosslessCodec(predictor int) {
	registry := codec.GetGlobalRegistry()
	losslessCodec := NewLosslessCodec(predictor)
	registry.RegisterCodec(transfer.JPEGLossless, losslessCodec)
}
