package nearlossless

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*JPEGLSNearLosslessCodec)(nil)

// JPEGLSNearLosslessCodec implements the external codec.Codec interface for JPEG-LS Near-Lossless
type JPEGLSNearLosslessCodec struct {
	transferSyntax *transfer.TransferSyntax
	defaultNEAR    int // Default NEAR parameter (0-255)
}

// NewJPEGLSNearLosslessCodec creates a new JPEG-LS Near-Lossless codec
// defaultNEAR: default error bound (0=lossless, 1-255=near-lossless)
func NewJPEGLSNearLosslessCodec(defaultNEAR int) *JPEGLSNearLosslessCodec {
	if defaultNEAR < 0 || defaultNEAR > 255 {
		defaultNEAR = 2 // Default near-lossless value
	}
	return &JPEGLSNearLosslessCodec{
		transferSyntax: transfer.JPEGLSNearLossless,
		defaultNEAR:    defaultNEAR,
	}
}

// Name returns the codec name
func (c *JPEGLSNearLosslessCodec) Name() string {
	return fmt.Sprintf("JPEG-LS Near-Lossless (NEAR=%d)", c.defaultNEAR)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *JPEGLSNearLosslessCodec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG-LS Near-Lossless format
func (c *JPEGLSNearLosslessCodec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
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

	// Get NEAR parameter from parameters, otherwise use codec's default
	near := c.defaultNEAR
	if params != nil {
		if n := params.GetParameter("near"); n != nil {
			if nInt, ok := n.(int); ok && nInt >= 0 && nInt <= 255 {
				near = nInt
			}
		}
	}

	// Encode using the JPEG-LS near-lossless encoder
	jpegData, err := Encode(
		src.Data,
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		near,
	)
	if err != nil {
		return fmt.Errorf("JPEG-LS Near-Lossless encode failed: %w", err)
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

// Decode decodes JPEG-LS Near-Lossless data to uncompressed pixel data
func (c *JPEGLSNearLosslessCodec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Decode using the JPEG-LS near-lossless decoder
	pixelData, width, height, components, bitDepth, near, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG-LS Near-Lossless decode failed: %w", err)
	}

	// Verify dimensions match if specified
	if src.Width > 0 && width != int(src.Width) {
		return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, src.Width)
	}
	if src.Height > 0 && height != int(src.Height) {
		return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, src.Height)
	}

	// Store NEAR value in parameters if provided
	if params != nil {
		params.SetParameter("near", near)
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

// RegisterJPEGLSNearLosslessCodec registers the JPEG-LS Near-Lossless codec with the global registry
func RegisterJPEGLSNearLosslessCodec(defaultNEAR int) {
	registry := codec.GetGlobalRegistry()
	jpegLSCodec := NewJPEGLSNearLosslessCodec(defaultNEAR)
	registry.RegisterCodec(transfer.JPEGLSNearLossless, jpegLSCodec)
}

func init() {
	RegisterJPEGLSNearLosslessCodec(2)
}
