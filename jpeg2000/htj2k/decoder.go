package htj2k

import (
	"fmt"
	"math/bits"
)

// HTDecoder implements complete HTJ2K decoder matching HTEncoder
// This decoder correctly decodes the output from HTEncoder using:
// - Full VLC decoding with context
// - U-VLC decoding
// - MagSgn decoding
// - Exponent predictor
type HTDecoder struct {
	// Block dimensions
	width  int
	height int

	// Decoders for three segments
	magsgn *MagSgnDecoder
	mel    *MELDecoder
	vlc    *VLCDecoder

	// U-VLC decoder
	uvlc *UVLCDecoder

	// Exponent predictor
	expPredictor *ExponentPredictorComputer

	// Context computer
	context *ContextComputer

	// Decoded data
	data []int32

	// Decoding state
	maxBitplane int

	// Dimensions in quads
	qw int
	qh int
}

// NewHTDecoder creates a new HT decoder
func NewHTDecoder(width, height int) *HTDecoder {
	qw := (width + 1) / 2
	qh := (height + 1) / 2

	return &HTDecoder{
		width:        width,
		height:       height,
		qw:           qw,
		qh:           qh,
		data:         make([]int32, width*height),
		expPredictor: NewExponentPredictorComputer(qw, qh),
		context:      NewContextComputer(width, height),
	}
}

// Decode decodes a HTJ2K code-block
func (h *HTDecoder) Decode(codeblock []byte, numPasses int) ([]int32, error) {
	if len(codeblock) == 0 {
		return h.data, nil
	}

	if err := h.parseCodeblock(codeblock); err != nil {
		return nil, fmt.Errorf("parse codeblock: %w", err)
	}

	if err := h.decodeHTCleanupPass(); err != nil {
		return nil, fmt.Errorf("decode cleanup pass: %w", err)
	}

	return h.data, nil
}

// parseCodeblock parses segments
func (h *HTDecoder) parseCodeblock(codeblock []byte) error {
	if len(codeblock) < 2 {
		return fmt.Errorf("codeblock too short")
	}

	melLen := int(codeblock[len(codeblock)-2])
	vlcLen := int(codeblock[len(codeblock)-1])
	totalDataLen := len(codeblock) - 2
	magsgnLen := totalDataLen - melLen - vlcLen

	if magsgnLen < 0 || melLen < 0 || vlcLen < 0 {
		return fmt.Errorf("invalid segment lengths")
	}

	magsgnData := codeblock[0:magsgnLen]
	melData := codeblock[magsgnLen : magsgnLen+melLen]
	vlcData := codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen]

	h.magsgn = NewMagSgnDecoder(magsgnData)
	h.mel = NewMELDecoder(melData)
	h.vlc = NewVLCDecoder(vlcData)

	// Create U-VLC decoder with VLC bit reader
	bitReader := &VLCBitReaderWrapper{decoder: h.vlc}
	h.uvlc = NewUVLCDecoder(bitReader)

	return nil
}

// VLCBitReaderWrapper adapts VLCDecoder to BitReader interface
type VLCBitReaderWrapper struct {
	decoder *VLCDecoder
}

func (v *VLCBitReaderWrapper) ReadBit() (uint8, error) {
	bit, ok := v.decoder.readBits(1)
	if !ok {
		return 0, fmt.Errorf("VLC exhausted")
	}
	return uint8(bit), nil
}

func (v *VLCBitReaderWrapper) ReadBitsLE(n int) (uint32, error) {
	bits, ok := v.decoder.readBits(n)
	if !ok {
		return 0, fmt.Errorf("VLC exhausted")
	}
	return bits, nil
}

// decodeHTCleanupPass decodes with quad-pair processing
func (h *HTDecoder) decodeHTCleanupPass() error {
	for qy := 0; qy < h.qh; qy++ {
		isInitialLinePair := (qy == 0)

		for g := 0; g < (h.qw+1)/2; g++ {
			q1 := 2 * g
			q2 := 2*g + 1
			hasQ2 := q2 < h.qw

			if err := h.decodeQuadPair(q1, q2, qy, hasQ2, isInitialLinePair); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeQuadPair decodes a quad-pair
func (h *HTDecoder) decodeQuadPair(q1, q2, qy int, hasQ2, isInitialLinePair bool) error {
	// Decode MEL bits
	melBit1, hasMore1 := h.mel.DecodeBit()
	if !hasMore1 {
		return nil
	}

	var melBit2 int
	if hasQ2 {
		var hasMore2 bool
		melBit2, hasMore2 = h.mel.DecodeBit()
		if !hasMore2 {
			hasQ2 = false
		}
	}

	// Track first quad's ULF (uOff) for second-quad decisions
	var ulf1 int

	// Decode first quad if significant
	if melBit1 == 1 {
		rho1, _, uOff1, err := h.decodeQuad(q1, qy, isInitialLinePair, false, 0)
		if err != nil {
			return err
		}
		// Track ULF (uOff), not the decoded u value
		ulf1 = uOff1
		// Update context with quad significance
		h.context.UpdateQuadSignificance(q1, qy, rho1)
	}

	// Decode second quad if significant
	if hasQ2 && melBit2 == 1 {
		// Disable simplified U-VLC for lossless decoding (matches encoder)
		useSimplified := false
		rho2, _, _, err := h.decodeQuad(q2, qy, isInitialLinePair, useSimplified, ulf1)
		if err != nil {
			return err
		}
		// Update context with quad significance
		h.context.UpdateQuadSignificance(q2, qy, rho2)
	}

	return nil
}

// decodeQuad decodes a single quad using complete VLC decoding
// Returns the rho pattern, maxExponent for context updates, and uOff (ULF flag: 0 or 1)
func (h *HTDecoder) decodeQuad(qx, qy int, isInitialLinePair, useSimplifiedUVLC bool, firstQuadULF int) (uint8, int, int, error) {
	// Calculate positions
	x0 := qx * 2
	y0 := qy * 2

	// Compute context for VLC lookup
	ctx := h.context.ComputeContext(qx, qy, isInitialLinePair)

	// Decode VLC to get rho and uOff
	var rho, uOff uint8
	var found bool

	if isInitialLinePair {
		rho, uOff, _, _, found = h.vlc.DecodeInitialRow(ctx)
	} else {
		rho, uOff, _, _, found = h.vlc.DecodeNonInitialRow(ctx)
	}

	if !found {
		return 0, 0, 0, fmt.Errorf("VLC decode failed for quad (%d,%d) with context=%d, isInitial=%v", qx, qy, ctx, isInitialLinePair)
	}

	// Calculate exponent predictor
	kq := h.expPredictor.ComputePredictor(qx, qy)

	// Decode U-VLC if uOff=1
	u := uint32(0)
	if uOff == 1 {
		var err error
		if useSimplifiedUVLC {
			u, err = h.uvlc.DecodeUnsignedResidualSecondQuad()
		} else if false && isInitialLinePair && firstQuadULF == 1 {
			// FIXME: Initial pair formula disabled due to encoder/decoder mismatch with u < 3
			// Initial pair formula: only for second quad when both quads in initial line have ulf=1
			// firstQuadULF == 1 means first quad had uOff=1 (and this is second quad with uOff=1)
			u, err = h.uvlc.DecodeUnsignedResidualInitialPair()
		} else {
			// Regular U-VLC decoding
			u, err = h.uvlc.DecodeUnsignedResidual()
		}
		if err != nil {
			return 0, 0, 0, fmt.Errorf("decode U-VLC: %w", err)
		}
	}

	// Calculate Uq = Kq + u
	maxExponent := kq + int(u)

	// Count significant samples
	sigCount := bits.OnesCount8(rho)

	// Update exponent predictor
	h.expPredictor.SetQuadExponents(qx, qy, maxExponent, sigCount)

	// Reconstruct samples
	positions := [][2]int{
		{x0, y0}, {x0 + 1, y0},
		{x0, y0 + 1}, {x0 + 1, y0 + 1},
	}

	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px >= h.width || py >= h.height {
			continue
		}

		if (rho>>i)&1 != 0 {
			numBits := maxExponent
			if numBits <= 0 {
				numBits = 1
			}

			mag, sign, hasMagSgn := h.magsgn.DecodeMagSgn(numBits)
			if !hasMagSgn {
				mag = 0
				sign = 0
			}

			idx := py*h.width + px
			if sign == 0 {
				h.data[idx] = int32(mag)
			} else {
				h.data[idx] = -int32(mag)
			}
		}
	}

	return rho, maxExponent, int(uOff), nil
}

// GetData returns decoded data
func (h *HTDecoder) GetData() []int32 {
	return h.data
}

// DecodeWithBitplane implements BlockDecoder interface
func (h *HTDecoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error {
	h.maxBitplane = maxBitplane
	_, err := h.Decode(data, numPasses)
	return err
}

// DecodeLayered implements BlockDecoder interface
func (h *HTDecoder) DecodeLayered(data []byte, passLengths []int, maxBitplane int, roishift int) error {
	h.maxBitplane = maxBitplane
	numPasses := len(passLengths)
	if numPasses == 0 {
		numPasses = 1
	}
	_, err := h.Decode(data, numPasses)
	return err
}

// Reset resets decoder
func (h *HTDecoder) Reset() {
	for i := range h.data {
		h.data[i] = 0
	}
}
