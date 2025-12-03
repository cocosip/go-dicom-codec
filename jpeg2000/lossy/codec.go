package lossy

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the JPEG 2000 Lossy codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.91
type Codec struct {
	transferSyntax *transfer.Syntax
	quality        int // Default quality (1-100)
}

// NewCodec creates a new JPEG 2000 Lossy codec
// quality: 1-100, where 100 is near-lossless (default: 80)
func NewCodec(quality int) *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000, quality)
}

// NewCodecWithTransferSyntax allows constructing the codec for alternate JPEG 2000 transfer syntaxes.
func NewCodecWithTransferSyntax(ts *transfer.Syntax, quality int) *Codec {
	if quality < 1 || quality > 100 {
		quality = 80 // default
	}
	return &Codec{
		transferSyntax: ts, // Lossy or Lossless
		quality:        quality,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG 2000 Lossy (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
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

	// Get encoding parameters
	var lossyParams *JPEG2000LossyParameters
	if params != nil {
		// Try to use typed parameters if provided
		if jp, ok := params.(*JPEG2000LossyParameters); ok {
			lossyParams = jp
		} else {
			// Fallback: create from generic parameters
			lossyParams = NewLossyParameters()
			if q := params.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					lossyParams.Quality = qInt
				}
			}
			if nl := params.GetParameter("numLevels"); nl != nil {
				if nlInt, ok := nl.(int); ok && nlInt >= 0 && nlInt <= 6 {
					lossyParams.NumLevels = nlInt
				}
			}
		}
	} else {
		// Use codec defaults
		lossyParams = NewLossyParameters()
		lossyParams.Quality = c.quality
	}

	// Validate parameters
	lossyParams.Validate()

	// Rate control: if TargetRatio > 0, adjust quality to approach target ratio.
	var encoded []byte
	var err error
	if lossyParams.TargetRatio > 0 {
		encoded, err = c.encodeWithTargetRatio(src, lossyParams)
	} else {
		encoded, err = c.encodeOnce(src, lossyParams)
	}
	if err != nil {
		return err
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
	j2kCodec := NewCodec(80) // Default quality: 80
	registry.RegisterCodec(transfer.JPEG2000, j2kCodec)
}

// RegisterJPEG2000MultiComponentCodec registers JPEG 2000 Part 2 multi-component codec.
func RegisterJPEG2000MultiComponentCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponent, 80)
	registry.RegisterCodec(transfer.JPEG2000Part2MultiComponent, j2kCodec)
}

func init() {
	RegisterJPEG2000LossyCodec()
	RegisterJPEG2000MultiComponentCodec()
}

// encodeOnce performs a single encode using the provided parameters.
func (c *Codec) encodeOnce(src *codec.PixelData, p *JPEG2000LossyParameters) ([]byte, error) {
	encParams := jpeg2000.DefaultEncodeParams(
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		src.PixelRepresentation != 0,
	)
	encParams.Lossless = false
	encParams.NumLevels = clampNumLevels(p.NumLevels, int(src.Width), int(src.Height))
	encParams.NumLayers = p.NumLayers
	encParams.Quality = effectiveQuality(p)
	encParams.TargetRatio = p.TargetRatio
	encParams.CustomQuantSteps = customQuantSteps(p, encParams.NumLevels)

	encoder := jpeg2000.NewEncoder(encParams)
	encoded, err := encoder.Encode(src.Data)
	if err != nil {
		return nil, fmt.Errorf("JPEG 2000 encode failed: %w", err)
	}
	return encoded, nil
}

// encodeWithTargetRatio performs rate control on quality to reach target ratio.
// 使用单次编码 + PCRD 截断，避免多次重复编码。
func (c *Codec) encodeWithTargetRatio(src *codec.PixelData, base *JPEG2000LossyParameters) ([]byte, error) {
	target := base.TargetRatio
	if target <= 0 {
		return c.encodeOnce(src, base)
	}

	pcopy := *base
	pcopy.TargetRatio = target
	return c.encodeOnce(src, &pcopy)
}

// clampNumLevels limits decomposition levels so the LL band remains at least 2x2.
// This avoids excessive quantization on very small images (e.g., 16x16).
func clampNumLevels(requested, width, height int) int {
	minDim := width
	if height < minDim {
		minDim = height
	}

	maxLevels := 0
	// Each level halves dimensions; keep LL >= 2 pixels on the shortest side.
	for maxLevels < 6 && (minDim>>(maxLevels+1)) >= 1 {
		maxLevels++
	}
	if maxLevels < 0 {
		maxLevels = 0
	}
	if requested > maxLevels {
		return maxLevels
	}
	return requested
}

// effectiveQuality derives the final quality value after considering target ratio and quantization scaling.
func effectiveQuality(p *JPEG2000LossyParameters) int {
	q := p.Quality

	// If a target ratio is provided, estimate a quality value from it.
	if p.TargetRatio > 0 {
		q = qualityFromRatio(p.TargetRatio)
	}

	// Apply quantization step scaling by adjusting quality in log2 domain.
	if p.QuantStepScale > 0 && p.QuantStepScale != 1.0 {
		// baseStep = 2^((100-quality)/12.5); scaling step by S is equivalent to reducing quality by 12.5*log2(S).
		adjust := int(math.Round(12.5 * math.Log2(p.QuantStepScale)))
		q = clampQuality(q - adjust)
	}

	return clampQuality(q)
}

func qualityFromRatio(ratio float64) int {
	if ratio <= 0 {
		return 80
	}
	// Heuristic: quality drops logarithmically with target ratio.
	// ratio=2 -> ~85, ratio=3 -> ~76, ratio=5 -> ~65, ratio=10 -> ~50
	q := int(math.Round(100 - 15*math.Log2(ratio)))
	return clampQuality(q)
}

func clampQuality(q int) int {
	if q < 1 {
		return 1
	}
	if q > 100 {
		return 100
	}
	return q
}

// customQuantSteps returns per-subbands quant steps if provided and sized correctly, applying QuantStepScale.
func customQuantSteps(p *JPEG2000LossyParameters, numLevels int) []float64 {
	if len(p.SubbandSteps) == 0 {
		return nil
	}
	expected := 3*numLevels + 1
	if len(p.SubbandSteps) != expected {
		return nil
	}
	if p.QuantStepScale == 1.0 {
		return p.SubbandSteps
	}
	out := make([]float64, len(p.SubbandSteps))
	for i, v := range p.SubbandSteps {
		out[i] = v * p.QuantStepScale
	}
	return out
}
