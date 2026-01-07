package lossless

import (
	"fmt"
	"io"
)

// GolombWriter writes Golomb-Rice encoded data with JPEG-LS byte stuffing
type GolombWriter struct {
	w            io.Writer
	bitBuffer    uint32 // bit buffer (32 bits)
	freeBitCount int    // number of free bits in buffer (32 initially)
	isFFWritten  bool   // true if last byte written was 0xFF
	bytesWritten int    // total bytes written
}

// NewGolombWriter creates a new Golomb-Rice writer
func NewGolombWriter(w io.Writer) *GolombWriter {
	return &GolombWriter{
		w:            w,
		freeBitCount: 32, // Start with 32 free bits
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
	return gw.WriteBits(uint32(bit&1), 1)
}

// WriteBits writes n bits (matches CharLS append_to_bit_stream)
func (gw *GolombWriter) WriteBits(bits uint32, bitCount int) error {
	gw.freeBitCount -= bitCount
	if gw.freeBitCount >= 0 {
		gw.bitBuffer |= bits << uint(gw.freeBitCount)
	} else {
		// Add as much bits in the remaining space as possible and flush
		gw.bitBuffer |= bits >> uint(-gw.freeBitCount)
		if err := gw.flush(); err != nil {
			return err
		}

		// A second flush may be required if extra marker detect bits were needed
		if gw.freeBitCount < 0 {
			gw.bitBuffer |= bits >> uint(-gw.freeBitCount)
			if err := gw.flush(); err != nil {
				return err
			}
		}

		gw.bitBuffer |= bits << uint(gw.freeBitCount)
	}
	return nil
}

// flush writes buffered bits to output (matches CharLS flush())
func (gw *GolombWriter) flush() error {
	for i := 0; i < 4; i++ {
		if gw.freeBitCount >= 32 {
			gw.freeBitCount = 32
			break
		}

		var b byte
		if gw.isFFWritten {
			// JPEG-LS requirement (T.87, A.1): after 0xFF, insert a single 0 bit
			// Write only 7 bits (top 7 bits of buffer)
			b = byte(gw.bitBuffer >> 25)
			gw.bitBuffer <<= 7
			gw.freeBitCount += 7
		} else {
			// Normal case: write 8 bits
			b = byte(gw.bitBuffer >> 24)
			gw.bitBuffer <<= 8
			gw.freeBitCount += 8
		}

		if _, err := gw.w.Write([]byte{b}); err != nil {
			return err
		}
		gw.isFFWritten = (b == 0xFF)
		gw.bytesWritten++
	}
	return nil
}

// Flush flushes remaining bits and completes the bitstream (matches CharLS end_scan())
func (gw *GolombWriter) Flush() error {
	if err := gw.flush(); err != nil {
		return err
	}

	// If a 0xFF was written, flush() will force one unset bit anyway
	if gw.isFFWritten {
		if err := gw.WriteBits(0, (gw.freeBitCount-1)%8); err != nil {
			return err
		}
	}

	if err := gw.flush(); err != nil {
		return err
	}

	return nil
}

// WriteUnary writes a unary code: n zeros followed by one 1
func (gw *GolombWriter) WriteUnary(n int) error {
	// Write n zeros
	for i := 0; i < n; i++ {
		if err := gw.WriteBit(0); err != nil {
			return err
		}
	}
	// Write one 1
	return gw.WriteBit(1)
}

// WriteZeros writes n zero bits
func (gw *GolombWriter) WriteZeros(n int) error {
	for i := 0; i < n; i++ {
		if err := gw.WriteBit(0); err != nil {
			return err
		}
	}
	return nil
}

// EncodeMappedValue encodes a mapped error value with limit handling (CharLS encode_mapped_value).
func (gw *GolombWriter) EncodeMappedValue(k, mappedError, limit, quantizedBitsPerPixel int) error {
	highBits := mappedError >> uint(k)

	// Normal case: high_bits < limit - (qbpp + 1)
	if highBits < limit-(quantizedBitsPerPixel+1) {
		// CharLS optimization: split unary code if too long (> 31 bits)
		if highBits+1 > 31 {
			// Write half as zeros first
			if err := gw.WriteZeros(highBits / 2); err != nil {
				return err
			}
			highBits = highBits - highBits/2
		}
		// Write unary code: highBits zeros followed by one 1
		if err := gw.WriteUnary(highBits); err != nil {
			return err
		}
		// write remainder
		if k > 0 {
			remainder := mappedError & ((1 << uint(k)) - 1)
			if err := gw.WriteBits(uint32(remainder), k); err != nil {
				return err
			}
		}
		return nil
	}

	// Escape case: write (limit - qbpp) zeros then 1, then mappedError-1 with qbpp bits
	escapeBits := limit - quantizedBitsPerPixel

	// CharLS optimization: split escape code if too long (> 31 bits)
	if escapeBits > 31 {
		// Write 31 zeros first
		if err := gw.WriteZeros(31); err != nil {
			return err
		}
		// Write remaining unary code
		if err := gw.WriteUnary(escapeBits - 31 - 1); err != nil {
			return err
		}
	} else {
		// Write unary code: escapeBits-1 zeros followed by one 1
		if err := gw.WriteUnary(escapeBits - 1); err != nil {
			return err
		}
	}

	value := (mappedError - 1) & ((1 << uint(quantizedBitsPerPixel)) - 1)
	return gw.WriteBits(uint32(value), quantizedBitsPerPixel)
}

// GolombReader reads Golomb-Rice encoded data with JPEG-LS byte stuffing
type GolombReader struct {
	r            io.Reader
	buffer       uint32 // bit buffer
	bufferSize   int    // number of bits in buffer
	bitsRead     int64  // debug counter
	skipNextBit  bool   // true if we need to skip first bit of next byte
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
			return 0, fmt.Errorf("read highBits after %d bits: %w", gr.bitsRead, err)
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
			return 0, fmt.Errorf("read overflow bits after %d bits: %w", gr.bitsRead, err)
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
		return 0, fmt.Errorf("read remainder after %d bits: %w", gr.bitsRead, err)
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
			return 0, fmt.Errorf("fillBuffer: %w", err)
		}
	}

	gr.bufferSize--
	bit := int((gr.buffer >> uint(gr.bufferSize)) & 1)
	gr.bitsRead++
	return bit, nil
}

// ReadBits reads n bits
func (gr *GolombReader) ReadBits(n int) (uint32, error) {
	result := uint32(0)
	for n > 0 {
		if gr.bufferSize == 0 {
			if err := gr.fillBuffer(); err != nil {
				return 0, fmt.Errorf("fillBuffer: %w", err)
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
		gr.bitsRead += int64(take)
	}
	return result, nil
}

// fillBuffer reads next byte into buffer with JPEG-LS byte stuffing handling
// Matches CharLS fill_read_cache logic (decoder_strategy.h:242-250)
func (gr *GolombReader) fillBuffer() error {
	buf := make([]byte, 1)
	_, err := io.ReadFull(gr.r, buf)
	if err != nil {
		return err
	}

	b := buf[0]

	// JPEG-LS byte stuffing (ISO/IEC 14495-1, A.1):
	// After a 0xFF byte is written, a stuffed 0 bit is inserted
	// This stuffed bit is the high bit of the next byte
	// CharLS: read all 8 bits, add to cache, then decrement valid_bits if prev was 0xFF
	if gr.skipNextBit {
		// Previous byte was 0xFF, so skip the high bit of this byte
		gr.buffer = uint32(b & 0x7F)  // Only keep lower 7 bits
		gr.bufferSize = 7
		gr.skipNextBit = false
	} else {
		// Normal byte: use all 8 bits
		gr.buffer = uint32(b)
		gr.bufferSize = 8
	}

	// If this byte is 0xFF, mark that we need to skip next byte's high bit
	if b == 0xFF {
		gr.skipNextBit = true
	}

	return nil
}
