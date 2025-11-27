package baseline

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*BaselineCodec)(nil)

// BaselineCodec implements the external codec.Codec interface for JPEG Baseline
type BaselineCodec struct {
	transferSyntax *transfer.TransferSyntax
	quality        int // Default quality (1-100)
}

// NewBaselineCodec creates a new JPEG Baseline codec
// quality: 1-100, where 100 is best quality (default: 85)
func NewBaselineCodec(quality int) *BaselineCodec {
	if quality < 1 || quality > 100 {
		quality = 85 // default
	}
	return &BaselineCodec{
		transferSyntax: transfer.JPEGBaseline8Bit,
		quality:        quality,
	}
}

// Name returns the codec name
func (c *BaselineCodec) Name() string {
	return fmt.Sprintf("JPEG Baseline (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *BaselineCodec) TransferSyntax() *transfer.TransferSyntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG Baseline format
func (c *BaselineCodec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// JPEG Baseline only supports 8-bit data
	if src.BitsStored > 8 {
		return fmt.Errorf("JPEG Baseline only supports 8-bit data, got %d bits", src.BitsStored)
	}

	// Get encoding parameters
	var baselineParams *JPEGBaselineParameters
	if params != nil {
		// Try to use typed parameters if provided
		if jp, ok := params.(*JPEGBaselineParameters); ok {
			baselineParams = jp
		} else {
			// Fallback: create from generic parameters
			baselineParams = NewBaselineParameters()
			if q := params.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					baselineParams.Quality = qInt
				}
			}
		}
	} else {
		// Use codec defaults
		baselineParams = NewBaselineParameters()
		baselineParams.Quality = c.quality
	}

	// Validate parameters
	baselineParams.Validate()
	quality := baselineParams.Quality

	// Encode using the baseline encoder
	jpegData, err := Encode(
		src.Data,
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		quality,
	)
	if err != nil {
		return fmt.Errorf("JPEG Baseline encode failed: %w", err)
	}

	// Set destination data
	dst.Data = jpegData
	dst.Width = src.Width
	dst.Height = src.Height
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = 8
	dst.BitsStored = 8
	dst.HighBit = 7
	dst.SamplesPerPixel = src.SamplesPerPixel
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = c.transferSyntax.UID().UID()

	return nil
}

// Decode decodes JPEG Baseline data to uncompressed pixel data
func (c *BaselineCodec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Decode using the baseline decoder
	pixelData, width, height, components, err := Decode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG Baseline decode failed: %w", err)
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
	dst.BitsAllocated = 8
	dst.BitsStored = 8
	dst.HighBit = 7
	dst.SamplesPerPixel = uint16(components)
	dst.PixelRepresentation = 0 // Baseline is always unsigned
	dst.PlanarConfiguration = 0 // Always interleaved after decode

	// Preserve or infer photometric interpretation
	if src.PhotometricInterpretation != "" {
		dst.PhotometricInterpretation = src.PhotometricInterpretation
	} else {
		switch components {
		case 1:
			dst.PhotometricInterpretation = "MONOCHROME2"
		case 3:
			dst.PhotometricInterpretation = "RGB"
		}
	}

	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID()

	return nil
}

// RegisterBaselineCodec registers the JPEG Baseline codec with the global registry
func RegisterBaselineCodec(quality int) {
	registry := codec.GetGlobalRegistry()
	baselineCodec := NewBaselineCodec(quality)
	registry.RegisterCodec(transfer.JPEGBaseline8Bit, baselineCodec)
}

func init() {
	RegisterBaselineCodec(85)
}
