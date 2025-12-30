package lossless

import (
	"fmt"
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

// EncodeMappedValue encodes a mapped error value with limit handling
// This matches CharLS encode_mapped_value (scan.h)
func (gw *GolombWriter) EncodeMappedValue(k, mappedError, limit, quantizedBitsPerPixel int) error {
	highBits := mappedError >> uint(k)

	// Normal case: high_bits < limit - qbpp - 1
	if highBits < limit-quantizedBitsPerPixel-1 {
		// Write unary code for high_bits (high_bits 0's followed by 1)
		// If highBits + 1 > 31, split into two writes to avoid overflow
		if highBits+1 > 31 {
			// Write half as 0's
			if err := gw.WriteBits(0, highBits/2); err != nil {
				return err
			}
			highBits = highBits - highBits/2
		}

		// Write (highBits + 1) bits with value 1 at the end
		// This is: highBits 0's followed by one 1
		if err := gw.WriteBits(1, highBits+1); err != nil {
			return err
		}

		// Write remainder (k bits)
		if k > 0 {
			remainder := mappedError & ((1 << uint(k)) - 1)
			if err := gw.WriteBits(uint32(remainder), k); err != nil {
				return err
			}
		}

		return nil
	}

	// Overflow case: high_bits >= limit - qbpp - 1
	// Write (limit - qbpp) bits as unary code: (limit-qbpp-1) 0's followed by one 1
	limitBits := limit - quantizedBitsPerPixel
	if limitBits > 31 {
		// Split into two writes
		if err := gw.WriteBits(0, 31); err != nil {
			return err
		}
		if err := gw.WriteBits(1, limitBits-31); err != nil {
			return err
		}
	} else {
		// Write all at once: (limitBits-1) 0's followed by one 1
		// This is equivalent to writing value 1 with limitBits bits
		if err := gw.WriteBits(1, limitBits); err != nil {
			return err
		}
	}

	// Write (mappedError - 1) masked to qbpp bits
	value := (mappedError - 1) & ((1 << uint(quantizedBitsPerPixel)) - 1)
	if err := gw.WriteBits(uint32(value), quantizedBitsPerPixel); err != nil {
		return err
	}

	return nil
}

// GolombReader reads Golomb-Rice encoded data
type GolombReader struct {
	r          io.Reader
	buffer     uint32 // bit buffer
	bufferSize int    // number of bits in buffer
}

// NewGolombReader creates a new Golomb-Rice reader
func NewGolombReader(r io.Reader) *GolombReader {
	return &GolombReader{
		r:          r,
		bufferSize: 0,
	}
}

// DecodeValue reads a Golomb-encoded value with limit handling
// This matches CharLS decode_value (scan.h)
func (gr *GolombReader) DecodeValue(k, limit, quantizedBitsPerPixel int) (int, error) {
	// Read high bits (unary code - count of 0's before the 1)
	highBits := 0
	for {
		bit, err := gr.ReadBit()
		if err != nil {
			return 0, err
		}
		if bit == 1 {
			break
		}
		highBits++
		if highBits > 1000 {  // Safety check
			return 0, fmt.Errorf("highBits exceeded safety limit")
		}
	}

	// CharLS: if (high_bits >= limit - (quantized_bits_per_pixel + 1))
	//             return Strategy::read_value(quantized_bits_per_pixel) + 1;
	if highBits >= limit-(quantizedBitsPerPixel+1) {
		val, err := gr.ReadBits(quantizedBitsPerPixel)
		if err != nil {
			return 0, err
		}
		return int(val) + 1, nil
	}

	// CharLS: if (k == 0) return high_bits;
	if k == 0 {
		return highBits, nil
	}

	// CharLS: return (high_bits << k) + Strategy::read_value(k);
	remainder, err := gr.ReadBits(k)
	if err != nil {
		return 0, err
	}

	return (highBits << uint(k)) + int(remainder), nil
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
// Note: byte-stuffing (0xFF 0x00 -> 0xFF) is already handled by the decoder
// before passing data to GolombReader, so we don't need to handle it here.
func (gr *GolombReader) fillBuffer() error {
	buf := make([]byte, 1)
	_, err := io.ReadFull(gr.r, buf)
	if err != nil {
		return err
	}

	gr.buffer = uint32(buf[0])
	gr.bufferSize = 8

	return nil
}
