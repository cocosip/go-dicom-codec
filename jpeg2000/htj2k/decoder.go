package htj2k

import (
	"encoding/binary"
	"fmt"
)

// HTDecoder is the HTJ2K block decoder used by the JPEG2000 pipeline.
// It preserves the legacy raw passthrough mode (mel/vlc length == 0).
type HTDecoder struct {
	// Block dimensions
	width  int
	height int

	// Decoders for three segments (legacy simplified path)
	magsgn *MagSgnDecoder
	mel    *MELDecoder
	vlc    *VLCDecoder

	// Decoded data
	data []int32

	// Decoding state
	maxBitplane int

	// Dimensions in quads
	qw int
	qh int

	// Raw passthrough mode (mel/vlc length == 0)
	rawMode bool
}

// NewHTDecoder creates a new HT decoder.
func NewHTDecoder(width, height int) *HTDecoder {
	qw := (width + 1) / 2
	qh := (height + 1) / 2

	return &HTDecoder{
		width:  width,
		height: height,
		qw:     qw,
		qh:     qh,
		data:   make([]int32, width*height),
	}
}

// Decode decodes a HTJ2K code-block.
func (h *HTDecoder) Decode(codeblock []byte, numPasses int) ([]int32, error) {
	if len(codeblock) == 0 {
		return h.data, nil
	}

	if err := h.parseCodeblock(codeblock); err != nil {
		return nil, fmt.Errorf("parse codeblock: %w", err)
	}

	if h.rawMode {
		return h.data, nil
	}

	blockDecoder := NewHTBlockDecoder(h.width, h.height)
	decoded, err := blockDecoder.DecodeBlock(codeblock)
	if err != nil {
		return nil, fmt.Errorf("decode cleanup pass: %w", err)
	}
	h.data = decoded
	return h.data, nil
}

// parseCodeblock parses segments.
func (h *HTDecoder) parseCodeblock(codeblock []byte) error {
	if len(codeblock) < 2 {
		return fmt.Errorf("codeblock too short")
	}

	// Parse 12-bit Scup from last 2 bytes (OpenJPH/ITU-T.814 format)
	// Byte[n-2]: low 4 bits = Scup[3:0], high 4 bits reserved
	// Byte[n-1]: Scup[11:4] (8 bits)
	// Formula: Scup = (byte[n-1] << 4) | (byte[n-2] & 0x0F)
	scupLow := int(codeblock[len(codeblock)-2] & 0x0F)
	scupHigh := int(codeblock[len(codeblock)-1])
	scup := (scupHigh << 4) | scupLow

	lcup := len(codeblock) // Total codeblock length
	magsgnLen := lcup - 2 - scup
	var melLen, vlcLen int

	// Validate Scup (ITU-T.814: 0 or [2, min(Lcup-2, 4079)])
	if scup > 0 {
		if scup < 2 || scup > (lcup-2) || scup > 4079 {
			return fmt.Errorf("invalid Scup: %d (Lcup=%d)", scup, lcup)
		}
		// TODO: Determine MEL/VLC split within Scup
		// For now, assume equal split or parse MEL termination
		// OpenJPH uses dynamic allocation: MEL grows forward, VLC grows backward
		// We need the actual split point from the encoded data
		// Placeholder: assume MEL is first half, VLC is second half
		melLen = scup / 2
		vlcLen = scup - melLen
	} else {
		// Raw mode: Scup=0 means no MEL/VLC segments
		melLen = 0
		vlcLen = 0
	}

	if magsgnLen < 0 || magsgnLen+melLen+vlcLen > lcup {
		return fmt.Errorf("segment lengths exceed codeblock size")
	}

	// HTJ2K Scup (MEL+VLC length) validation (OpenJPEG alignment)
	// ITU-T.814 standard requirement: 2 <= Scup <= min(Lcup, 4079)
	// Exception: Scup=0 is valid for raw mode (MEL=0, VLC=0)
	// NOTE: Validation currently disabled - encoder generates codeblocks that don't comply
	// TODO: Re-enable after encoder is fixed to meet ITU-T.814 requirements
	// scup := melLen + vlcLen
	// lcup := len(codeblock)
	// const maxScup = 4079
	// if scup > 0 { // Only validate if not raw mode
	// 	if scup < 2 || scup > lcup {
	// 		return fmt.Errorf("HTJ2K Scup validation failed: segment length out of bounds")
	// 	}
	// 	if scup > maxScup {
	// 		return fmt.Errorf("HTJ2K Scup validation failed: exceeds maximum 4079 bytes")
	// 	}
	// }

	magsgnData := codeblock[0:magsgnLen]
	melData := codeblock[magsgnLen : magsgnLen+melLen]
	vlcData := codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen]

	// Raw mode: mel/vlc are zero; read int32 coefficients directly.
	if melLen == 0 && vlcLen == 0 {
		h.rawMode = true
		for i := 0; i < h.width*h.height && i*4+3 < len(magsgnData); i++ {
			h.data[i] = int32(binary.LittleEndian.Uint32(magsgnData[i*4:]))
		}
		return nil
	}

	h.rawMode = false
	h.magsgn = NewMagSgnDecoder(magsgnData)
	h.mel = NewMELDecoder(melData)
	h.vlc = NewVLCDecoder(vlcData)

	// Note: MEL unstuffing validation is performed by the MEL decoder itself
	// during initialization and decoding, not here in segment parsing

	return nil
}

// decodeQuad decodes a single quad using the simplified format:
// rho(4bit) + per-sample length (6bit) + MagSgn(numBits).
func (h *HTDecoder) decodeQuad(qx, qy int) error {
	melBit, hasMore := h.mel.DecodeBit()
	if !hasMore {
		return nil
	}
	if melBit == 0 {
		return nil
	}

	rhoBits, ok := h.vlc.readBits(4)
	if !ok {
		return fmt.Errorf("VLC exhausted while reading rho")
	}
	rho := uint8(rhoBits)

	x0 := qx * 2
	y0 := qy * 2
	positions := [][2]int{
		{x0, y0}, {x0, y0 + 1},
		{x0 + 1, y0}, {x0 + 1, y0 + 1},
	}

	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px >= h.width || py >= h.height {
			continue
		}

		if (rho>>i)&1 != 0 {
			lenMinus1, ok := h.vlc.readBits(6)
			if !ok {
				return fmt.Errorf("VLC exhausted while reading length")
			}
			numBits := int(lenMinus1) + 1
			if numBits <= 0 || numBits > 32 {
				return fmt.Errorf("invalid magnitude bit length %d", numBits)
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

	return nil
}

// GetData returns decoded data.
func (h *HTDecoder) GetData() []int32 {
	return h.data
}

// DecodeWithBitplane implements BlockDecoder interface.
func (h *HTDecoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error {
	h.maxBitplane = maxBitplane
	_, err := h.Decode(data, numPasses)
	return err
}

// DecodeLayered implements BlockDecoder interface.
func (h *HTDecoder) DecodeLayered(data []byte, passLengths []int, maxBitplane int, roishift int) error {
	h.maxBitplane = maxBitplane
	numPasses := len(passLengths)
	if numPasses == 0 {
		numPasses = 1
	}
	_, err := h.Decode(data, numPasses)
	return err
}

// Reset resets decoder.
func (h *HTDecoder) Reset() {
	for i := range h.data {
		h.data[i] = 0
	}
}
