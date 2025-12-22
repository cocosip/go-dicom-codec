package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/types"
)

var _ codec.Codec = (*LosslessCodec)(nil)

// LosslessCodec implements the external codec.Codec interface for JPEG Lossless (Process 14)
type LosslessCodec struct {
	transferSyntax *transfer.Syntax
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
func (c *LosslessCodec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *LosslessCodec) GetDefaultParameters() codec.Parameters {
	params := NewLosslessParameters()
	params.Predictor = c.predictor
	return params
}

// Encode encodes pixel data to JPEG Lossless format
func (c *LosslessCodec) Encode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Get encoding parameters
	var losslessParams *JPEGLosslessParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEGLosslessParameters); ok {
			losslessParams = jp
		} else {
			// Fallback: create from generic parameters
			losslessParams = NewLosslessParameters()
			if p := parameters.GetParameter("predictor"); p != nil {
				if pInt, ok := p.(int); ok {
					losslessParams.Predictor = pInt
				}
			}
		}
	} else {
		// Use codec defaults
		losslessParams = NewLosslessParameters()
		losslessParams.Predictor = c.predictor
	}

	// Validate parameters
	losslessParams.Validate()
	predictor := losslessParams.Predictor

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Encode using the lossless encoder
		jpegData, err := Encode(
			frameData,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitsStored),
			predictor,
		)
		if err != nil {
			return fmt.Errorf("JPEG Lossless encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Lossless data to uncompressed pixel data
func (c *LosslessCodec) Decode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get encoded frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Decode using the lossless decoder
		pixelData, width, height, components, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Lossless decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match
		if width != int(frameInfo.Width) || height != int(frameInfo.Height) {
			return fmt.Errorf("decoded dimensions (%dx%d) don't match expected (%dx%d)",
				width, height, frameInfo.Width, frameInfo.Height)
		}

		if components != int(frameInfo.SamplesPerPixel) {
			return fmt.Errorf("decoded components (%d) don't match expected (%d)",
				components, frameInfo.SamplesPerPixel)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(pixelData); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterLosslessCodec registers the JPEG Lossless codec with the global registry
func RegisterLosslessCodec(predictor int) {
	registry := codec.GetGlobalRegistry()
	losslessCodec := NewLosslessCodec(predictor)
	registry.RegisterCodec(transfer.JPEGLossless, losslessCodec)
}

func init() {
	RegisterLosslessCodec(0)
}
