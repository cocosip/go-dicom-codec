package htj2k

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
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

	// Create JPEG 2000 encoding parameters with HTJ2K enabled
	encParams := jpeg2000.DefaultEncodeParams(
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		src.PixelRepresentation != 0,
	)

	// Configure HTJ2K-specific settings
	// Adjust NumLevels based on image size to ensure minimum subband size >= 1
	maxLevels := calculateMaxLevels(int(src.Width), int(src.Height))
	// Cap at 5 for now due to issues with larger images (TODO: investigate)
	if maxLevels > 5 {
		maxLevels = 5
	}
	if htj2kParams.NumLevels > maxLevels {
		encParams.NumLevels = maxLevels
	} else {
		encParams.NumLevels = htj2kParams.NumLevels
	}
	encParams.CodeBlockWidth = htj2kParams.BlockWidth
	encParams.CodeBlockHeight = htj2kParams.BlockHeight

	// Set HTJ2K block encoder factory
	encParams.BlockEncoderFactory = func(width, height int) jpeg2000.BlockEncoder {
		return NewHTEncoder(width, height)
	}

	// Configure lossless vs lossy mode
	if c.lossless {
		encParams.Lossless = true
	} else {
		encParams.Lossless = false
		encParams.Quality = htj2kParams.Quality
	}

	// Set progression order based on transfer syntax
	if c.transferSyntax == transfer.HTJ2KLosslessRPCL {
		encParams.ProgressionOrder = 2 // RPCL
	}

	// Create encoder with HTJ2K enabled
	encoder := jpeg2000.NewEncoder(encParams)

	// Encode using full JPEG 2000 pipeline (DWT + HTJ2K block coding + T2)
	encoded, err := encoder.Encode(src.Data)
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

	// Create JPEG 2000 decoder
	decoder := jpeg2000.NewDecoder()

	// Set HTJ2K block decoder factory
	// The decoder will use this factory to create HTJ2K block decoders instead of EBCOT T1 decoders
	decoder.SetBlockDecoderFactory(func(width, height int, cblkstyle int) t2.BlockDecoder {
		return NewHTDecoder(width, height)
	})

	// Decode using full JPEG 2000 pipeline (T2 + HTJ2K block decoding + Inverse DWT)
	if err := decoder.Decode(src.Data); err != nil {
		return fmt.Errorf("HTJ2K decode failed: %w", err)
	}

	// Get decoded pixel data
	pixelData := decoder.GetPixelData()

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

// calculateMaxLevels calculates the maximum number of wavelet decomposition levels
// that can be applied to an image of given dimensions.
// Each level divides dimensions by 2, so max levels = floor(log2(min(width, height)))
func calculateMaxLevels(width, height int) int {
	minDim := width
	if height < minDim {
		minDim = height
	}

	if minDim <= 0 {
		return 0
	}

	// Calculate floor(log2(minDim))
	maxLevels := 0
	for (1 << maxLevels) < minDim {
		maxLevels++
	}

	// Cap at 6 levels (JPEG2000 standard limit)
	if maxLevels > 6 {
		maxLevels = 6
	}

	return maxLevels
}
