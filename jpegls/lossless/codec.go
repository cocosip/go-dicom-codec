package lossless

import (
	"encoding/binary"
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg/common"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/types"
)

var _ codec.Codec = (*JPEGLSLosslessCodec)(nil)

// JPEGLSLosslessCodec implements the external codec.Codec interface for JPEG-LS Lossless
type JPEGLSLosslessCodec struct {
	transferSyntax *transfer.Syntax
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
func (c *JPEGLSLosslessCodec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *JPEGLSLosslessCodec) GetDefaultParameters() codec.Parameters {
	return codec.NewBaseParameters()
}

// Encode encodes pixel data to JPEG-LS Lossless format
func (c *JPEGLSLosslessCodec) Encode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Validate bit depth (JPEG-LS supports 2-16 bits)
	if frameInfo.BitsStored < 2 || frameInfo.BitsStored > 16 {
		return fmt.Errorf("JPEG-LS supports 2-16 bit depth, got %d bits", frameInfo.BitsStored)
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

		// For PR=1, shift only when pixel values actually跨过符号位；避免无符号数据被多余偏移。
		adjustedFrame := frameData
		if common.ShouldShiftPixelDataWithPR(frameData, int(frameInfo.BitsStored), int(frameInfo.PixelRepresentation)) {
			shifted, serr := shiftSignedFrame(frameData, frameInfo.BitsStored, frameInfo.HighBit, frameInfo.BitsAllocated, true)
			if serr != nil {
				return fmt.Errorf("failed to shift signed frame %d: %w", frameIndex, serr)
			}
			adjustedFrame = shifted
		}

		// Encode using the JPEG-LS encoder
		jpegData, err := Encode(
			adjustedFrame,
			int(frameInfo.Width),
			int(frameInfo.Height),
			int(frameInfo.SamplesPerPixel),
			int(frameInfo.BitsStored),
		)
		if err != nil {
			return fmt.Errorf("JPEG-LS Lossless encode failed for frame %d: %w", frameIndex, err)
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(jpegData); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG-LS Lossless data to uncompressed pixel data
func (c *JPEGLSLosslessCodec) Decode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
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

		// Decode using the JPEG-LS decoder
		pixelData, width, height, _, _, err := Decode(frameData)
		if err != nil {
			return fmt.Errorf("JPEG-LS Lossless decode failed for frame %d: %w", frameIndex, err)
		}

		// Verify dimensions match if specified
		if frameInfo.Width > 0 && width != int(frameInfo.Width) {
			return fmt.Errorf("decoded width (%d) doesn't match expected (%d)", width, frameInfo.Width)
		}
		if frameInfo.Height > 0 && height != int(frameInfo.Height) {
			return fmt.Errorf("decoded height (%d) doesn't match expected (%d)", height, frameInfo.Height)
		}

		// If original pixels are signed, shift decoded samples back to signed range.
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

// RegisterJPEGLSLosslessCodec registers the JPEG-LS Lossless codec with the global registry
func RegisterJPEGLSLosslessCodec() {
	registry := codec.GetGlobalRegistry()
	jpegLSCodec := NewJPEGLSLosslessCodec()
	registry.RegisterCodec(transfer.JPEGLSLossless, jpegLSCodec)
}

func init() {
	RegisterJPEGLSLosslessCodec()
}

// shiftSignedFrame shifts signed samples into unsigned domain (encode) or back (decode).
// It respects BitsStored/HighBit (sign bit) and BitsAllocated for proper two's complement handling.
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

	offset := int32(1) << (bitsStored - 1)               // half-range for signed->unsigned
	maxUnsigned := int32((1 << bitsStored) - 1)          // max in unsigned domain
	minSigned := -offset                                 // min in signed domain
	maxSigned := offset - 1                              // max in signed domain
	signMask := uint32(1) << highBit                     // sign bit position
	valueMask := uint32((uint64(1) << (highBit + 1)) - 1) // bits up to HighBit

	out := make([]byte, len(frame))
	for i := 0; i < len(frame); i += bytesPerSample {
		var raw uint32
		if bytesPerSample == 1 {
			raw = uint32(frame[i])
		} else {
			raw = uint32(binary.LittleEndian.Uint16(frame[i:]))
		}

		if toUnsigned {
			// interpret signed value using HighBit, then shift to unsigned range
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
			// convert unsigned decoded sample back to signed two's complement at HighBit position
			signedVal := int32(raw) - offset
			if signedVal < minSigned {
				signedVal = minSigned
			}
			if signedVal > maxSigned {
				signedVal = maxSigned
			}

			stored := uint32(uint64(signedVal) & uint64((1<<bitsStored)-1))
			// sign-extend into BitsAllocated using HighBit
			if (stored & signMask) != 0 {
				upperMask := ^uint32((uint64(1)<<(highBit+1)) - 1)
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

// signExtend takes raw bits (with sign at HighBit) and returns int32 signed value.
func signExtend(raw uint32, valueMask uint32, signMask uint32) int32 {
	val := raw & valueMask
	if (val & signMask) != 0 {
		val |= ^valueMask
	}
	return int32(val)
}
