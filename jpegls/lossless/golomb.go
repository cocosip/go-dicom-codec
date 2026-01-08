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
	// CharLS end_scan logic - exactly matches encoder_strategy.h:79-91
	// First flush: write out buffered data
	if err := gw.flush(); err != nil {
		return err
	}

	// If a 0xFF was written, flush() will force one unset bit anyway
	// Add padding to align properly (CharLS line 86)
	if gw.isFFWritten {
		padBits := (gw.freeBitCount - 1) % 8
		if err := gw.WriteBits(0, padBits); err != nil {
			return err
		}
	}

	// Second flush: write out any remaining data
	if err := gw.flush(); err != nil {
		return err
	}

	return nil
}

// WriteUnary writes a unary code: n zeros followed by one 1
// Matches CharLS append_to_bit_stream(1, n+1) which writes n zeros then one 1
func (gw *GolombWriter) WriteUnary(n int) error {
	// CharLS optimization: write all bits at once
	// append_to_bit_stream(1, n+1) writes value 1 with n+1 bits
	// This produces n zeros followed by one 1
	return gw.WriteBits(1, n+1)
}

// WriteZeros writes n zero bits
func (gw *GolombWriter) WriteZeros(n int) error {
	// Optimized: write zeros in chunks to avoid loop overhead
	const maxChunk = 31 // Maximum safe bits per write
	for n > 0 {
		chunk := n
		if chunk > maxChunk {
			chunk = maxChunk
		}
		if err := gw.WriteBits(0, chunk); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}

// WriteOnes writes n one bits (matches CharLS append_ones_to_bit_stream)
func (gw *GolombWriter) WriteOnes(n int) error {
	// CharLS: append_to_bit_stream((1U << length) - 1U, length)
	// Creates a value with n one bits
	const maxChunk = 31 // Maximum safe bits per write
	for n > 0 {
		chunk := n
		if chunk > maxChunk {
			chunk = maxChunk
		}
		value := (uint32(1) << uint(chunk)) - 1 // All ones
		if err := gw.WriteBits(value, chunk); err != nil {
			return err
		}
		n -= chunk
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
// Matches CharLS decoder_strategy implementation exactly
type GolombReader struct {
	readCache   uint64 // read_cache_ - bit buffer (64-bit like CharLS size_t on 64-bit systems)
	validBits   int32  // valid_bits_ - number of valid bits in cache
	data        []byte // scan data buffer
	position    int    // position_ - current read position
	endPosition int    // end_position_ - end of scan data
	positionFF  int    // position_ff_ - position of next 0xFF byte (for optimization)
	bitsRead    int64  // debug counter
}

const (
	cacheTBitCount       = 64 // sizeof(uint64) * 8
	maxReadableCacheBits = 56 // cacheTBitCount - 8
	jpegMarkerStartByte  = 0xFF
)

// NewGolombReader creates a new Golomb-Rice reader matching CharLS
func NewGolombReader(r io.Reader) *GolombReader {
	// Read all data upfront (CharLS operates on byte arrays)
	data, err := io.ReadAll(r)
	if err != nil {
		// Shouldn't happen in normal usage
		return &GolombReader{
			data:        []byte{},
			position:    0,
			endPosition: 0,
			positionFF:  0,
			validBits:   0,
		}
	}

	gr := &GolombReader{
		data:        data,
		position:    0,
		endPosition: len(data),
		validBits:   0,
	}

	// Initialize positionFF to first 0xFF or end
	gr.findJPEGMarkerStartByte()

	return gr
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
// Matches CharLS read_bit logic
func (gr *GolombReader) ReadBit() (int, error) {
	if gr.validBits == 0 {
		if err := gr.fillReadCache(); err != nil {
			return 0, err
		}
	}

	gr.validBits--
	// Extract top bit from cache
	bit := int((gr.readCache >> uint(cacheTBitCount-1)) & 1)
	gr.readCache <<= 1
	gr.bitsRead++
	return bit, nil
}

// ReadBits reads n bits from the stream
// Matches CharLS read_value logic
func (gr *GolombReader) ReadBits(n int) (uint32, error) {
	if n == 0 {
		return 0, nil
	}

	if n > 32 {
		return 0, fmt.Errorf("cannot read more than 32 bits at once")
	}

	// Ensure we have enough bits in cache
	if gr.validBits < int32(n) {
		if err := gr.fillReadCache(); err != nil {
			return 0, err
		}
		if gr.validBits < int32(n) {
			// Debug: show state when we don't have enough bits
			return 0, fmt.Errorf("not enough bits available: need %d, have %d (position=%d/%d)",
				n, gr.validBits, gr.position, gr.endPosition)
		}
	}

	// Extract top n bits from cache
	result := uint32(gr.readCache >> uint(cacheTBitCount-n))
	gr.readCache <<= uint(n)
	gr.validBits -= int32(n)
	gr.bitsRead += int64(n)

	return result, nil
}

// findJPEGMarkerStartByte finds the next 0xFF byte for optimization
// Matches CharLS find_jpeg_marker_start_byte (decoder_strategy.h:272-280)
func (gr *GolombReader) findJPEGMarkerStartByte() {
	// Search for next 0xFF byte from current position
	for i := gr.position; i < gr.endPosition; i++ {
		if gr.data[i] == jpegMarkerStartByte {
			gr.positionFF = i
			return
		}
	}
	// No 0xFF found, set to end
	gr.positionFF = gr.endPosition
}

// fillReadCacheOptimistic tries to read multiple bytes quickly when no 0xFF is nearby
// Matches CharLS fill_read_cache_optimistic (decoder_strategy.h:257-270)
func (gr *GolombReader) fillReadCacheOptimistic() bool {
	// Fast path: if there's no 0xFF byte nearby, read multiple bytes at once
	// positionFF points to next 0xFF, so we can safely read up to that point
	if gr.position < gr.positionFF-7 { // Need at least 8 bytes safety margin
		// Read up to 8 bytes (64 bits) at once
		bytesToRead := (cacheTBitCount - int(gr.validBits)) / 8
		if bytesToRead > gr.positionFF-gr.position {
			bytesToRead = gr.positionFF - gr.position
		}
		if bytesToRead > 8 {
			bytesToRead = 8
		}

		// Read bytes big-endian and add to cache
		for i := 0; i < bytesToRead; i++ {
			b := uint64(gr.data[gr.position])
			gr.readCache |= b << (cacheTBitCount - 8 - int(gr.validBits))
			gr.validBits += 8
			gr.position++
		}

		return gr.validBits >= maxReadableCacheBits
	}
	return false
}

// fillReadCache fills the read cache with bits from the stream
// Matches CharLS fill_read_cache (decoder_strategy.h:206-255)
func (gr *GolombReader) fillReadCache() error {
	// Try optimistic (fast) path first
	if gr.fillReadCacheOptimistic() {
		return nil
	}

	// Slow path: read byte by byte with marker detection and byte stuffing
	for gr.validBits < maxReadableCacheBits {
		// Check if we've reached end of data
		if gr.position >= gr.endPosition {
			if gr.validBits == 0 {
				// Decoding expects at least some bits
				return fmt.Errorf("unexpected end of data")
			}
			// Have some bits left, allow them to be consumed
			return nil
		}

		newByteValue := gr.data[gr.position]

		// JPEG-LS marker detection: if 0xFF is followed by high bit set, it's a marker
		if newByteValue == jpegMarkerStartByte &&
			(gr.position == gr.endPosition-1 || (gr.data[gr.position+1]&0x80) != 0) {
			// Marker detected
			if gr.validBits <= 0 {
				// Decoding expects at least some bits
				return fmt.Errorf("marker encountered with no bits in cache")
			}
			// Stop reading but allow remaining bits to be consumed
			return nil
		}

		// Add byte to cache (big-endian: new bits go to high end)
		gr.readCache |= uint64(newByteValue) << (maxReadableCacheBits - int(gr.validBits))
		gr.validBits += 8
		gr.position++

		// JPEG-LS byte stuffing: after 0xFF, decrement valid_bits
		// The stuffed bit is the high bit of the NEXT byte
		if newByteValue == jpegMarkerStartByte {
			gr.validBits--
		}
	}

	// Update positionFF to next 0xFF for optimization
	gr.findJPEGMarkerStartByte()

	return nil
}

// BitsRead returns the total number of bits read from the stream
func (gr *GolombReader) BitsRead() int64 {
	return gr.bitsRead
}
