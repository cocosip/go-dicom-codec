package standard

import (
	"encoding/binary"
	"io"
)

// Reader provides utilities for reading JPEG data
type Reader struct {
	r   io.Reader
	buf [2]byte
}

// NewReader creates a new JPEG reader
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// ReadByte reads a single byte
func (r *Reader) ReadByte() (byte, error) {
	_, err := io.ReadFull(r.r, r.buf[:1])
	if err != nil {
		return 0, err
	}
	return r.buf[0], nil
}

// ReadUint16 reads a 16-bit big-endian value
func (r *Reader) ReadUint16() (uint16, error) {
	_, err := io.ReadFull(r.r, r.buf[:2])
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(r.buf[:2]), nil
}

// ReadMarker reads the next JPEG marker
// Returns the marker value (without the 0xFF prefix)
func (r *Reader) ReadMarker() (uint16, error) {
	// Read first byte, should be 0xFF
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != 0xFF {
		return 0, ErrInvalidMarker
	}

	// Skip any padding 0xFF bytes
	for {
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b != 0xFF {
			break
		}
	}

	// 0x00 is a stuffed byte (escaped 0xFF in data), not a marker
	if b == 0x00 {
		return 0, ErrInvalidMarker
	}

	return uint16(0xFF00) | uint16(b), nil
}

// ReadSegment reads a segment with its length
// Returns the segment data (without the length field)
func (r *Reader) ReadSegment() ([]byte, error) {
	length, err := r.ReadUint16()
	if err != nil {
		return nil, err
	}

	// Length includes itself (2 bytes)
	if length < 2 {
		return nil, ErrInvalidData
	}

	data := make([]byte, length-2)
	_, err = io.ReadFull(r.r, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ReadFull reads exactly len(buf) bytes
func (r *Reader) ReadFull(buf []byte) error {
	_, err := io.ReadFull(r.r, buf)
	return err
}

// Skip skips n bytes
func (r *Reader) Skip(n int) error {
	if n <= 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, r.r, int64(n))
	return err
}

// Read implements io.Reader interface
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}
