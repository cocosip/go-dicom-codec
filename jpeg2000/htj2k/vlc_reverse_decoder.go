package htj2k

// Reverse VLC decoding for HTJ2K VLC segments that grow backward.

type reverseBitReader struct {
	data     []byte
	pos      int
	tmp      uint64
	bits     int
	unstuff  bool
	initDone bool
}

func newReverseBitReader(data []byte) *reverseBitReader {
	return &reverseBitReader{
		data: data,
		pos:  len(data) - 1,
	}
}

func (r *reverseBitReader) init() bool {
	if r.initDone {
		return true
	}
	r.initDone = true
	if r.pos < 0 {
		return false
	}

	// The VLC stream is terminated with a 4-bit padding nibble.
	d := r.data[r.pos]
	r.pos--

	r.tmp = uint64(d >> 4)
	r.bits = 4
	if (r.tmp & 0x7) == 0x7 {
		r.bits--
	}
	r.unstuff = (d|0x0F) > 0x8F

	return true
}

func (r *reverseBitReader) readMore(minBits int) bool {
	if !r.init() {
		return false
	}
	for r.bits < minBits && r.pos >= 0 {
		b := r.data[r.pos]
		r.pos--
		bits := 8
		if r.unstuff && (b&0x7F) == 0x7F {
			bits = 7
		}
		r.tmp |= uint64(b) << r.bits
		r.bits += bits
		r.unstuff = b > 0x8F
	}
	return r.bits >= minBits
}

func (r *reverseBitReader) readBits(n int) (uint32, bool) {
	if n == 0 {
		return 0, true
	}
	if !r.readMore(n) {
		return 0, false
	}
	mask := uint64((1 << n) - 1)
	val := uint32(r.tmp & mask)
	r.tmp >>= n
	r.bits -= n
	return val, true
}

func (r *reverseBitReader) ReadBit() (uint8, error) {
	bit, ok := r.readBits(1)
	if !ok {
		return 0, ErrInsufficientData
	}
	return uint8(bit), nil
}

func (r *reverseBitReader) ReadBitsLE(n int) (uint32, error) {
	val, ok := r.readBits(n)
	if !ok {
		return 0, ErrInsufficientData
	}
	return val, nil
}

// VLCDecoderReverse implements VLC decoding using a reverse-growing stream.
type VLCDecoderReverse struct {
	reader *reverseBitReader
	tbl0   [1024]uint16
	tbl1   [1024]uint16
}

// NewVLCDecoderReverse creates a new reverse VLC decoder.
func NewVLCDecoderReverse(data []byte) *VLCDecoderReverse {
	v := &VLCDecoderReverse{
		reader: newReverseBitReader(data),
	}
	v.buildLookupTables()
	return v
}

func (v *VLCDecoderReverse) buildLookupTables() {
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		cq := i >> 7
		bestLen := uint8(0)
		var packed uint16
		for j := range VLC_tbl0 {
			entry := &VLC_tbl0[j]
			if int(entry.CQ) != cq {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == (cwd & mask) {
				if entry.CwdLen > bestLen {
					bestLen = entry.CwdLen
					packed = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
				}
			}
		}
		v.tbl0[i] = packed
	}

	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		cq := i >> 7
		bestLen := uint8(0)
		var packed uint16
		for j := range VLC_tbl1 {
			entry := &VLC_tbl1[j]
			if int(entry.CQ) != cq {
				continue
			}
			mask := (1 << entry.CwdLen) - 1
			if int(entry.Cwd) == (cwd & mask) {
				if entry.CwdLen > bestLen {
					bestLen = entry.CwdLen
					packed = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
				}
			}
		}
		v.tbl1[i] = packed
	}
}

func (v *VLCDecoderReverse) readBits(n int) (uint32, bool) {
	return v.reader.readBits(n)
}

// ReadBit implements BitReader for UVLC decoding.
func (v *VLCDecoderReverse) ReadBit() (uint8, error) {
	return v.reader.ReadBit()
}

// ReadBitsLE implements BitReader for UVLC decoding.
func (v *VLCDecoderReverse) ReadBitsLE(n int) (uint32, error) {
	return v.reader.ReadBitsLE(n)
}

// DecodeQuadWithContext decodes a quad using context-based VLC decoding.
func (v *VLCDecoderReverse) DecodeQuadWithContext(context uint8, isFirstRow bool) (uint8, uint8, uint8, uint8, bool) {
	var bits uint32
	for length := 1; length <= 7; length++ {
		b, ok := v.readBits(1)
		if !ok {
			return 0, 0, 0, 0, false
		}
		bits |= (b << (length - 1))

		index := (uint16(context) << 7) | uint16(bits&0x7F)
		var entry uint16
		if isFirstRow {
			entry = v.tbl0[index]
		} else {
			entry = v.tbl1[index]
		}

		cwdLen := uint8(entry & 0x7)
		if cwdLen == uint8(length) {
			rho := uint8((entry >> 4) & 0xF)
			uOff := uint8((entry >> 3) & 0x1)
			e1 := uint8((entry >> 8) & 0xF)
			ek := uint8((entry >> 12) & 0xF)
			return rho, uOff, ek, e1, true
		}
	}
	return 0, 0, 0, 0, false
}
