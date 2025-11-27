package lossy

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the JPEG 2000 Lossy codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.91
type Codec struct {
	transferSyntax *transfer.TransferSyntax
}

// NewCodec creates a new JPEG 2000 Lossy codec
func NewCodec() *Codec {
	return &Codec{
		transferSyntax: transfer.JPEG2000, // Lossy or Lossless
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	return "JPEG 2000"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG 2000 Lossy format
func (c *Codec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Create encoding parameters for lossy compression
	encParams := jpeg2000.DefaultEncodeParams(
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		src.PixelRepresentation != 0,
	)

	// Configure for lossy compression (9/7 wavelet)
	encParams.Lossless = false
	encParams.NumLevels = 5 // Default 5 decomposition levels

	// Get quality parameter if provided (0-100, default 80)
	// Higher quality = less compression, lower quality = more compression
	// TODO: Implement quality parameter when codec.Parameters interface is defined
	// For now, use default settings
	_ = params

	// Create encoder
	encoder := jpeg2000.NewEncoder(encParams)

	// Encode
	encoded, err := encoder.Encode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG 2000 encode failed: %w", err)
	}

	// Set destination data
	dst.Data = encoded
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

// Decode decodes JPEG 2000 Lossy data to uncompressed pixel data
func (c *Codec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Create decoder
	decoder := jpeg2000.NewDecoder()

	// Decode (decoder automatically detects lossy vs lossless from codestream)
	if err := decoder.Decode(src.Data); err != nil {
		return fmt.Errorf("JPEG 2000 decode failed: %w", err)
	}

	// Set destination data
	dst.Data = decoder.GetPixelData()
	dst.Width = uint16(decoder.Width())
	dst.Height = uint16(decoder.Height())
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = src.BitsAllocated
	dst.BitsStored = uint16(decoder.BitDepth())
	dst.HighBit = dst.BitsStored - 1
	dst.SamplesPerPixel = uint16(decoder.Components())
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID() // Decoded to uncompressed

	return nil
}

// RegisterJPEG2000LossyCodec registers the JPEG 2000 Lossy codec with the global registry
func RegisterJPEG2000LossyCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec()
	registry.RegisterCodec(transfer.JPEG2000, j2kCodec)
}

func init() {
	RegisterJPEG2000LossyCodec()
}
