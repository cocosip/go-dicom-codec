package htj2k

import (
	"fmt"
)

// HTDecoder implements the HTJ2K High-Throughput block decoder
// Reference: ITU-T T.814 | ISO/IEC 15444-15:2019

type HTDecoder struct {
	// Block dimensions
	width  int
	height int

	// Decoders for three segments
	magsgn *MagSgnDecoder
	mel    *MELDecoder
	vlc    *VLCDecoder

	// Decoded data
	data []int32

	// Decoding state
	maxBitplane int
}

// NewHTDecoder creates a new HT block decoder
func NewHTDecoder(width, height int) *HTDecoder {
	return &HTDecoder{
		width:  width,
		height: height,
		data:   make([]int32, width*height),
	}
}

// Decode decodes a HTJ2K code-block
func (h *HTDecoder) Decode(codeblock []byte, numPasses int) ([]int32, error) {
	if len(codeblock) == 0 {
		// Empty codeblock - all zeros
		return h.data, nil
	}

	// Parse the codeblock to extract three segments
	if err := h.parseCodeblock(codeblock); err != nil {
		return nil, fmt.Errorf("failed to parse codeblock: %w", err)
	}

	// Decode using HT cleanup pass
	if err := h.decodeHTCleanup(); err != nil {
		return nil, fmt.Errorf("failed to decode HT cleanup: %w", err)
	}

	return h.data, nil
}

// parseCodeblock parses the codeblock and initializes segment decoders
func (h *HTDecoder) parseCodeblock(codeblock []byte) error {
	if len(codeblock) < 2 {
		return fmt.Errorf("codeblock too short: %d bytes", len(codeblock))
	}

	// Extract lengths from last 2 bytes
	melLen := int(codeblock[len(codeblock)-2])
	vlcLen := int(codeblock[len(codeblock)-1])

	// Calculate segment boundaries
	// Layout: [MagSgn] [MEL] [VLC] [Lengths(2)]
	totalDataLen := len(codeblock) - 2
	magsgnLen := totalDataLen - melLen - vlcLen

	if magsgnLen < 0 || melLen < 0 || vlcLen < 0 {
		return fmt.Errorf("invalid segment lengths: magsgn=%d, mel=%d, vlc=%d",
			magsgnLen, melLen, vlcLen)
	}

	// Extract segments
	magsgnData := codeblock[0:magsgnLen]
	melData := codeblock[magsgnLen : magsgnLen+melLen]
	vlcData := codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen]

	// Initialize decoders
	h.magsgn = NewMagSgnDecoder(magsgnData)
	h.mel = NewMELDecoder(melData)
	h.vlc = NewVLCDecoder(vlcData)

	return nil
}

// decodeHTCleanup performs HT cleanup pass decoding
func (h *HTDecoder) decodeHTCleanup() error {
	// Process code-block in 2x2 quads
	for y := 0; y < h.height; y += 2 {
		for x := 0; x < h.width; x += 2 {
			if err := h.decodeQuad(x, y); err != nil {
				return err
			}
		}
	}

	return nil
}

// decodeQuad decodes a single 2x2 quad
func (h *HTDecoder) decodeQuad(x, y int) error {
	// Decode MEL bit to determine if quad has significant samples
	melBit, hasMore := h.mel.DecodeBit()
	if !hasMore {
		// End of MEL data, remaining quads are zero
		return nil
	}

	if melBit == 0 {
		// Run continuation - all samples in quad are zero
		// (already initialized to zero in h.data)
		return nil
	}

	// melBit == 1: Run termination - quad has significant samples

	// Decode VLC to get significance pattern and magnitudes
	quadSig, mags, hasVLC := h.vlc.DecodeQuad()
	if !hasVLC {
		// VLC decoder exhausted - this shouldn't happen in valid stream
		// Treat remaining samples as zero
		return nil
	}

	// Decode MagSgn for each significant sample
	magIdx := 0
	positions := [][2]int{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}}

	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px >= h.width || py >= h.height {
			continue
		}

		if (quadSig>>i)&1 != 0 {
			// Sample is significant - decode magnitude and sign
			if magIdx >= len(mags) {
				return fmt.Errorf("VLC magnitude count mismatch at quad (%d,%d)", x, y)
			}

			// Decode full magnitude and sign from MagSgn
			numBits := bitlen(uint32(mags[magIdx]))
			mag, sign, hasMagSgn := h.magsgn.DecodeMagSgn(numBits)
			if !hasMagSgn {
				// MagSgn exhausted - use VLC magnitude as fallback
				mag = uint32(mags[magIdx])
				sign = 0
			}

			// Reconstruct signed value
			idx := py*h.width + px
			if sign == 0 {
				h.data[idx] = int32(mag)
			} else {
				h.data[idx] = -int32(mag)
			}

			magIdx++
		}
	}

	return nil
}

// GetData returns the decoded coefficient data
func (h *HTDecoder) GetData() []int32 {
	return h.data
}

// Reset resets the decoder for reuse
func (h *HTDecoder) Reset() {
	for i := range h.data {
		h.data[i] = 0
	}
}

// Utility functions

// bitlen returns the number of bits needed to represent a value
func bitlen(x uint32) int {
	if x == 0 {
		return 1
	}

	count := 0
	for x > 0 {
		x >>= 1
		count++
	}
	return count
}
