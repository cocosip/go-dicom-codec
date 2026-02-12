package standard

import "io"

// HuffmanEncoder encodes data using Huffman coding
type HuffmanEncoder struct {
	w     io.Writer
	bits  uint32 // Bit buffer
	nBits int    // Number of bits in buffer
}

// NewHuffmanEncoder creates a new Huffman encoder
func NewHuffmanEncoder(w io.Writer) *HuffmanEncoder {
	return &HuffmanEncoder{w: w}
}

// WriteBits writes n bits
func (e *HuffmanEncoder) WriteBits(bits uint32, n int) error {
	if n == 0 {
		return nil
	}

	e.bits = (e.bits << uint(n)) | (bits & ((1 << uint(n)) - 1))
	e.nBits += n

	for e.nBits >= 8 {
		b := byte(e.bits >> uint(e.nBits-8))
		if err := e.writeByte(b); err != nil {
			return err
		}
		e.nBits -= 8
	}

	return nil
}

// writeByte writes a byte with byte stuffing
func (e *HuffmanEncoder) writeByte(b byte) error {
	if _, err := e.w.Write([]byte{b}); err != nil {
		return err
	}

	// Byte stuffing: if we write 0xFF, follow with 0x00
	if b == 0xFF {
		if _, err := e.w.Write([]byte{0x00}); err != nil {
			return err
		}
	}

	return nil
}

// Flush writes any remaining bits
func (e *HuffmanEncoder) Flush() error {
	if e.nBits > 0 {
		// Pad with 1s
		b := byte((e.bits << uint(8-e.nBits)) | ((1 << uint(8-e.nBits)) - 1))
		if err := e.writeByte(b); err != nil {
			return err
		}
		e.nBits = 0
		e.bits = 0
	}
	return nil
}

// HuffmanCode represents a Huffman code
type HuffmanCode struct {
	Code uint16 // The Huffman code
	Len  int    // Code length in bits
}

// BuildHuffmanCodes builds Huffman codes from a table
func BuildHuffmanCodes(table *HuffmanTable) []HuffmanCode {
	codes := make([]HuffmanCode, 256)

	code := uint16(0)
	p := 0

	for l := 0; l < 16; l++ {
		for i := 0; i < table.Bits[l]; i++ {
			if p < len(table.Values) {
				val := table.Values[p]
				codes[val] = HuffmanCode{
					Code: code,
					Len:  l + 1,
				}
				code++
				p++
			}
		}
		code <<= 1
	}

	return codes
}

// EncodeCategory encodes a coefficient category and value
func (e *HuffmanEncoder) EncodeCategory(val int) (cat int, bits uint32) {
	if val == 0 {
		return 0, 0
	}

	absVal := val
	if absVal < 0 {
		absVal = -absVal
	}

	// Find category (number of bits needed)
	cat = 1
	for (1 << uint(cat)) <= absVal {
		cat++
	}

	// Encode value
	if val > 0 {
		bits = uint32(val)
	} else {
		bits = uint32((1 << uint(cat)) + val - 1)
	}

	return cat, bits
}
