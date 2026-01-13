package htj2k

import (
	"fmt"
	"math/bits"
)

// HTEncoder implements the HTJ2K High-Throughput block encoder
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// HTJ2K replaces the standard EBCOT Tier-1 encoder with a high-throughput
// block coder that processes samples in 2x2 quads using three entropy coding tools:
// 1. MagSgn - Magnitude and sign bits
// 2. MEL - Adaptive run-length coding for quad significance
// 3. VLC - Variable-length coding for sample patterns with context and U-VLC

type HTEncoder struct {
	// Block dimensions
	width  int
	height int

	// Input data
	data []int32 // Wavelet coefficients

	// Significance map (tracks which samples are significant)
	sigMap [][]bool

	// Encoders for three segments
	magsgn *MagSgnEncoder
	mel    *MELEncoder
	vlc    *VLCEncoder
	uvlc   *UVLCEncoder

	// Context computer
	context *ContextComputer

	// Exponent predictor
	expPredictor *ExponentPredictorComputer

	// Encoding state
	maxBitplane int
	roishift    int

	// Dimensions in quads
	qw int // width in quads
	qh int // height in quads
}

// NewHTEncoder creates a new HT block encoder
func NewHTEncoder(width, height int) *HTEncoder {
	qw := (width + 1) / 2
	qh := (height + 1) / 2

	vlcEnc := NewVLCEncoder()
	uvlcEnc := NewUVLCEncoder()

	// Connect U-VLC encoder to VLC encoder (they share the same bit stream)
	uvlcEnc.SetWriter(vlcEnc)

	enc := &HTEncoder{
		width:        width,
		height:       height,
		qw:           qw,
		qh:           qh,
		sigMap:       make([][]bool, height),
		magsgn:       NewMagSgnEncoder(),
		mel:          NewMELEncoder(),
		vlc:          vlcEnc,
		uvlc:         uvlcEnc,
		context:      NewContextComputer(width, height),
		expPredictor: NewExponentPredictorComputer(qw, qh),
	}

	// Initialize significance map
	for y := 0; y < height; y++ {
		enc.sigMap[y] = make([]bool, width)
	}

	return enc
}

// Encode encodes a code-block using HTJ2K HT cleanup pass
func (h *HTEncoder) Encode(data []int32, numPasses int, roishift int) ([]byte, error) {
	if len(data) != h.width*h.height {
		return nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			h.width*h.height, len(data))
	}

	h.data = data
	h.roishift = roishift

	// Find maximum bitplane
	h.maxBitplane = h.findMaxBitplane()
	if h.maxBitplane < 0 {
		// All coefficients are zero - return empty block
		return []byte{}, nil
	}

	// Build significance map
	h.buildSignificanceMap()

	// Encode using HT cleanup pass with quad-pair processing
	if err := h.encodeHTCleanupPass(); err != nil {
		return nil, err
	}

	// Assemble the three segments into final codeblock
	result := h.assembleCodel()

	return result, nil
}

// findMaxBitplane finds the maximum bitplane with non-zero coefficients
func (h *HTEncoder) findMaxBitplane() int {
	maxVal := int32(0)
	for _, v := range h.data {
		abs := v
		if abs < 0 {
			abs = -abs
		}
		if abs > maxVal {
			maxVal = abs
		}
	}

	if maxVal == 0 {
		return -1
	}

	// Calculate bitplane (position of most significant bit)
	return bits.Len32(uint32(maxVal)) - 1
}

// buildSignificanceMap builds the significance map from input data
func (h *HTEncoder) buildSignificanceMap() {
	for y := 0; y < h.height; y++ {
		for x := 0; x < h.width; x++ {
			idx := y*h.width + x
			h.sigMap[y][x] = (h.data[idx] != 0)
		}
	}
}

// encodeHTCleanupPass performs HT cleanup pass encoding with quad-pair processing
func (h *HTEncoder) encodeHTCleanupPass() error {
	// Process code-block in quad-pairs (row by row, pair by pair)
	for qy := 0; qy < h.qh; qy++ {
		isInitialLinePair := (qy == 0)

		for g := 0; g < (h.qw+1)/2; g++ {
			// Quad indices: q1 = 2*g, q2 = 2*g+1
			q1 := 2 * g
			q2 := 2*g + 1

			// Check if second quad exists (handle odd width)
			hasQ2 := q2 < h.qw

			// Encode this quad-pair
			if err := h.encodeQuadPair(q1, q2, qy, hasQ2, isInitialLinePair); err != nil {
				return err
			}
		}
	}

	return nil
}

// QuadInfo holds encoding information for a single quad
type QuadInfo struct {
	Qx, Qy      int      // Quad position
	Samples     [4]int32 // Sample values
	Significant [4]bool  // Significance flags
	Rho         uint8    // Significance pattern (0-15)
	Context     uint8    // VLC context index
	Kq          int      // Exponent predictor
	Uq          int      // Exponent bound
	ULF         uint8    // ULF flag (0 or 1)
	EK          uint8    // EMB parameter
	E1          uint8    // EMB parameter
	MelBit      int      // MEL bit (0=all zero, 1=has significant)
}

// encodeQuadPair encodes a pair of horizontally adjacent quads
// FIXED: Now processes quads sequentially with context updates between them
func (h *HTEncoder) encodeQuadPair(q1, q2, qy int, hasQ2, isInitialLinePair bool) error {
	// Get quad1 information
	info1 := h.getQuadInfo(q1, qy)

	// Encode MEL bit for quad1
	h.mel.EncodeBit(info1.MelBit)

	// Encode quad1 VLC/UVLC/MagSgn if significant
	if info1.MelBit == 1 {
		// First quad: no previous quad, so firstQuadULF=0
		if err := h.encodeQuadData(info1, isInitialLinePair, false, 0); err != nil {
			return err
		}
		// Update context with quad1 significance
		h.context.UpdateQuadSignificance(q1, qy, info1.Rho)
	}

	// Now get quad2 information (AFTER quad1 context update)
	if hasQ2 {
		info2 := h.getQuadInfo(q2, qy)

		// Encode MEL bit for quad2
		h.mel.EncodeBit(info2.MelBit)

		// Encode quad2 VLC/UVLC/MagSgn if significant
		if info2.MelBit == 1 {
			// Pass first quad's ULF for initial pair formula decision
			if err := h.encodeQuadData(info2, isInitialLinePair, false, int(info1.ULF)); err != nil {
				return err
			}
			// Update context with quad2 significance
			h.context.UpdateQuadSignificance(q2, qy, info2.Rho)
		}
	}

	return nil
}

// getQuadInfo extracts encoding information for a single quad
func (h *HTEncoder) getQuadInfo(qx, qy int) *QuadInfo {
	info := &QuadInfo{
		Qx: qx,
		Qy: qy,
	}

	// Quad top-left position in samples
	x0 := qx * 2
	y0 := qy * 2

	// Collect 4 samples in quad
	// OpenJPH sample ordering (column-major within 2x2 quad):
	// bit 0: (x0, y0)     - left column, top row
	// bit 1: (x0, y0+1)   - left column, bottom row
	// bit 2: (x0+1, y0)   - right column, top row
	// bit 3: (x0+1, y0+1) - right column, bottom row
	positions := [][2]int{
		{x0, y0}, {x0, y0 + 1},
		{x0 + 1, y0}, {x0 + 1, y0 + 1},
	}

	allZero := true
	maxMag := int32(0)
	sigCount := 0

	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px < h.width && py < h.height {
			idx := py*h.width + px
			info.Samples[i] = h.data[idx]
			info.Significant[i] = (info.Samples[i] != 0)

			if info.Significant[i] {
				allZero = false
				sigCount++

				// Build rho pattern
				info.Rho |= (1 << i)

				// Track max magnitude
				mag := info.Samples[i]
				if mag < 0 {
					mag = -mag
				}
				if mag > maxMag {
					maxMag = mag
				}
			}
		}
	}

	// Set MEL bit
	if allZero {
		info.MelBit = 0
		return info
	}
	info.MelBit = 1

	// Calculate context
	info.Context = h.context.ComputeContext(qx, qy, qy == 0)

	// Calculate exponent predictor Kq
	info.Kq = h.expPredictor.ComputePredictor(qx, qy)

	// Calculate exponent bound Uq
	if maxMag > 0 {
		info.Uq = bits.Len32(uint32(maxMag))
	}

	// Calculate U = Uq - Kq (with clipping)
	u := info.Uq - info.Kq
	if u < 0 {
		u = 0
	}

	// ULF flag
	if u > 0 {
		info.ULF = 1
	}

	// IMPORTANT: When u is clipped to 0, the actual exponent bound is Kq, not Uq!
	// The decoder will compute maxExponent = Kq + u, and we must encode with that many bits
	// So update Uq to be max(Uq, Kq) for consistency with decoder
	if info.Uq < info.Kq {
		info.Uq = info.Kq
	}

	// Update exponent predictor with Uq and significance count
	h.expPredictor.SetQuadExponents(qx, qy, info.Uq, sigCount)

	// Compute ek/e1 from magnitude bit patterns for significant samples
	// IMPORTANT: Use adjusted Uq for ek/e1 computation
	var ek uint8 = 0
	var e1 uint8 = 0
	if info.Uq > 0 {
		msbPos := info.Uq - 1
		for i := 0; i < 4; i++ {
			if info.Significant[i] {
				val := info.Samples[i]
				if val < 0 {
					val = -val
				}
				mag := uint32(val)
				if ((mag >> uint(msbPos)) & 1) == 1 {
					ek |= (1 << i)
				}
				if (mag & 1) == 1 {
					e1 |= (1 << i)
				}
			}
		}
	}
	info.EK = ek
	info.E1 = e1

	return info
}

// encodeQuadData encodes VLC, U-VLC, and MagSgn data for a quad
// useSimplifiedUVLC: if provided and true, use simplified U-VLC (1 bit)
// firstQuadULF: ULF of first quad in pair (used for initial pair formula decision)
func (h *HTEncoder) encodeQuadData(info *QuadInfo, isInitialLinePair bool, useSimplifiedUVLC bool, firstQuadULF int) error {
	// Encode CxtVLC
	if err := h.vlc.EncodeCxtVLC(info.Context, info.Rho, info.ULF, info.EK, info.E1, isInitialLinePair); err != nil {
		return fmt.Errorf("encode CxtVLC: %w", err)
	}

	// Encode U-VLC if ULF=1
	if info.ULF == 1 {
		u := info.Uq - info.Kq
		if u < 0 {
			u = 0
		}

		// Encode U-VLC (may use simplified or regular encoding)
		if useSimplifiedUVLC {
			// Simplified coding only supports values 1 or 2
			if u < 1 {
				u = 1
			} else if u > 2 {
				u = 2
			}
			if err := h.uvlc.EncodeUVLCSimplified(u); err != nil {
				return fmt.Errorf("encode simplified U-VLC: %w", err)
			}
		} else {
			// 启用初始行对公式（公式 4）：当处于初始行对且前一 quad ULF=1 时，对 u 应用减 2 的偏置编码。
			useInitialPairFormula := isInitialLinePair && firstQuadULF == 1 && u >= 2
			if err := h.uvlc.EncodeUVLC(u, useInitialPairFormula); err != nil {
				return fmt.Errorf("encode U-VLC: %w", err)
			}
		}
	}

	// Encode MagSgn for each significant sample
	for i := 0; i < 4; i++ {
		if info.Significant[i] {
			val := info.Samples[i]
			mag := uint32(val)
			sign := 0
			if val < 0 {
				mag = uint32(-val)
				sign = 1
			}

			// Number of magnitude bits to encode
			numBits := info.Uq
			if numBits > 0 {
				h.magsgn.EncodeMagSgn(mag, sign, numBits)
			}
		}
	}

	return nil
}

// assembleCodel assembles the three segments into final codeblock
func (h *HTEncoder) assembleCodel() []byte {
	// Flush all encoders
	magsgnData := h.magsgn.Flush()
	melData := h.mel.Flush()
	vlcData := h.vlc.Flush()

	// HTJ2K segment layout (ISO/IEC 15444-15:2019):
	// [MagSgn][MEL][VLC][Lengths(2 bytes)]
	//
	// Last 2 bytes encode segment lengths:
	//   - Byte[n-2]: MEL length
	//   - Byte[n-1]: VLC length

	melLen := len(melData)
	vlcLen := len(vlcData)

	// Use 2-byte length fields for MEL and VLC to avoid overflow on larger blocks.
	totalLen := len(magsgnData) + melLen + vlcLen + 4
	result := make([]byte, totalLen)
	pos := 0

	// Copy MagSgn segment
	copy(result[pos:], magsgnData)
	pos += len(magsgnData)

	// Copy MEL segment
	copy(result[pos:], melData)
	pos += melLen

	// Copy VLC segment
	copy(result[pos:], vlcData)
	pos += vlcLen

	// Encode segment lengths (big-endian uint16)
	result[pos] = byte((melLen >> 8) & 0xFF)
	result[pos+1] = byte(melLen & 0xFF)
	result[pos+2] = byte((vlcLen >> 8) & 0xFF)
	result[pos+3] = byte(vlcLen & 0xFF)

	return result
}
