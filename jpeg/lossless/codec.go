package lossless

import (
	"encoding/binary"
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
	// DICOM UID 1.2.840.10008.1.2.4.57 (transfer.JPEGLossless) is Selection Value 1 only.
	// Enforce predictor=1 for this TS to match reader expectations.
	if c.transferSyntax == transfer.JPEGLossless || predictor == 0 {
		predictor = 1
	}

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

		// Shift signed samples into unsigned domain per JPEG Lossless expectations.
		adjusted := frameData
		if frameInfo.PixelRepresentation != 0 {
			shifted, serr := shiftSignedFrame(frameData, frameInfo.BitsStored, frameInfo.HighBit, frameInfo.BitsAllocated, true)
			if serr != nil {
				return fmt.Errorf("failed to shift signed frame %d: %w", frameIndex, serr)
			}
			adjusted = shifted
		}

		// Encode using the lossless encoder
		jpegData, err := Encode(
			adjusted,
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

		// Shift back to signed domain if needed.
		decodedFrame := pixelData
		if frameInfo.PixelRepresentation != 0 {
			shifted, serr := shiftSignedFrame(pixelData, frameInfo.BitsStored, frameInfo.HighBit, frameInfo.BitsAllocated, false)
			if serr != nil {
				return fmt.Errorf("failed to unshift decoded frame %d: %w", frameIndex, serr)
			}
			decodedFrame = shifted
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(decodedFrame); err != nil {
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

// shiftSignedFrame shifts signed samples into unsigned domain (encode) or back (decode).
// It respects BitsStored/HighBit and BitsAllocated for correct two's-complement handling.
func shiftSignedFrame(frame []byte, bitsStored, highBit, bitsAllocated uint16, toUnsigned bool) ([]byte, error) {
	if bitsStored == 0 || bitsStored > bitsAllocated || bitsAllocated > 16 {
		return nil, fmt.Errorf("unsupported BitsStored=%d BitsAllocated=%d", bitsStored, bitsAllocated)
	}
	if highBit >= bitsAllocated {
		return nil, fmt.Errorf("invalid HighBit=%d for BitsAllocated=%d", highBit, bitsAllocated)
	}

	bytesPerSample := int((bitsAllocated + 7) / 8)
	if len(frame)%bytesPerSample != 0 {
		return nil, fmt.Errorf("frame length %d is not aligned to %d bytes/sample", len(frame), bytesPerSample)
	}

	offset := int32(1) << (bitsStored - 1)
	maxUnsigned := int32((1 << bitsStored) - 1)
	minSigned := -offset
	maxSigned := offset - 1
	signMask := uint32(1) << highBit
	valueMask := uint32((uint64(1) << (highBit + 1)) - 1)

	out := make([]byte, len(frame))
	for i := 0; i < len(frame); i += bytesPerSample {
		var raw uint32
		if bytesPerSample == 1 {
			raw = uint32(frame[i])
		} else {
			raw = uint32(binary.LittleEndian.Uint16(frame[i:]))
		}

		if toUnsigned {
			signedVal := signExtend(raw, valueMask, signMask)
			unsignedVal := signedVal + offset
			if unsignedVal < 0 {
				unsignedVal = 0
			}
			if unsignedVal > maxUnsigned {
				unsignedVal = maxUnsigned
			}
			if bytesPerSample == 1 {
				out[i] = byte(unsignedVal)
			} else {
				binary.LittleEndian.PutUint16(out[i:], uint16(unsignedVal))
			}
		} else {
			signedVal := int32(raw) - offset
			if signedVal < minSigned {
				signedVal = minSigned
			}
			if signedVal > maxSigned {
				signedVal = maxSigned
			}
			stored := uint32(uint64(signedVal) & uint64((1<<bitsStored)-1))
			if (stored & signMask) != 0 {
				upperMask := ^uint32((uint64(1) << (highBit + 1)) - 1)
				stored |= upperMask
			}
			if bytesPerSample == 1 {
				out[i] = byte(stored & 0xFF)
			} else {
				binary.LittleEndian.PutUint16(out[i:], uint16(stored))
			}
		}
	}

	return out, nil
}

func signExtend(raw uint32, valueMask uint32, signMask uint32) int32 {
	val := raw & valueMask
	if (val & signMask) != 0 {
		val |= ^valueMask
	}
	return int32(val)
}
