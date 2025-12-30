package lossless14sv1

import (
	"encoding/binary"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

var _ codec.Codec = (*LosslessSV1Codec)(nil)

// LosslessSV1Codec implements the external codec.Codec interface for JPEG Lossless SV1
// SV1 (Selection Value 1) means it only uses predictor 1 (left pixel)
type LosslessSV1Codec struct {
	transferSyntax *transfer.Syntax
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
func (c *LosslessSV1Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *LosslessSV1Codec) GetDefaultParameters() codec.Parameters {
	return codec.NewBaseParameters()
}

// Encode encodes pixel data to JPEG Lossless SV1 format
func (c *LosslessSV1Codec) Encode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
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
		// Get frame data
		frameData, err := oldPixelData.GetFrame(frameIndex)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", frameIndex, err)
		}

		if len(frameData) == 0 {
			return fmt.Errorf("frame %d pixel data is empty", frameIndex)
		}

		// Determine if shift is needed; only when PR=1 and values cross sign bit.
		adjustedFrame := frameData
		// For PR=1, shift only when pixel values actually cross sign bit; avoids disturbing unsigned-but-tagged data.
		needsShift := common.ShouldShiftPixelDataWithPR(frameData, int(frameInfo.BitsStored), int(frameInfo.PixelRepresentation))

		if needsShift {
			// Pixel values exceed threshold - shift to unsigned range for JPEG
			shifted, err := shiftSignedFrame(frameData, frameInfo.BitsStored, frameInfo.HighBit, frameInfo.BitsAllocated, true)
			if err != nil {
				return fmt.Errorf("failed to shift signed frame %d: %w", frameIndex, err)
			}
			adjustedFrame = shifted
		}

		// Encode using the lossless SV1 encoder
		jpegData, err := Encode(
			adjustedFrame,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitsStored),
		)
		if err != nil {
			return fmt.Errorf("JPEG Lossless SV1 encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG Lossless SV1 data to uncompressed pixel data
func (c *LosslessSV1Codec) Decode(oldPixelData imagetypes.PixelData, newPixelData imagetypes.PixelData, parameters codec.Parameters) error {
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

		// Decode using the lossless SV1 decoder
		pixelData, width, height, components, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG Lossless SV1 decode failed for frame %d: %w", frameIndex, err)
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

		// JPEG Lossless decode: reverse shift only when数据明显处于“已平移的无符号域”
		adjustedPixels := pixelData
		if common.ShouldReverseShiftPixelData(pixelData, int(frameInfo.BitsStored), int(frameInfo.PixelRepresentation)) {
			shifted, err := shiftSignedFrame(pixelData, frameInfo.BitsStored, frameInfo.HighBit, frameInfo.BitsAllocated, false)
			if err != nil {
				return fmt.Errorf("failed to reverse shift for frame %d: %w", frameIndex, err)
			}
			adjustedPixels = shifted
		}

		// Add decoded frame to destination
		if err := newPixelData.AddFrame(adjustedPixels); err != nil {
			return fmt.Errorf("failed to add decoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// RegisterLosslessSV1Codec registers the JPEG Lossless SV1 codec with the global registry
func RegisterLosslessSV1Codec() {
	registry := codec.GetGlobalRegistry()
	losslessSV1Codec := NewLosslessSV1Codec()
	registry.RegisterCodec(transfer.JPEGLosslessSV1, losslessSV1Codec)
}

func init() {
	RegisterLosslessSV1Codec()
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
