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
// params: codeblock - encoded bytes, numPasses - pass count (unused in HT path)
// returns: decoded int32 coefficients and error
func (h *HTDecoder) Decode(codeblock []byte, _ int) ([]int32, error) {
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
// Footer format: 4 bytes - melLen (uint16 LE) + vlcLen (uint16 LE)
// Layout: [MagSgn][MEL][VLC][melLen(2)][vlcLen(2)]
func (h *HTDecoder) parseCodeblock(codeblock []byte) error {
	if len(codeblock) < 4 {
		return fmt.Errorf("codeblock too short")
	}

	lcup := len(codeblock)
	melLen := int(binary.LittleEndian.Uint16(codeblock[lcup-4 : lcup-2]))
	vlcLen := int(binary.LittleEndian.Uint16(codeblock[lcup-2 : lcup]))
	scup := melLen + vlcLen

	magsgnLen := lcup - 4 - scup
	if magsgnLen < 0 {
		return fmt.Errorf("invalid segment lengths")
	}

	magsgnData := codeblock[0:magsgnLen]

	// Raw mode: mel/vlc lengths are both zero; read int32 coefficients directly.
	if scup == 0 {
		h.rawMode = true
		for i := 0; i < h.width*h.height && i*4+3 < len(magsgnData); i++ {
			h.data[i] = int32(binary.LittleEndian.Uint32(magsgnData[i*4:]))
		}
		return nil
	}

	h.rawMode = false
	h.magsgn = NewMagSgnDecoder(magsgnData)
	h.mel = NewMELDecoder(codeblock[magsgnLen : magsgnLen+melLen])
	h.vlc = NewVLCDecoder(codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen])

	return nil
}

// GetData returns decoded data.
func (h *HTDecoder) GetData() []int32 {
	return h.data
}

// DecodeWithBitplane implements BlockDecoder interface.
func (h *HTDecoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, _ int) error {
	h.maxBitplane = maxBitplane
	_, err := h.Decode(data, numPasses)
	return err
}

// DecodeLayered implements BlockDecoder interface.
func (h *HTDecoder) DecodeLayered(data []byte, passLengths []int, maxBitplane int, _ int) error {
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
