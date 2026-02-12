package htj2k

import (
	"encoding/binary"
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

	// Encoders for three segments
	magsgn *MagSgnEncoder
	mel    *MELEncoder
	vlc    *VLCEncoder
	uvlc   *UVLCEncoder

	// Exponent predictor
	expPred *ExponentPredictorComputer

	// Encoding state
	roishift int

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

	enc := &HTEncoder{
		width:   width,
		height:  height,
		qw:      qw,
		qh:      qh,
		magsgn:  NewMagSgnEncoder(),
		mel:     NewMELEncoder(),
		vlc:     vlcEnc,
		uvlc:    uvlcEnc,
		expPred: NewExponentPredictorComputer(qw, qh),
	}

	return enc
}

// Encode encodes a code-block using HTJ2K HT cleanup pass
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
func (h *HTEncoder) Encode(data []int32, numPasses int, roishift int) ([]byte, error) {
	if len(data) != h.width*h.height {
		return nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			h.width*h.height, len(data))
	}

	h.data = data
	h.roishift = roishift

	// Reset encoders
	h.magsgn = NewMagSgnEncoder()
	h.mel = NewMELEncoder()
	h.vlc = NewVLCEncoder()
	h.uvlc = NewUVLCEncoder()
	h.expPred = NewExponentPredictorComputer(h.qw, h.qh)

	// Connect UVLC encoder to VLC stream
	h.uvlc.SetWriter(h.vlc)

	// Create context computer for VLC encoding
	context := NewContextComputer(h.width, h.height)

	// Encode quad pairs row by row
	// Initial row (qy=0) uses different context computation than subsequent rows
	if err := h.encodeInitialRow(context); err != nil {
		return nil, fmt.Errorf("encode initial row: %w", err)
	}

	if err := h.encodeSubsequentRows(context); err != nil {
		return nil, fmt.Errorf("encode subsequent rows: %w", err)
	}

	// Assemble final codeblock
	return h.assembleCodel(), nil
}

// encodeInitialRow encodes the first row of quads (qy=0)
// Initial row uses table0 (VLC table for first row) and simpler context computation
// Reference: OpenJPH ojph_block_encoder.cpp lines 597-805
func (h *HTEncoder) encodeInitialRow(context *ContextComputer) error {
	if h.qh == 0 {
		return nil // No rows to encode
	}

	qy := 0
	// Process quads in horizontal pairs
	for qx := 0; qx < h.qw; qx += 2 {
		// Use EncodeQuadPair from vlc_encoder_enhanced.go
		if err := EncodeQuadPair(qx, qy, h.data, h.width, context, h.vlc, h.mel, h.magsgn, h.expPred, h.uvlc); err != nil {
			return fmt.Errorf("encode quad pair (%d,%d): %w", qx, qy, err)
		}
	}

	return nil
}

// encodeSubsequentRows encodes all rows after the initial row (qy>0)
// Subsequent rows use table1 and full context computation with neighbor quads
// Reference: OpenJPH ojph_block_encoder.cpp lines 809-1014
func (h *HTEncoder) encodeSubsequentRows(context *ContextComputer) error {
	if h.qh <= 1 {
		return nil // Only initial row exists
	}

	// Process each subsequent row
	for qy := 1; qy < h.qh; qy++ {
		// Process quads in horizontal pairs
		for qx := 0; qx < h.qw; qx += 2 {
			// Use EncodeQuadPair from vlc_encoder_enhanced.go
			if err := EncodeQuadPair(qx, qy, h.data, h.width, context, h.vlc, h.mel, h.magsgn, h.expPred, h.uvlc); err != nil {
				return fmt.Errorf("encode quad pair (%d,%d): %w", qx, qy, err)
			}
		}
	}

	return nil
}

// QuadInfo holds encoding information for a single quad
// Reference: ITU-T T.814 Annex C
type QuadInfo struct {
	Qx, Qy      int      // Quad position
	Samples     [4]int32 // Sample values [TL, TR, BL, BR]
	Significant [4]bool  // Significance flags
	Rho         uint8    // Significance pattern (0-15)
	EQ          [4]int   // Exponent for each sample
	MaxE        int      // Maximum exponent in quad
	Eps         uint8    // Exponent mask (4 bits)
	SigCount    int      // Number of significant samples
	MelBit      int      // MEL bit (0=all zero, 1=has significant)
}

// preprocessSample computes magnitude, sign, and exponent for a sample
// Reference: OpenJPH ojph_block_encoder.cpp lines 600-650
//
// Formula (ITU-T T.814):
//
//	val_adjusted = val << (p - missing_msbs)  // p = 30 for 32-bit
//	mag = abs(val_adjusted) >> 1
//	e_q = 31 - leading_zeros(mag)
//
// Returns:
//
//	mag: absolute magnitude
//	sign: 0 for positive, 1 for negative
//	eQ: exponent (bit position of MSB)
func (h *HTEncoder) preprocessSample(val int32) (mag uint32, sign int, eQ int) {
	if val == 0 {
		return 0, 0, -1
	}

	if val < 0 {
		mag = uint32(-val)
		sign = 1
	} else {
		mag = uint32(val)
		sign = 0
	}

	// Compute exponent: position of MSB (0-based)
	// e_q = floor(log2(mag)) for mag > 0
	if mag > 0 {
		eQ = bits.Len32(mag) - 1
	} else {
		eQ = -1
	}

	return mag, sign, eQ
}

// assembleCodel assembles the three segments into final codeblock with MEL/VLC fusion
// Reference: OpenJPH ojph_block_encoder.cpp lines 420-446
func (h *HTEncoder) assembleCodel() []byte {
	// Flush encoders with fusion support
	magsgnData := h.magsgn.Flush()
	melData, melLast, melUsed := h.mel.FlushForFusion()
	vlcData, vlcLast, vlcUsed := h.vlc.FlushForFusion()

	// Try to fuse the last bytes of MEL and VLC segments
	// Fusion is possible when:
	// 1. Both segments have data
	// 2. The used bits don't overlap (melUsed + vlcUsed <= 8)
	// 3. The fused byte is not 0xFF (to avoid stuffing issues)
	// 4. VLC has enough remaining space

	finalMEL := melData
	finalVLC := vlcData

	if len(melData) > 0 && len(vlcData) > 0 && melUsed > 0 && vlcUsed > 0 {
		// Calculate remaining bits in each byte
		melRemaining := 8 - melUsed
		vlcSpace := 8 - vlcUsed

		// Check if fusion is possible
		if melRemaining > 0 && vlcSpace > 0 && (melUsed+vlcUsed <= 8) {
			// Try to fuse: OR the two bytes together
			fusedByte := melLast | vlcLast

			// Verify fusion conditions:
			// 1. The bits don't overlap (XOR check)
			// 2. The fused byte is not 0xFF
			melMask := byte((0xFF << melRemaining) & 0xFF)
			vlcMask := byte(0xFF >> (8 - vlcUsed))

			bitsOverlap := ((fusedByte ^ melLast) & melMask) | ((fusedByte ^ vlcLast) & vlcMask)
			canFuse := (bitsOverlap == 0) && (fusedByte != 0xFF)

			if canFuse {
				// Fusion successful: remove last bytes and add fused byte
				if len(melData) > 0 {
					finalMEL = melData[:len(melData)-1]
				}
				if len(vlcData) > 0 {
					finalVLC = vlcData[:len(vlcData)-1]
				}

				// Add the fused byte to MEL segment (or VLC, convention varies)
				finalMEL = append(finalMEL, fusedByte)
			}
		}
	}

	melLen := len(finalMEL)
	vlcLen := len(finalVLC)

	// Assemble final codeblock
	// Layout: [MagSgn][MEL][VLC][melLen(2B)][vlcLen(2B)]
	totalLen := len(magsgnData) + melLen + vlcLen + 4
	result := make([]byte, totalLen)
	pos := 0

	// Copy MagSgn segment
	copy(result[pos:], magsgnData)
	pos += len(magsgnData)

	// Copy MEL segment
	copy(result[pos:], finalMEL)
	pos += melLen

	// Copy VLC segment
	copy(result[pos:], finalVLC)
	pos += vlcLen

	// Footer: 4 bytes total
	// bytes[n-4:n-2] = melLen as uint16 LE
	// bytes[n-2:n] = vlcLen as uint16 LE
	// This supports segment lengths up to 65535 bytes
	binary.LittleEndian.PutUint16(result[pos:pos+2], uint16(melLen))
	binary.LittleEndian.PutUint16(result[pos+2:pos+4], uint16(vlcLen))

	return result
}
