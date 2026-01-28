package htj2k

// Reverse VLC decoding for HTJ2K VLC segments that grow backward.
// Matches OpenJPH's dec_rev implementation:
// - Bytes are read from end to start
// - New bytes are shifted to the bottom of the accumulator
// - Bits are extracted from the top of the accumulator

type reverseBitReader struct {
	data     []byte
	pos      int
	tmp      uint64
	num      int // number of valid bits in tmp
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

	// Read init byte from end of stream (OpenJPH dec_initMixin)
	d := r.data[r.pos]
	r.pos--

	// Handle unstuffing: if byte > 0x8F and (byte & 0x7F) == 0x7F, strip stuffed bit
	tmp := uint64(d)
	if d > 0x8F && (d&0x7F) == 0x7F {
		tmp >>= 1
	}

	r.num = 4
	// Strip trailing 1-bits down to first 0
	if (tmp & 0x7) == 0x7 {
		r.num--
	}

	// Shift right to keep only top `num` bits at LSB side
	tmp >>= (8 - uint(r.num))
	r.tmp = tmp
	r.unstuff = (d | 0x0F) > 0x8F

	return true
}

func (r *reverseBitReader) readMore(minBits int) bool {
	if !r.init() {
		return false
	}
	for r.num < minBits && r.pos >= 0 {
		b := r.data[r.pos]
		r.pos--
		bits := 8
		if r.unstuff && (b&0x7F) == 0x7F {
			bits = 7
		}
		// OpenJPH: tmp = (tmp << bits) | b
		// Shift existing bits up, add new byte at bottom
		r.tmp = (r.tmp << uint(bits)) | uint64(b)
		r.num += bits
		r.unstuff = b > 0x8F
	}
	return r.num >= minBits
}

func (r *reverseBitReader) readBits(n int) (uint32, bool) {
	if n == 0 {
		return 0, true
	}
	if !r.readMore(n) {
		return 0, false
	}
	// OpenJPH: extract from TOP of accumulator
	r.num -= n
	val := uint32(r.tmp >> uint(r.num))
	r.tmp &= (1 << uint(r.num)) - 1
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
