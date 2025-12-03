package htj2k

// VLCEncoder implements VLC encoding (simplified stub)
type VLCEncoder struct {
	buffer []byte
}

// NewVLCEncoder creates a new VLC encoder
func NewVLCEncoder() *VLCEncoder {
	return &VLCEncoder{
		buffer: make([]byte, 0, 256),
	}
}

// EncodeQuad encodes a quad (stub implementation)
func (v *VLCEncoder) EncodeQuad(sig uint8, mag []int) {
	// Simplified VLC encoding
	v.buffer = append(v.buffer, sig)
	for _, m := range mag {
		if m > 0 {
			v.buffer = append(v.buffer, uint8(m&0xFF))
		}
	}
}

// Flush flushes any pending bits and returns the data
func (v *VLCEncoder) Flush() []byte {
	return v.buffer
}

// Length returns the length of encoded data
func (v *VLCEncoder) Length() int {
	return len(v.buffer)
}

// Bytes returns the encoded VLC data
func (v *VLCEncoder) Bytes() []byte {
	return v.buffer
}
