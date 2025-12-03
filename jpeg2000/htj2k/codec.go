package htj2k

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the HTJ2K (High-Throughput JPEG 2000) codec
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// Supported Transfer Syntaxes:
// - 1.2.840.10008.1.2.4.201: HTJ2K Lossless
// - 1.2.840.10008.1.2.4.202: HTJ2K Lossless RPCL
// - 1.2.840.10008.1.2.4.203: HTJ2K (Lossy)
type Codec struct {
	transferSyntax *transfer.Syntax
	lossless       bool
	quality        int // For lossy encoding (1-100)
}

// NewLosslessCodec creates a new HTJ2K lossless codec
func NewLosslessCodec() *Codec {
	return &Codec{
		transferSyntax: transfer.HTJ2KLossless,
		lossless:       true,
	}
}

// NewLosslessRPCLCodec creates a new HTJ2K lossless RPCL codec
func NewLosslessRPCLCodec() *Codec {
	return &Codec{
		transferSyntax: transfer.HTJ2KLosslessRPCL,
		lossless:       true,
	}
}

// NewCodec creates a new HTJ2K lossy codec with specified quality
func NewCodec(quality int) *Codec {
	if quality < 1 || quality > 100 {
		quality = 80 // default
	}
	return &Codec{
		transferSyntax: transfer.HTJ2K,
		lossless:       false,
		quality:        quality,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	if c.lossless {
		if c.transferSyntax == transfer.HTJ2KLosslessRPCL {
			return "HTJ2K Lossless RPCL"
		}
		return "HTJ2K Lossless"
	}
	return fmt.Sprintf("HTJ2K (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// Encode encodes pixel data to HTJ2K format
func (c *Codec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Get encoding parameters
	var htj2kParams *HTJ2KParameters
	if params != nil {
		// Try to use typed parameters if provided
		if hp, ok := params.(*HTJ2KParameters); ok {
			htj2kParams = hp
		} else {
			// Fallback: create from generic parameters
			htj2kParams = NewHTJ2KParameters()
			if q := params.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok {
					htj2kParams.Quality = qInt
				}
			}
			if bw := params.GetParameter("blockWidth"); bw != nil {
				if bwInt, ok := bw.(int); ok {
					htj2kParams.BlockWidth = bwInt
				}
			}
			if bh := params.GetParameter("blockHeight"); bh != nil {
				if bhInt, ok := bh.(int); ok {
					htj2kParams.BlockHeight = bhInt
				}
			}
			if nl := params.GetParameter("numLevels"); nl != nil {
				if nlInt, ok := nl.(int); ok {
					htj2kParams.NumLevels = nlInt
				}
			}
		}
	} else {
		// Use defaults
		if c.lossless {
			htj2kParams = NewHTJ2KLosslessParameters()
		} else {
			htj2kParams = NewHTJ2KParameters()
			htj2kParams.Quality = c.quality
		}
	}

	// Validate parameters
	htj2kParams.Validate()

	// Use actual image dimensions for block size if not specified
	// (simplified approach for small images)
	blockWidth := htj2kParams.BlockWidth
	blockHeight := htj2kParams.BlockHeight
	if int(src.Width) < blockWidth {
		blockWidth = int(src.Width)
	}
	if int(src.Height) < blockHeight {
		blockHeight = int(src.Height)
	}

	// Create HTJ2K block encoder
	encoder := NewHTEncoder(blockWidth, blockHeight)

	// Convert pixel data to wavelet coefficients
	// For now, treat pixel data directly as coefficients (simplified)
	// In a full implementation, this would perform DWT first
	coeffs := make([]int32, len(src.Data))
	for i, b := range src.Data {
		coeffs[i] = int32(int8(b)) // Simplified conversion
	}

	// Encode the block
	encoded, err := encoder.Encode(coeffs, 1, 0)
	if err != nil {
		return fmt.Errorf("HTJ2K encode failed: %w", err)
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

// Decode decodes HTJ2K data to uncompressed pixel data
func (c *Codec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Get decoding parameters
	var htj2kParams *HTJ2KParameters
	if params != nil {
		// Try to use typed parameters if provided
		if hp, ok := params.(*HTJ2KParameters); ok {
			htj2kParams = hp
		} else {
			// Fallback: create from generic parameters
			htj2kParams = NewHTJ2KParameters()
			if bw := params.GetParameter("blockWidth"); bw != nil {
				if bwInt, ok := bw.(int); ok {
					htj2kParams.BlockWidth = bwInt
				}
			}
			if bh := params.GetParameter("blockHeight"); bh != nil {
				if bhInt, ok := bh.(int); ok {
					htj2kParams.BlockHeight = bhInt
				}
			}
		}
	} else {
		// Use defaults
		htj2kParams = NewHTJ2KParameters()
	}

	// Validate parameters
	htj2kParams.Validate()

	// Use actual image dimensions for block size
	blockWidth := htj2kParams.BlockWidth
	blockHeight := htj2kParams.BlockHeight
	if int(src.Width) < blockWidth {
		blockWidth = int(src.Width)
	}
	if int(src.Height) < blockHeight {
		blockHeight = int(src.Height)
	}

	// Create HTJ2K block decoder
	decoder := NewHTDecoder(blockWidth, blockHeight)

	// Decode the block
	coeffs, err := decoder.Decode(src.Data, 1)
	if err != nil {
		return fmt.Errorf("HTJ2K decode failed: %w", err)
	}

	// Convert coefficients back to pixel data
	// In a full implementation, this would perform inverse DWT
	pixelData := make([]byte, len(coeffs))
	for i, c := range coeffs {
		// Clamp to byte range
		val := c
		if val < -128 {
			val = -128
		} else if val > 127 {
			val = 127
		}
		pixelData[i] = byte(val)
	}

	// Set destination data
	dst.Data = pixelData
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
	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID() // Decoded to uncompressed

	return nil
}

// RegisterHTJ2KCodecs registers all HTJ2K codecs with the global registry
func RegisterHTJ2KCodecs() {
	registry := codec.GetGlobalRegistry()

	// Register HTJ2K Lossless
	losslessCodec := NewLosslessCodec()
	registry.RegisterCodec(transfer.HTJ2KLossless, losslessCodec)

	// Register HTJ2K Lossless RPCL
	losslessRPCLCodec := NewLosslessRPCLCodec()
	registry.RegisterCodec(transfer.HTJ2KLosslessRPCL, losslessRPCLCodec)

	// Register HTJ2K Lossy
	lossyCodec := NewCodec(80) // Default quality: 80
	registry.RegisterCodec(transfer.HTJ2K, lossyCodec)
}

func init() {
	RegisterHTJ2KCodecs()
}
