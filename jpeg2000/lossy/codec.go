package lossy

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/types"
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

// NewPart2MultiComponentCodec creates a JPEG 2000 Part 2 Multi-component codec (UID .93)
// quality: 1-100 (default 80 if out of range)
func NewPart2MultiComponentCodec(quality int) *Codec {
    return NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponent, quality)
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG 2000 Lossy (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// GetDefaultParameters returns the default codec parameters
func (c *Codec) GetDefaultParameters() codec.Parameters {
	return NewLossyParameters()
}

// Encode encodes pixel data to JPEG 2000 Lossy format
func (c *Codec) Encode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	// Get frame info
	frameInfo := oldPixelData.GetFrameInfo()
	if frameInfo == nil {
		return fmt.Errorf("failed to get frame info from source pixel data")
	}

	// Get encoding parameters
	var lossyParams *JPEG2000LossyParameters
	if parameters != nil {
		// Try to use typed parameters if provided
		if jp, ok := parameters.(*JPEG2000LossyParameters); ok {
			lossyParams = jp
		} else {
			// Fallback: create from generic parameters
			lossyParams = NewLossyParameters()
			if q := parameters.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					lossyParams.Quality = qInt
				}
			}
			if nl := parameters.GetParameter("numLevels"); nl != nil {
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

	// Create encoding parameters
	baseEncParams := jpeg2000.DefaultEncodeParams(
		int(frameInfo.Width),
		int(frameInfo.Height),
		int(frameInfo.SamplesPerPixel),
		int(frameInfo.BitsStored),
		frameInfo.PixelRepresentation != 0,
	)

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

		// Rate control: if TargetRatio > 0, adjust quality to approach target ratio.
		var encoded []byte
		var encErr error
		if lossyParams.TargetRatio > 0 {
			encoded, encErr = c.encodeFrameWithTargetRatio(frameData, frameInfo, lossyParams, baseEncParams)
		} else {
			encoded, encErr = c.encodeFrameOnce(frameData, frameInfo, lossyParams, baseEncParams)
		}
		if encErr != nil {
			return encErr
		}

		// Add encoded frame to destination
		if err := newPixelData.AddFrame(encoded); err != nil {
			return fmt.Errorf("failed to add encoded frame %d: %w", frameIndex, err)
		}
	}

	return nil
}

// Decode decodes JPEG 2000 Lossy data to uncompressed pixel data
func (c *Codec) Decode(oldPixelData types.PixelData, newPixelData types.PixelData, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
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

		// Create decoder
		decoder := jpeg2000.NewDecoder()

		// Decode (decoder automatically detects lossy vs lossless from codestream)
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

// RegisterJPEG2000LossyCodec registers the JPEG 2000 Lossy codec with the global registry
func RegisterJPEG2000LossyCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec(80) // Default quality: 80
	registry.RegisterCodec(transfer.JPEG2000, j2kCodec)
}

// RegisterJPEG2000MultiComponentCodec registers JPEG 2000 Part 2 multi-component codec.
func RegisterJPEG2000MultiComponentCodec() {
    registry := codec.GetGlobalRegistry()
    j2kCodec := NewPart2MultiComponentCodec(80)
    registry.RegisterCodec(transfer.JPEG2000Part2MultiComponent, j2kCodec)
}

func init() {
	RegisterJPEG2000LossyCodec()
	RegisterJPEG2000MultiComponentCodec()
}

// encodeFrameOnce performs a single encode using the provided parameters for a single frame.
func (c *Codec) encodeFrameOnce(frameData []byte, frameInfo *types.FrameInfo, p *JPEG2000LossyParameters, baseEncParams *jpeg2000.EncodeParams) ([]byte, error) {
	encParams := *baseEncParams
	encParams.Lossless = false
	encParams.NumLevels = clampNumLevels(p.NumLevels, int(frameInfo.Width), int(frameInfo.Height))
	encParams.NumLayers = p.NumLayers
	encParams.Quality = effectiveQuality(p)
	if int(frameInfo.SamplesPerPixel) >= 3 && encParams.Quality < 100 {
		bump := 10
		q := encParams.Quality + bump
		if q > 100 {
			q = 100
		}
		encParams.Quality = q
	}

	minDim := int(frameInfo.Width)
	if int(frameInfo.Height) < minDim {
		minDim = int(frameInfo.Height)
	}
	if int(frameInfo.SamplesPerPixel) >= 3 && minDim <= 32 {
		encParams.Lossless = true
		if encParams.NumLevels > 1 {
			encParams.NumLevels = 1
		}
	}
	if encParams.Lossless == false && minDim <= 48 {
		if encParams.NumLevels > 1 {
			encParams.NumLevels = 1
		}
		if encParams.NumLayers > 2 {
			encParams.NumLayers = 2
		}
		q := encParams.Quality
		if q < 92 {
			encParams.Quality = 92
		}
	}
	encParams.TargetRatio = p.TargetRatio
	encParams.CustomQuantSteps = customQuantSteps(p, encParams.NumLevels)

	if p != nil {
		if v := p.GetParameter("mctMatrix"); v != nil {
			if m, ok := v.([][]float64); ok {
				encParams.MCTMatrix = m
			}
		}
		if v := p.GetParameter("inverseMctMatrix"); v != nil {
			if m, ok := v.([][]float64); ok {
				encParams.InverseMCTMatrix = m
			}
		}
		if v := p.GetParameter("mctOffsets"); v != nil {
			if m, ok := v.([]int32); ok {
				encParams.MCTOffsets = m
			}
		}
		if v := p.GetParameter("mctNormScale"); v != nil {
			switch x := v.(type) {
			case float64:
				encParams.MCTNormScale = x
			case float32:
				encParams.MCTNormScale = float64(x)
			}
		}
		if v := p.GetParameter("mctAssocType"); v != nil {
			if t, ok := v.(uint8); ok {
				encParams.MCTAssocType = t
			}
		}
		if v := p.GetParameter("mctMatrixElementType"); v != nil {
			if t, ok := v.(uint8); ok {
				encParams.MCTMatrixElementType = t
			}
		}
		if v := p.GetParameter("mcoPrecision"); v != nil {
			if t, ok := v.(uint8); ok {
				encParams.MCOPrecision = t
			}
		}
		if v := p.GetParameter("mcoRecordOrder"); v != nil {
			if arr, ok := v.([]uint8); ok {
				encParams.MCORecordOrder = arr
			}
		}
		if v := p.GetParameter("mctBindings"); v != nil {
			if arr, ok := v.([]jpeg2000.MCTBindingParams); ok {
				encParams.MCTBindings = arr
			}
		}
	}

	encoder := jpeg2000.NewEncoder(&encParams)
	encoded, err := encoder.Encode(frameData)
	if err != nil {
		return nil, fmt.Errorf("JPEG 2000 encode failed: %w", err)
	}
	return encoded, nil
}

// encodeFrameWithTargetRatio performs rate control on quality to reach target ratio for a single frame.
func (c *Codec) encodeFrameWithTargetRatio(frameData []byte, frameInfo *types.FrameInfo, base *JPEG2000LossyParameters, baseEncParams *jpeg2000.EncodeParams) ([]byte, error) {
	target := base.TargetRatio
	if target <= 0 {
		return c.encodeFrameOnce(frameData, frameInfo, base, baseEncParams)
	}

	pcopy := *base
	pcopy.TargetRatio = target
	return c.encodeFrameOnce(frameData, frameInfo, &pcopy, baseEncParams)
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
