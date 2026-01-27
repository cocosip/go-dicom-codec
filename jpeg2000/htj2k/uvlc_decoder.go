package htj2k

import "fmt"

// UVLCDecoder implements U-VLC (Unsigned Variable Length Code) decoding
// for HTJ2K as specified in ISO/IEC 15444-15:2019 Clause 7.3.6.
type UVLCDecoder struct {
	bitReader BitReader
}

// NewUVLCDecoder creates a new U-VLC decoder with the given bit reader.
func NewUVLCDecoder(reader BitReader) *UVLCDecoder {
	return &UVLCDecoder{
		bitReader: reader,
	}
}

// BitReader interface for reading bits from VLC stream.
type BitReader interface {
	// ReadBit reads a single bit (returns 0 or 1).
	ReadBit() (uint8, error)
	// ReadBitsLE reads n bits in little-endian order (LSB first).
	ReadBitsLE(n int) (uint32, error)
}

// DecodeUnsignedResidual decodes an unsigned residual value uq.
func (d *UVLCDecoder) DecodeUnsignedResidual() (uint32, error) {
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

	return uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext), nil
}

// DecodeUnsignedResidualInitialPair decodes uq for the initial line-pair
// when both quads have u_off=1.
func (d *UVLCDecoder) DecodeUnsignedResidualInitialPair() (uint32, error) {
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

	return 2 + uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext), nil
}

// DecodeUnsignedResidualSecondQuad decodes the second quad's residual when
// the first quad has uq1 > 2 (special simplified case).
func (d *UVLCDecoder) DecodeUnsignedResidualSecondQuad() (uint32, error) {
	ubit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, fmt.Errorf("failed to read ubit: %w", err)
	}
	return uint32(ubit) + 1, nil
}

// DecodePair decodes U-VLC for a quad pair using OpenJPH tables.
// uOff0/uOff1 are the u_off flags for the two quads.
// melEvent is used only for initial pairs when both u_off are 1.
func (d *UVLCDecoder) DecodePair(uOff0, uOff1 uint8, initialPair bool, melEvent int) (uint32, uint32, error) {
	mode := int(uOff0) + 2*int(uOff1)
	if mode == 0 {
		return 0, 0, nil
	}

	if initialPair && mode == 3 {
		if melEvent > 0 {
			mode = 4
		}
	}

	var table []UVLCDecodeEntry
	if initialPair {
		table = UVLCTbl0[:]
	} else {
		table = UVLCTbl1[:]
	}

	entry, head, bitsRead, err := d.decodeUVLCEntry(table, mode)
	if err != nil {
		return 0, 0, err
	}
	if entry == 0 {
		return 0, 0, ErrInsufficientData
	}

	lp := entry.TotalPrefixLen()
	if bitsRead < lp {
		for i := bitsRead; i < lp; i++ {
			if _, err := d.bitReader.ReadBit(); err != nil {
				return 0, 0, err
			}
		}
	}

	ls := entry.TotalSuffixLen()
	var suffix uint32
	if ls > 0 {
		val, err := d.bitReader.ReadBitsLE(ls)
		if err != nil {
			return 0, 0, err
		}
		suffix = val
	}

	u0sufLen := entry.U0SuffixLen()
	var u0suf uint32
	if u0sufLen > 0 {
		mask := uint32((1 << u0sufLen) - 1)
		u0suf = suffix & mask
	}
	u1suf := suffix >> u0sufLen

	u0 := uint32(entry.U0Prefix()) + u0suf
	u1 := uint32(entry.U1Prefix()) + u1suf

	if initialPair {
		bias := UVLCBias[(mode<<6)|int(head)]
		u0Bias := uint32(bias & 0x3)
		u1Bias := uint32((bias >> 2) & 0x3)

		u0, err = d.applyUExtension(u0, u0Bias, true)
		if err != nil {
			return 0, 0, err
		}
		u1, err = d.applyUExtension(u1, u1Bias, true)
		if err != nil {
			return 0, 0, err
		}
	} else {
		u0, err = d.applyUExtension(u0, 0, false)
		if err != nil {
			return 0, 0, err
		}
		u1, err = d.applyUExtension(u1, 0, false)
		if err != nil {
			return 0, 0, err
		}
	}

	if uOff0 == 0 {
		u0 = 0
	}
	if uOff1 == 0 {
		u1 = 0
	}

	return u0, u1, nil
}

// DecodeWithTable decodes a single quad using the UVLC tables.
func (d *UVLCDecoder) DecodeWithTable(uOff uint8, initialPair bool, melBit int) (uint32, bool) {
	u0, _, err := d.DecodePair(uOff, 0, initialPair, melBit)
	if err != nil {
		return 0, false
	}
	return u0, true
}

func (d *UVLCDecoder) decodeUVLCEntry(table []UVLCDecodeEntry, mode int) (UVLCDecodeEntry, uint32, int, error) {
	var head uint32
	bitsRead := 0

	for bitsRead < 6 {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, head, bitsRead, err
		}
		head |= uint32(bit) << bitsRead
		bitsRead++

		entry := table[(mode<<6)|int(head)]
		if entry != 0 && entry.TotalPrefixLen() == bitsRead {
			return entry, head, bitsRead, nil
		}
	}

	entry := table[(mode<<6)|int(head)]
	if entry == 0 {
		return 0, head, bitsRead, ErrInsufficientData
	}

	return entry, head, bitsRead, nil
}

func (d *UVLCDecoder) applyUExtension(u uint32, bias uint32, useBias bool) (uint32, error) {
	threshold := uint32(32)
	if useBias {
		if u <= bias || u-bias <= threshold {
			return u, nil
		}
	} else if u <= threshold {
		return u, nil
	}

	ext, err := d.bitReader.ReadBitsLE(4)
	if err != nil {
		return 0, err
	}
	return u + (ext << 2), nil
}

// decodeUPrefix decodes the U-VLC prefix component.
func (d *UVLCDecoder) decodeUPrefix() (uint8, error) {
	bit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}
	if bit == 1 {
		return 1, nil
	}

	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}
	if bit == 1 {
		return 2, nil
	}

	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}
	if bit == 1 {
		return 3, nil
	}

	return 5, nil
}

// decodeUSuffix decodes the U-VLC suffix component.
func (d *UVLCDecoder) decodeUSuffix(u_pfx uint8) (uint8, error) {
	if u_pfx < 3 {
		return 0, nil
	}

	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}
	if u_pfx == 3 {
		return val, nil
	}

	for i := 1; i < 5; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}

// decodeUExtension decodes the U-VLC extension component.
func (d *UVLCDecoder) decodeUExtension(u_sfx uint8) (uint8, error) {
	if u_sfx < 28 {
		return 0, nil
	}

	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	for i := 1; i < 4; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}
