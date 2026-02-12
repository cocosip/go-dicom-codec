package common

import "io"

// HuffmanTable represents a Huffman coding table
type HuffmanTable struct {
	// Number of codes of each length (1-16 bits)
	Bits [16]int
	// Values for each code, in order of code length
	Values []byte
	// Lookup tables for fast decoding
	minCode [16]int32
	maxCode [16]int32
	valPtr  [16]int32
	// Lookup table for fast decoding of short codes
	lookupTable [256]int16 // value: (nbits << 8) | value, -1 if not found
}

// Build builds lookup tables for fast Huffman decoding
func (h *HuffmanTable) Build() error {
	// Build fast lookup table for codes up to 8 bits
	for i := range h.lookupTable {
		h.lookupTable[i] = -1
	}

	p := 0
	for l := 0; l < 8; l++ {
		for i := 0; i < h.Bits[l]; i++ {
			// Extend the code to 8 bits
			code := p << uint(7-l)
			for j := 0; j < (1 << uint(7-l)); j++ {
				h.lookupTable[code+j] = int16((l+1)<<8 | int(h.Values[p]))
			}
			p++
		}
	}

	// Build min/max codes and value pointers for codes longer than 8 bits
	code := int32(0)
	p = 0
	for l := 0; l < 16; l++ {
		if h.Bits[l] == 0 {
			h.maxCode[l] = -1
		} else {
			h.valPtr[l] = int32(p)
			h.minCode[l] = code
			p += h.Bits[l]
			code += int32(h.Bits[l])
			h.maxCode[l] = code - 1
		}
		code <<= 1
	}

	return nil
}

// HuffmanDecoder decodes Huffman-encoded data
type HuffmanDecoder struct {
	r       io.Reader
	bits    uint32 // Bit buffer
	nBits   int    // Number of bits in buffer
	readErr error  // Read error, if any
}

// NewHuffmanDecoder creates a new Huffman decoder
func NewHuffmanDecoder(r io.Reader) *HuffmanDecoder {
	return &HuffmanDecoder{r: r}
}

// ReadBit reads a single bit
func (d *HuffmanDecoder) ReadBit() (bool, error) {
	if d.readErr != nil {
		return false, d.readErr
	}

	if d.nBits == 0 {
		var b [1]byte
		_, err := io.ReadFull(d.r, b[:])
		if err != nil {
			d.readErr = err
			return false, err
		}

		// Handle byte stuffing (0xFF followed by 0x00)
		if b[0] == 0xFF {
			var b2 [1]byte
			_, err := io.ReadFull(d.r, b2[:])
			if err != nil {
				d.readErr = err
				return false, err
			}
			if b2[0] != 0x00 {
				// Found a marker, this is an error in the middle of scan data
				d.readErr = ErrInvalidData
				return false, ErrInvalidData
			}
		}

		d.bits = uint32(b[0])
		d.nBits = 8
	}

	d.nBits--
	bit := (d.bits >> uint(d.nBits)) & 1
	return bit == 1, nil
}

// ReadBits reads n bits as an unsigned integer
func (d *HuffmanDecoder) ReadBits(n int) (uint32, error) {
	if n == 0 {
		return 0, nil
	}

	for d.nBits < n {
		if d.readErr != nil {
			return 0, d.readErr
		}

		var b [1]byte
		_, err := io.ReadFull(d.r, b[:])
		if err != nil {
			d.readErr = err
			return 0, err
		}

		// Handle byte stuffing
		if b[0] == 0xFF {
			var b2 [1]byte
			_, err := io.ReadFull(d.r, b2[:])
			if err != nil {
				d.readErr = err
				return 0, err
			}
			if b2[0] != 0x00 {
				d.readErr = ErrInvalidData
				return 0, ErrInvalidData
			}
		}

		d.bits = (d.bits << 8) | uint32(b[0])
		d.nBits += 8
	}

	d.nBits -= n
	return (d.bits >> uint(d.nBits)) & ((1 << uint(n)) - 1), nil
}

// Decode decodes the next Huffman symbol
func (d *HuffmanDecoder) Decode(table *HuffmanTable) (byte, error) {
	if d.readErr != nil {
		return 0, d.readErr
	}

	// Try fast lookup for codes up to 8 bits
	if d.nBits >= 8 {
		peek := (d.bits >> uint(d.nBits-8)) & 0xFF
		entry := table.lookupTable[peek]
		if entry >= 0 {
			nbits := int(entry >> 8)
			value := byte(entry & 0xFF)
			d.nBits -= nbits
			return value, nil
		}
	}

	// Slow path: decode bit by bit
	code := uint32(0)
	for l := 0; l < 16; l++ {
		bit, err := d.ReadBit()
		if err != nil {
			return 0, err
		}

		code = (code << 1) | map[bool]uint32{false: 0, true: 1}[bit]

		if int32(code) <= table.maxCode[l] && table.maxCode[l] >= 0 {
			idx := table.valPtr[l] + int32(code) - table.minCode[l]
			if idx >= 0 && int(idx) < len(table.Values) {
				return table.Values[idx], nil
			}
		}
	}

	return 0, ErrHuffmanDecode
}

// ReceiveExtend decodes a coefficient value
// This combines RECEIVE and EXTEND operations
func (d *HuffmanDecoder) ReceiveExtend(ssss int) (int, error) {
	if ssss == 0 {
		return 0, nil
	}

	bits, err := d.ReadBits(ssss)
	if err != nil {
		return 0, err
	}

	// Extend: convert to signed value
	val := int(bits)
	if val < (1 << uint(ssss-1)) {
		val += (-1 << uint(ssss)) + 1
	}

	return val, nil
}
