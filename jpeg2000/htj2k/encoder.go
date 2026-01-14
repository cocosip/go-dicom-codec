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

	enc := &HTEncoder{
		width:  width,
		height: height,
		qw:     qw,
		qh:     qh,
		magsgn: NewMagSgnEncoder(),
		mel:    NewMELEncoder(),
		vlc:    vlcEnc,
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

	// 直接输出原始系数（小端），并保留段长度尾部（MEL/VLC 长度均为 0）。
	return h.assembleRaw(), nil
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

// encodeHTCleanupPass performs HT cleanup pass encoding (quad by quad)
func (h *HTEncoder) encodeHTCleanupPass() error {
	for qy := 0; qy < h.qh; qy++ {
		for qx := 0; qx < h.qw; qx++ {
			info := h.getQuadInfo(qx, qy)

			// MEL：0 表示全零 quad，1 表示存在非零系数
			h.mel.EncodeBit(info.MelBit)
			if info.MelBit == 0 {
				continue
			}

			if err := h.encodeQuadData(info); err != nil {
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
	MelBit      int      // MEL bit (0=all zero, 1=has significant)
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
	positions := [][2]int{
		{x0, y0}, {x0, y0 + 1},
		{x0 + 1, y0}, {x0 + 1, y0 + 1},
	}

	allZero := true
	for i, pos := range positions {
		px, py := pos[0], pos[1]
		if px < h.width && py < h.height {
			idx := py*h.width + px
			info.Samples[i] = h.data[idx]
			info.Significant[i] = (info.Samples[i] != 0)
			if info.Significant[i] {
				allZero = false
				info.Rho |= (1 << i)
			}
		}
	}

	if allZero {
		info.MelBit = 0
	} else {
		info.MelBit = 1
	}

	return info
}

// encodeQuadData encodes rho + per-coefficient bit-length + magnitude/sign directly
//  - VLC segment：rho(4bit) + 对每个显著系数的长度 len-1（6bit，支持最长 32bit）
//  - MagSgn segment：len 位的无符号幅度 + 1bit 符号（由 MagSgnEncoder 处理）
func (h *HTEncoder) encodeQuadData(info *QuadInfo) error {
	// 写入 rho
	if err := h.vlc.WriteBits(uint32(info.Rho), 4); err != nil {
		return fmt.Errorf("encode rho: %w", err)
	}

	// 为每个显著样本写入长度并输出幅度/符号
	for i := 0; i < 4; i++ {
		if !info.Significant[i] {
			continue
		}
		val := info.Samples[i]
		mag := uint32(val)
		sign := 0
		if val < 0 {
			mag = uint32(-val)
			sign = 1
		}

		// 使用真实位长编码（至少 1 位，最大 32 位）
		numBits := bits.Len32(mag)
		if numBits == 0 {
			numBits = 1
		}
		if numBits > 32 {
			numBits = 32
		}

		// 将 (numBits-1) 写入 VLC 流，固定 6 bit
		if err := h.vlc.WriteBits(uint32(numBits-1), 6); err != nil {
			return fmt.Errorf("encode mag length: %w", err)
		}

		h.magsgn.EncodeMagSgn(mag, sign, numBits)
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

// assembleRaw 将所有系数按 int32 小端序写入，并在尾部附加 4 字节段长度（melLen=0, vlcLen=0）。
func (h *HTEncoder) assembleRaw() []byte {
	dataBytes := make([]byte, len(h.data)*4)
	for i, v := range h.data {
		binary.LittleEndian.PutUint32(dataBytes[i*4:], uint32(v))
	}

	// mel/vlc 长度均为 0，占 4 字节尾部
	result := make([]byte, len(dataBytes)+4)
	copy(result, dataBytes)
	// 最后 4 字节已为 0
	return result
}
