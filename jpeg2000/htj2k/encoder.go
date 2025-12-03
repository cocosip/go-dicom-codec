package htj2k

import (
	"fmt"
	"math"
)

// HTEncoder implements the HTJ2K High-Throughput block encoder
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019
//
// HTJ2K replaces the standard EBCOT Tier-1 encoder with a high-throughput
// block coder that processes samples in 2x2 quads using three entropy coding tools:
// 1. MagSgn - Magnitude and sign bits
// 2. MEL - Adaptive run-length coding for quad significance
// 3. VLC - Variable-length coding for sample patterns

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

	// Encoding state
	maxBitplane int
	roishift    int
}

// NewHTEncoder creates a new HT block encoder
func NewHTEncoder(width, height int) *HTEncoder {
	return &HTEncoder{
		width:  width,
		height: height,
		magsgn: NewMagSgnEncoder(),
		mel:    NewMELEncoder(),
		vlc:    NewVLCEncoder(),
	}
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

	// Encode using HT cleanup pass
	if err := h.encodeHTCleanup(); err != nil {
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

	// Calculate bitplane
	bitplane := 0
	for maxVal > 0 {
		maxVal >>= 1
		bitplane++
	}

	return bitplane - 1
}

// encodeHTCleanup performs the HT cleanup pass encoding
func (h *HTEncoder) encodeHTCleanup() error {
	// Process code-block in 2x2 quads (quad-based scanning)
	for y := 0; y < h.height; y += 2 {
		for x := 0; x < h.width; x += 2 {
			if err := h.encodeQuad(x, y); err != nil {
				return err
			}
		}
	}

	return nil
}

// encodeQuad encodes a single 2x2 quad
func (h *HTEncoder) encodeQuad(x, y int) error {
	// Collect 4 samples in quad (handle boundaries)
	quad := make([]int32, 4)
	quadSig := uint8(0)  // Significance pattern (4 bits)

	// Sample order in quad: [0,1]
	//                       [2,3]
	positions := [][2]int{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}}

	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px < h.width && py < h.height {
			idx := py*h.width + px
			quad[i] = h.data[idx]

			// Check if significant at current bitplane
			if quad[i] != 0 {
				quadSig |= (1 << i)
			}
		}
	}

	// Encode quad significance using MEL
	// If all samples are zero, encode as run continuation (0)
	// Otherwise, encode as run termination (1)
	allZero := (quadSig == 0)
	if allZero {
		h.mel.EncodeBit(0) // Continue run
	} else {
		h.mel.EncodeBit(1) // End run

		// Encode significance pattern and magnitudes using VLC
		mags := make([]int, 0, 4)
		for i := 0; i < 4; i++ {
			if (quadSig>>i)&1 != 0 {
				// Sample is significant
				mag := quad[i]
				if mag < 0 {
					mag = -mag
				}
				mags = append(mags, int(mag))
			}
		}
		h.vlc.EncodeQuad(quadSig, mags)

		// Encode magnitude and sign bits using MagSgn
		for i := 0; i < 4; i++ {
			if (quadSig>>i)&1 != 0 {
				val := quad[i]
				mag := val
				sign := 0
				if val < 0 {
					mag = -val
					sign = 1
				}

				// Encode magnitude bits (numBits determined by bitplane)
				numBits := h.maxBitplane + 1
				h.magsgn.EncodeMagSgn(uint32(mag), sign, numBits)
			}
		}
	}

	return nil
}

// assembleCodel assembles the three segments (MagSgn, MEL, VLC) into final codeblock
func (h *HTEncoder) assembleCodel() []byte {
	// Flush all encoders
	magsgnData := h.magsgn.Flush()
	melData := h.mel.Flush()
	vlcData := h.vlc.Flush()

	// HTJ2K segment layout:
	// [MagSgn segment (grows forward)] [MEL segment (grows forward)] [VLC segment (grows backward)]
	//
	// The last 2 bytes of the segment encode the length of MEL and VLC segments

	// Calculate total size
	melLen := len(melData)
	vlcLen := len(vlcData)

	// Assemble: MagSgn + MEL + VLC + Length(2 bytes)
	totalLen := len(magsgnData) + melLen + vlcLen + 2

	result := make([]byte, totalLen)
	pos := 0

	// Copy MagSgn segment (grows forward)
	copy(result[pos:], magsgnData)
	pos += len(magsgnData)

	// Copy MEL segment (grows forward)
	copy(result[pos:], melData)
	pos += melLen

	// Copy VLC segment
	copy(result[pos:], vlcData)
	pos += vlcLen

	// Encode lengths in last 2 bytes
	// Format: [MEL_len (1 byte)] [VLC_len (1 byte)]
	// For larger blocks, this may need more sophisticated encoding
	if melLen > 255 || vlcLen > 255 {
		// Use extended format (not fully implemented here)
		result[pos] = byte(melLen & 0xFF)
		result[pos+1] = byte(vlcLen & 0xFF)
	} else {
		result[pos] = byte(melLen)
		result[pos+1] = byte(vlcLen)
	}

	return result
}

// GetSegmentLengths returns the lengths of the three segments (for debugging)
func (h *HTEncoder) GetSegmentLengths() (int, int, int) {
	return h.magsgn.Length(), h.mel.Length(), h.vlc.Length()
}

// Utility functions

// abs returns absolute value of int32
func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// bitplane returns the bitplane index for a value
func bitplane(x int32) int {
	if x == 0 {
		return -1
	}

	val := abs(x)
	bp := 0
	for val > 0 {
		val >>= 1
		bp++
	}
	return bp - 1
}

// log2 returns the floor of log2(x)
func log2(x uint32) int {
	if x == 0 {
		return -1
	}
	return int(math.Floor(math.Log2(float64(x))))
}
