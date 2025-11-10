package lossless

import (
	"io"
)

// GolombWriter writes Golomb-Rice encoded data
type GolombWriter struct {
	w           io.Writer
	buffer      uint32 // bit buffer
	bufferSize  int    // number of bits in buffer
	bytesPut    int    // number of bytes written
	enableLimit bool   // enable byte-stuffing (0xFF -> 0xFF 0x00)
}

// NewGolombWriter creates a new Golomb-Rice writer
func NewGolombWriter(w io.Writer) *GolombWriter {
	return &GolombWriter{
		w:           w,
		enableLimit: true, // JPEG-LS uses byte-stuffing
	}
}

// WriteGolomb writes a value using Golomb-Rice coding with parameter k
func (gw *GolombWriter) WriteGolomb(value int, k int) error {
	// Golomb-Rice coding for non-negative integers
	// Split value into quotient and remainder
	quotient := value >> uint(k)
	remainder := value & ((1 << uint(k)) - 1)

	// Write quotient in unary (quotient zeros followed by a one)
	for i := 0; i < quotient; i++ {
		if err := gw.WriteBit(0); err != nil {
			return err
		}
	}
	if err := gw.WriteBit(1); err != nil {
		return err
	}

	// Write remainder in binary (k bits)
	if k > 0 {
		if err := gw.WriteBits(uint32(remainder), k); err != nil {
			return err
		}
	}

	return nil
}

// WriteBit writes a single bit
func (gw *GolombWriter) WriteBit(bit int) error {
	gw.buffer = (gw.buffer << 1) | uint32(bit&1)
	gw.bufferSize++

	if gw.bufferSize == 8 {
		return gw.flushByte()
	}
	return nil
}

// WriteBits writes n bits
func (gw *GolombWriter) WriteBits(bits uint32, n int) error {
	for n > 0 {
		// How many bits can we fit in current byte
		space := 8 - gw.bufferSize
		if space > n {
			space = n
		}

		// Extract the top 'space' bits
		shift := uint(n - space)
		value := (bits >> shift) & ((1 << uint(space)) - 1)

		gw.buffer = (gw.buffer << uint(space)) | value
		gw.bufferSize += space
		n -= space

		if gw.bufferSize == 8 {
			if err := gw.flushByte(); err != nil {
				return err
			}
		}
	}
	return nil
}

// flushByte writes the buffered byte
func (gw *GolombWriter) flushByte() error {
	b := byte(gw.buffer)
	gw.buffer = 0
	gw.bufferSize = 0

	// Write byte
	if _, err := gw.w.Write([]byte{b}); err != nil {
		return err
	}
	gw.bytesPut++

	// Byte-stuffing: if byte is 0xFF, write 0x00 after it
	if gw.enableLimit && b == 0xFF {
		if _, err := gw.w.Write([]byte{0x00}); err != nil {
			return err
		}
		gw.bytesPut++
	}

	return nil
}

// Flush flushes remaining bits (pad with zeros)
func (gw *GolombWriter) Flush() error {
	if gw.bufferSize > 0 {
		// Pad with zeros to make a full byte
		gw.buffer <<= uint(8 - gw.bufferSize)
		return gw.flushByte()
	}
	return nil
}

// GolombReader reads Golomb-Rice encoded data
type GolombReader struct {
	r          io.Reader
	buffer     uint32 // bit buffer
	bufferSize int    // number of bits in buffer
	prevByte   byte   // previous byte read (for byte-stuffing detection)
}

// NewGolombReader creates a new Golomb-Rice reader
func NewGolombReader(r io.Reader) *GolombReader {
	return &GolombReader{
		r:          r,
		prevByte:   0,
		bufferSize: 0,
	}
}

// ReadGolomb reads a value using Golomb-Rice coding with parameter k
func (gr *GolombReader) ReadGolomb(k int) (int, error) {
	// Read quotient (unary code)
	quotient := 0
	for {
		bit, err := gr.ReadBit()
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			break
		}
		quotient++
	}

	// Read remainder (k bits)
	remainder := 0
	if k > 0 {
		val, err := gr.ReadBits(k)
		if err != nil {
			return 0, err
		}
		remainder = int(val)
	}

	// Reconstruct value
	value := (quotient << uint(k)) | remainder
	return value, nil
}

// ReadBit reads a single bit
func (gr *GolombReader) ReadBit() (int, error) {
	if gr.bufferSize == 0 {
		if err := gr.fillBuffer(); err != nil {
			return 0, err
		}
	}

	gr.bufferSize--
	bit := int((gr.buffer >> uint(gr.bufferSize)) & 1)
	return bit, nil
}

// ReadBits reads n bits
func (gr *GolombReader) ReadBits(n int) (uint32, error) {
	result := uint32(0)
	for n > 0 {
		if gr.bufferSize == 0 {
			if err := gr.fillBuffer(); err != nil {
				return 0, err
			}
		}

		// Take as many bits as we can from current buffer
		take := n
		if take > gr.bufferSize {
			take = gr.bufferSize
		}

		gr.bufferSize -= take
		bits := (gr.buffer >> uint(gr.bufferSize)) & ((1 << uint(take)) - 1)
		result = (result << uint(take)) | bits
		n -= take
	}
	return result, nil
}

// fillBuffer reads next byte into buffer
func (gr *GolombReader) fillBuffer() error {
	buf := make([]byte, 1)
	_, err := io.ReadFull(gr.r, buf)
	if err != nil {
		return err
	}

	b := buf[0]

	// Handle byte-stuffing: 0xFF 0x00 -> 0xFF
	if gr.prevByte == 0xFF && b == 0x00 {
		// Skip the 0x00, read next byte
		_, err := io.ReadFull(gr.r, buf)
		if err != nil {
			return err
		}
		b = buf[0]
	}

	gr.buffer = uint32(b)
	gr.bufferSize = 8
	gr.prevByte = b

	return nil
}
