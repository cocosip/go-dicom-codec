package lossless

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the JPEG 2000 Lossless codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.90
type Codec struct {
	transferSyntax *transfer.Syntax
}

// NewCodec creates a new JPEG 2000 Lossless codec
func NewCodec() *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000Lossless)
}

// NewCodecWithTransferSyntax allows constructing the codec for alternate JPEG 2000 transfer syntaxes.
func NewCodecWithTransferSyntax(ts *transfer.Syntax) *Codec {
    return &Codec{
        transferSyntax: ts,
    }
}

// NewPart2MultiComponentLosslessCodec creates a JPEG 2000 Part 2 Multi-component Lossless codec (UID .92)
func NewPart2MultiComponentLosslessCodec() *Codec {
    return NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponentLosslessOnly)
}

// Name returns the codec name
func (c *Codec) Name() string {
	return "JPEG 2000 Lossless"
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	return NewLosslessParameters()
}

// Encode encodes pixel data to JPEG 2000 Lossless format
func (c *Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Get encoding parameters
	var losslessParams *JPEG2000LosslessParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEG2000LosslessParameters); ok {
			losslessParams = jp
		} else {
			// Fallback: create from generic parameters
			losslessParams = NewLosslessParameters()
			if n := parameters.GetParameter("numLevels"); n != nil {
				if nInt, ok := n.(int); ok && nInt >= 0 && nInt <= 6 {
					losslessParams.NumLevels = nInt
				}
			}
		}
	} else {
		// Use defaults
		losslessParams = NewLosslessParameters()
	}

	// Validate parameters
	losslessParams.Validate()

	// Create encoding parameters
	encParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		frameInfo.PixelRepresentation != 0,
	)
    encParams.NumLevels = losslessParams.NumLevels
    if parameters != nil {
        if v := parameters.GetParameter("mctMatrix"); v != nil {
            if m, ok := v.([][]float64); ok { encParams.MCTMatrix = m }
        }
        if v := parameters.GetParameter("inverseMctMatrix"); v != nil {
            if m, ok := v.([][]float64); ok { encParams.InverseMCTMatrix = m }
        }
        if v := parameters.GetParameter("mctOffsets"); v != nil {
            if m, ok := v.([]int32); ok { encParams.MCTOffsets = m }
        }
        if v := parameters.GetParameter("mctNormScale"); v != nil {
            switch x := v.(type) { case float64: encParams.MCTNormScale = x; case float32: encParams.MCTNormScale = float64(x) }
        }
        if v := parameters.GetParameter("mctAssocType"); v != nil {
            if t, ok := v.(uint8); ok { encParams.MCTAssocType = t }
        }
        if v := parameters.GetParameter("mctMatrixElementType"); v != nil {
            if t, ok := v.(uint8); ok { encParams.MCTMatrixElementType = t }
        }
        if v := parameters.GetParameter("mcoPrecision"); v != nil {
            if t, ok := v.(uint8); ok { encParams.MCOPrecision = t }
        }
        if v := parameters.GetParameter("mcoRecordOrder"); v != nil {
            if arr, ok := v.([]uint8); ok { encParams.MCORecordOrder = arr }
        }
        if v := parameters.GetParameter("mctBindings"); v != nil {
            if arr, ok := v.([]jpeg2000.MCTBindingParams); ok { encParams.MCTBindings = arr }
        }
    }

	// Create encoder
	encoder := jpeg2000.NewEncoder(encParams)

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}

	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Encode
		encoded, err := encoder.Encode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG 2000 encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG 2000 Lossless data to uncompressed pixel data
func (c *Codec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Process all frames
	frameCount := oldPixelData.FrameCount()
	if frameCount == 0 {
		return fmt.Errorf("source pixel data is empty (no frames)")
	}

	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		// Get encoded frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Create decoder
		decoder := jpeg2000.NewDecoder()

		// Decode
		if err := decoder.Decode(frameData); err != nil {
			return fmt.Errorf("JPEG 2000 decode failed for frame %d: %w", frameIndex, err)
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(decoder.GetPixelData()); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterJPEG2000LosslessCodec registers the JPEG 2000 Lossless codec with the global registry
func RegisterJPEG2000LosslessCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec()
	registry.RegisterCodec(transfer.JPEG2000Lossless, j2kCodec)
}

// RegisterJPEG2000MCLosslessCodec registers JPEG 2000 Part 2 Multi-component lossless codec.
func RegisterJPEG2000MCLosslessCodec() {
    registry := codec.GetGlobalRegistry()
    j2kCodec := NewPart2MultiComponentLosslessCodec()
    registry.RegisterCodec(transfer.JPEG2000Part2MultiComponentLosslessOnly, j2kCodec)
}

func init() {
	RegisterJPEG2000LosslessCodec()
	RegisterJPEG2000MCLosslessCodec()
}
