package htj2k

import (
	"encoding/binary"
	"fmt"
)

// HTDecoder 实现与 HTEncoder 对应的简化解码：
// - VLC 流只包含 rho 与每个显著系数的位长信息
// - MagSgn 流保存实际幅度与符号
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

	// Dimensions in quads
	qw int
	qh int

	// Raw passthrough mode (mel/vlc length == 0)
	rawMode bool
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

	// Prefer 2-byte footer (melLen, vlcLen) to support large segments; fall back to legacy 1-byte footer.
	var (
		melLen, vlcLen int
		magsgnLen      int
		ok             bool
	)

	if len(codeblock) >= 4 {
		melLen = int(binary.BigEndian.Uint16(codeblock[len(codeblock)-4 : len(codeblock)-2]))
		vlcLen = int(binary.BigEndian.Uint16(codeblock[len(codeblock)-2:]))
		totalDataLen := len(codeblock) - 4
		magsgnLen = totalDataLen - melLen - vlcLen
		if melLen >= 0 && vlcLen >= 0 && magsgnLen >= 0 {
			ok = true
		}
	}

	if !ok {
		melLen = int(codeblock[len(codeblock)-2])
		vlcLen = int(codeblock[len(codeblock)-1])
		totalDataLen := len(codeblock) - 2
		magsgnLen = totalDataLen - melLen - vlcLen
		if melLen < 0 || vlcLen < 0 || magsgnLen < 0 {
			return fmt.Errorf("invalid segment lengths")
		}
	}

	if magsgnLen+melLen+vlcLen > len(codeblock) {
		return fmt.Errorf("segment lengths exceed codeblock size")
	}

	magsgnData := codeblock[0:magsgnLen]
	melData := codeblock[magsgnLen : magsgnLen+melLen]
	vlcData := codeblock[magsgnLen+melLen : magsgnLen+melLen+vlcLen]

	// Raw 模式：mel/vlc 均为 0，直接按 int32 小端读回
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

	return nil
}

// decodeHTCleanupPass decodes quads in raster order
func (h *HTDecoder) decodeHTCleanupPass() error {
	if h.rawMode {
		// 已在 parseCodeblock 中完成填充
		return nil
	}
	for qy := 0; qy < h.qh; qy++ {
		for qx := 0; qx < h.qw; qx++ {
			if err := h.decodeQuad(qx, qy); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeQuad decodes a single quad using simplified format:
//  rho(4bit) + 每个显著样本的长度(6bit) + MagSgn(numBits)
func (h *HTDecoder) decodeQuad(qx, qy int) error {
	// MEL 流耗尽则视为剩余全零
	melBit, hasMore := h.mel.DecodeBit()
	if !hasMore {
		return nil
	}
	if melBit == 0 {
		return nil
	}

	// 读取 rho
	rhoBits, ok := h.vlc.readBits(4)
	if !ok {
		return fmt.Errorf("VLC exhausted while reading rho")
	}
	rho := uint8(rhoBits)

	// 计算样本坐标（列主序）
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
			// 读取长度（6bit）
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
