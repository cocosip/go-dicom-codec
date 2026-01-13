package htj2k

import "fmt"

// UVLCDecoder implements U-VLC (Unsigned Variable Length Code) decoding
// for HTJ2K as specified in ISO/IEC 15444-15:2019 Clause 7.3.6
//
// The U-VLC code is used to decode unsigned residual values uq for quads
// where the unsigned residual offset ulf is 1 (i.e., quad has non-zero residual).
//
// The decoding is done in three stages:
// 1. Prefix decoding (decodeUPrefix) - variable length prefix
// 2. Suffix decoding (decodeUSuffix) - fixed length suffix based on prefix
// 3. Extension decoding (decodeUExtension) - optional 4-bit extension
//
// Formula: u = u_pfx + u_sfx + 4*u_ext  (Equation 3)
//
// Special case for initial line-pair quad-pairs where both quads have ulf=1:
// Formula: u = 2 + u_pfx + u_sfx + 4*u_ext  (Equation 4)
type UVLCDecoder struct {
	bitReader BitReader
}

// NewUVLCDecoder creates a new U-VLC decoder with the given bit reader
func NewUVLCDecoder(reader BitReader) *UVLCDecoder {
	return &UVLCDecoder{
		bitReader: reader,
	}
}

// BitReader interface for reading bits from VLC stream
type BitReader interface {
	// ReadBit reads a single bit (returns 0 or 1)
	ReadBit() (uint8, error)
	// ReadBits reads n bits in little-endian order (LSB first)
	ReadBitsLE(n int) (uint32, error)
}

// DecodeUnsignedResidual decodes an unsigned residual value uq
// This is the main entry point for U-VLC decoding (Clause 7.3.6)
//
// Returns:
//   - u: The decoded unsigned residual value (1-based)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidual() (uint32, error) {
	// Step 1: Decode U-VLC prefix
	u_pfx, err := d.decodeUPrefix()
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC prefix: %w", err)
	}

	// Step 2: Decode U-VLC suffix (depends on prefix)
	u_sfx, err := d.decodeUSuffix(u_pfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC suffix: %w", err)
	}

	// Step 3: Decode U-VLC extension (depends on suffix)
	u_ext, err := d.decodeUExtension(u_sfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC extension: %w", err)
	}

	// Formula (3): u = u_pfx + u_sfx + 4*u_ext
	u := uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext)

	return u, nil
}

// DecodeUnsignedResidualInitialPair decodes unsigned residual for initial line-pair
// where both quads in the pair have ulf=1 (Equation 4)
//
// Returns:
//   - u: The decoded unsigned residual value (2-based for initial pair)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidualInitialPair() (uint32, error) {
	// 保持逐步解析：prefix -> suffix -> extension，然后应用初始行对偏置 (+2)
	u_pfx, err := d.decodeUPrefix()
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC prefix: %w", err)
	}

	u_sfx, err := d.decodeUSuffix(u_pfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC suffix: %w", err)
	}

	u_ext, err := d.decodeUExtension(u_sfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC extension: %w", err)
	}

	u := 2 + uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext)
	return u, nil
}

// DecodeWithTable 尝试使用表驱动方式解码当前 quad 的 U-VLC。
// 返回 (u, ok)。若 ok=false，调用方应回退到逐步解析。
// 注意：OpenJPH 的表是双 quad 索引，这里仅使用 u_off 和 melBit 近似映射，可能不完全覆盖全部情况。
func (d *UVLCDecoder) DecodeWithTable(uOff uint8, initialPair bool, melBit int) (uint32, bool) {
	if uOff == 0 {
		return 0, false
	}

	// mode: 1 表示 u_off=1 (单个)，3/4 用于初始行对时 mel=0/1
	mode := 1
	if initialPair && melBit >= 0 {
		if melBit == 0 {
			mode = 3
		} else {
			mode = 4
		}
	}
	// 读取最多 6 bit 头部
	var head uint32
	bits := 0
	var entry UVLCDecodeEntry
	for bits < 6 {
		b, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, false
		}
		head |= uint32(b) << bits
		bits++
		if initialPair {
			entry = UVLCTbl0[(mode<<6)|int(head)]
		} else {
			entry = UVLCTbl1[(mode<<6)|int(head)]
		}
		if entry != 0 && entry.TotalPrefixLen() == bits {
			break
		}
	}
	if entry == 0 || entry.TotalPrefixLen() == 0 {
		return 0, false
	}

	// 读取剩余前缀位（如果有）
	needPrefix := entry.TotalPrefixLen() - bits
	for i := 0; i < needPrefix; i++ {
		if _, err := d.bitReader.ReadBit(); err != nil {
			return 0, false
		}
	}

	// 读取后缀
	var suf uint32
	if entry.TotalSuffixLen() > 0 {
		val, err := d.bitReader.ReadBitsLE(entry.TotalSuffixLen())
		if err != nil {
			return 0, false
		}
		suf = val
	}

	// 对于单 quad：使用 u0 信息
	u0Prefix := uint32(entry.U0Prefix())
	u0SufLen := entry.U0SuffixLen()
	var u0Suf uint32
	if u0SufLen > 0 {
		mask := uint32((1 << u0SufLen) - 1)
		u0Suf = suf & mask
	}
	u := u0Prefix + u0Suf
	if initialPair {
		u += 2
		// 如果有偏置则解扩展
		bias := UVLCBias[(mode<<6)|int(head)]
		uBias := uint32(bias & 0x3)
		if uBias > 0 {
			ext, err := d.decodeUExtension(uint8(uBias + u0Suf))
			if err != nil {
				return 0, false
			}
			u += uint32(ext) * 4
		}
	}

	return u, true
}

// DecodeUnsignedResidualSecondQuad decodes the second quad's residual when first quad has uq1 > 2
// In this special case, the prefix decoding is simplified to a single bit (Clause 7.3.6)
//
// Returns:
//   - u: The decoded unsigned residual value (1 or 2)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidualSecondQuad() (uint32, error) {
	// Read single bit ubit
	ubit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, fmt.Errorf("failed to read ubit: %w", err)
	}

	// u_pfx = ubit + 1, and final value is also ubit + 1
	u := uint32(ubit) + 1

	return u, nil
}

// decodeWithTable decodes using UVLC tables (table0 when initial pair, table1 otherwise).
// biasIndex is the mode bits: initial pair uses (u_off0 + 2*u_off1 + 4*melEvent) encoded in caller.
func (d *UVLCDecoder) decodeWithTable(initialPair bool, biasIndex int) (uint32, int, error) {
	// Read up to 6 bits prefix head (LSB-first), but stop if table entry resolves earlier
	var head uint32
	bitsRead := 0
	var entry UVLCDecodeEntry
	for bitsRead < 6 {
		b, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, 0, err
		}
		head |= uint32(b) << bitsRead
		bitsRead++

		// lookup candidate entry
		if initialPair {
			entry = UVLCTbl0[biasIndex|int(head)]
		} else {
			entry = UVLCTbl1[biasIndex|int(head)]
		}
		// If entry is non-zero and total prefix length == bitsRead, we can stop early
		if entry != 0 && entry.TotalPrefixLen() == bitsRead {
			break
		}
	}

	// entry 已在上面循环中取到（或最后一次 head 尝试）

	lp := entry.TotalPrefixLen()
	ls := entry.TotalSuffixLen()
	u0sufLen := entry.U0SuffixLen()
	u0p := entry.U0Prefix()
	u1p := entry.U1Prefix()

	// total bits consumed so far: lp bits already in head, need to drop them from head
	// consume remaining prefix bits if any
	remainPrefix := lp - 6
	if remainPrefix > 0 {
		for i := 0; i < remainPrefix; i++ {
			_, err := d.bitReader.ReadBit()
			if err != nil {
				return 0, 0, err
			}
		}
	}

	// Suffix bits (ls) are immediately after prefix
	var suffix uint32
	if ls > 0 {
		val, err := d.bitReader.ReadBitsLE(ls)
		if err != nil {
			return 0, 0, err
		}
		suffix = val
	}

	// Split suffix for quad0/quad1
	var u0suf uint32
	if u0sufLen > 0 {
		mask := uint32((1 << u0sufLen) - 1)
		u0suf = suffix & mask
		suffix >>= u0sufLen
	}
	u1suf := suffix // remaining bits (ls - u0sufLen)

	// Calculate u0/u1 (without extension). u = u_pfx + u_sfx (+2 if initial pair)
	u0 := uint32(u0p) + u0suf
	u1 := uint32(u1p) + u1suf
	if initialPair {
		u0 += 2
		u1 += 2
	}

	// Handle bias to decide whether extension bits are needed (initial pair only).
	// When bias != 0, need to add extension decoding if u0/u1 were biased down.
	if initialPair {
		bias := UVLCBias[biasIndex|int(head)]
		// bias bits: low2 for u0, high2 for u1 (values 0,1,2)
		u0bias := uint32(bias & 0x3)
		u1bias := uint32((bias >> 2) & 0x3)
		if u0bias > 0 {
			ext, err := d.decodeUExtension(uint8(u0bias + u0suf))
			if err != nil {
				return 0, 0, err
			}
			u0 += uint32(ext) * 4
		}
		if u1bias > 0 {
			ext, err := d.decodeUExtension(uint8(u1bias + u1suf))
			if err != nil {
				return 0, 0, err
			}
			u1 += uint32(ext) * 4
		}
	}

	// Return u0; caller can ignore u1 if only one quad
	return u0, int(u1), nil
}

// decodeUPrefix decodes the U-VLC prefix component
// Implements the decodeUPrefix procedure from Clause 7.3.6
//
// From spec procedure:
//
//	bit = importVLCBit
//	if(bit == 1) return 1
//	bit = importVLCBit
//	if(bit == 1) return 2
//	bit = importVLCBit
//	return (bit == 1) ? 3 : 5
//
// This means:
//
//	prefix="1"   -> u_pfx=1
//	prefix="01"  -> u_pfx=2
//	prefix="001" -> u_pfx=3
//	prefix="000" -> u_pfx=5
//
// Note: u_pfx can ONLY be 1, 2, 3, or 5 (never 4)
//
// Returns:
//   - u_pfx: Prefix value (1, 2, 3, or 5)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUPrefix() (uint8, error) {
	// Read first bit
	bit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "1" -> u_pfx = 1
		return 1, nil
	}

	// First bit was 0, read second bit
	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "01" -> u_pfx = 2
		return 2, nil
	}

	// First two bits were "00", read third bit
	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "001" -> u_pfx = 3
		return 3, nil
	}

	// Prefix = "000" -> u_pfx = 5
	return 5, nil
}

// decodeUSuffix decodes the U-VLC suffix component
// Implements the decodeUSuffix procedure from Clause 7.3.6
//
// From spec procedure:
//
//	if(u_pfx < 3) return 0
//	val = importVLCBit
//	if(u_pfx == 3) return val
//	for(i=1; i<5; i++)
//	  bit = importVLCBit
//	  val = val + (bit << i)
//	return val
//
// This means:
//
//	u_pfx=1,2: no suffix (return 0)
//	u_pfx=3:   1-bit suffix
//	u_pfx=5:   5-bit suffix
//
// Suffix is read in little-endian order (LSB first)
//
// Returns:
//   - u_sfx: Suffix value (0 if no suffix needed)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUSuffix(u_pfx uint8) (uint8, error) {
	// if(u_pfx < 3) return 0
	if u_pfx < 3 {
		return 0, nil
	}

	// val = importVLCBit
	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	// if(u_pfx == 3) return val
	if u_pfx == 3 {
		return val, nil
	}

	// for(i=1; i<5; i++)
	//   bit = importVLCBit
	//   val = val + (bit << i)
	for i := 1; i < 5; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}

// decodeUExtension decodes the U-VLC extension component
// Implements the decodeUExtension procedure from Clause 7.3.6
//
// From spec procedure:
//
//	if(u_sfx < 28) return 0
//	val = importVLCBit
//	for(i=1; i<4; i++)
//	  bit = importVLCBit
//	  val = val + (bit << i)
//	return val
//
// Extension is only present when u_sfx >= 28 (4-bit extension in little-endian)
// This handles very large magnitude values that exceed 2^37
//
// Returns:
//   - u_ext: Extension value (0 if no extension needed)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUExtension(u_sfx uint8) (uint8, error) {
	// if(u_sfx < 28) return 0
	if u_sfx < 28 {
		return 0, nil
	}

	// val = importVLCBit
	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	// for(i=1; i<4; i++)
	//   bit = importVLCBit
	//   val = val + (bit << i)
	for i := 1; i < 4; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}
