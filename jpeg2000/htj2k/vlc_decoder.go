package htj2k

// VLCDecoder implements VLC decoding based on OpenJPEG's approach
// Reference: OpenJPEG t1_ht_generate_luts.c
type VLCDecoder struct {
	data      []byte
	pos       int       // Current byte position (grows backward)
	bitBuffer uint32    // Bit buffer
	bitCount  int       // Number of valid bits in buffer
	lastByte  uint8     // Last byte read (for bit-unstuffing detection)

	// Lookup tables for fast decoding (1024 entries each)
	// Key: (c_q << 7) | codeword
	tbl0 [1024]uint16  // For initial quad rows
	tbl1 [1024]uint16  // For non-initial quad rows
}

// NewVLCDecoder creates a new VLC decoder
func NewVLCDecoder(data []byte) *VLCDecoder {
	v := &VLCDecoder{
		data:     data,
		pos:      0,    // Start from beginning (no reversal)
		lastByte: 0xFF, // Initialize with encoder's vlcLast (first byte was skipped)
	}
	v.buildLookupTables()

	// Skip initial 4-bit padding
	_, _ = v.readBits(4)

	return v
}

// buildLookupTables builds the lookup tables from VLC_tbl0 and VLC_tbl1
// This follows the OpenJPEG approach in vlc_init_tables()
func (v *VLCDecoder) buildLookupTables() {
	// Build lookup table for tbl0
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F      // Extract codeword (7 bits)
		c_q := i >> 7        // Extract context (3 bits)

		// Search for matching entry in VLC_tbl0
		for j := range VLC_tbl0 {
			entry := &VLC_tbl0[j]
			if int(entry.CQ) == c_q {
				// Check if codeword matches (considering length)
				mask := (1 << entry.CwdLen) - 1
				if int(entry.Cwd) == (cwd & mask) {
					// Pack entry into uint16:
					// bits 15-12: e_k
					// bits 11-8:  e_1
					// bits 7-4:   rho
					// bits 3:     u_off
					// bits 2-0:   cwd_len
					v.tbl0[i] = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
					break
				}
			}
		}
	}

	// Build lookup table for tbl1
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		c_q := i >> 7

		for j := range VLC_tbl1 {
			entry := &VLC_tbl1[j]
			if int(entry.CQ) == c_q {
				mask := (1 << entry.CwdLen) - 1
				if int(entry.Cwd) == (cwd & mask) {
					v.tbl1[i] = (uint16(entry.EK) << 12) |
						(uint16(entry.E1) << 8) |
						(uint16(entry.Rho) << 4) |
						(uint16(entry.UOff) << 3) |
						uint16(entry.CwdLen)
					break
				}
			}
		}
	}
}

// readBits reads n bits from the bit stream (forward, LSB first)
// Implements bit-unstuffing for JPEG-style bit-stuffing
func (v *VLCDecoder) readBits(n int) (uint32, bool) {
	// Ensure we have enough bits in buffer (read forward)
	for v.bitCount < n && v.pos < len(v.data) {
		b := uint8(v.data[v.pos])
		v.pos++

		// Check for bit-unstuffing: if lastByte > 0x8F and b == 0x7F
		// Then the MSB of b is a stuffed bit and should be ignored
		if v.lastByte > 0x8F && b == 0x7F {
			// Only use lower 7 bits (MSB is stuffed)
			v.bitBuffer |= uint32(b&0x7F) << v.bitCount
			v.bitCount += 7
		} else {
			// Normal byte, use all 8 bits
			v.bitBuffer |= uint32(b) << v.bitCount
			v.bitCount += 8
		}

		v.lastByte = b
	}

	if v.bitCount < n {
		return 0, false // Not enough bits
	}

	// Extract n bits
	mask := uint32((1 << n) - 1)
	result := v.bitBuffer & mask
	v.bitBuffer >>= n
	v.bitCount -= n

	return result, true
}

// ReadBit implements BitReader interface for U-VLC integration
func (v *VLCDecoder) ReadBit() (uint8, error) {
	bit, ok := v.readBits(1)
	if !ok {
		return 0, ErrInsufficientData
	}
	return uint8(bit), nil
}

// ReadBitsLE implements BitReader interface for U-VLC integration
func (v *VLCDecoder) ReadBitsLE(n int) (uint32, error) {
	bits, ok := v.readBits(n)
	if !ok {
		return 0, ErrInsufficientData
	}
	return bits, nil
}

// DecodeInitialRow decodes VLC for initial quad row
// Returns: (rho, u_off, e_k, e_1, found)
func (v *VLCDecoder) DecodeInitialRow(context uint8) (uint8, uint8, uint8, uint8, bool) {
	// Read up to 7 bits for VLC codeword
	bits, ok := v.readBits(7)
	if !ok {
		return 0, 0, 0, 0, false
	}

	// Lookup in table: (context << 7) | codeword
	key := (uint32(context) << 7) | bits
	if key >= 1024 {
		return 0, 0, 0, 0, false
	}

	packed := v.tbl0[key]
	if packed == 0 {
		// Not found - might need fewer bits
		return 0, 0, 0, 0, false
	}

	// Extract fields
	cwd_len := packed & 0x7
	u_off := (packed >> 3) & 0x1
	rho := (packed >> 4) & 0xF
	e_1 := (packed >> 8) & 0xF
	e_k := (packed >> 12) & 0xF

	// Put back unused bits
	unusedBits := 7 - int(cwd_len)
	if unusedBits > 0 {
		v.bitBuffer |= (bits >> cwd_len) << v.bitCount
		v.bitCount += unusedBits
	}

	return uint8(rho), uint8(u_off), uint8(e_k), uint8(e_1), true
}

// DecodeNonInitialRow decodes VLC for non-initial quad row
func (v *VLCDecoder) DecodeNonInitialRow(context uint8) (uint8, uint8, uint8, uint8, bool) {
	bits, ok := v.readBits(7)
	if !ok {
		return 0, 0, 0, 0, false
	}

	key := (uint32(context) << 7) | bits
	if key >= 1024 {
		return 0, 0, 0, 0, false
	}

	packed := v.tbl1[key]
	if packed == 0 {
		return 0, 0, 0, 0, false
	}

	cwd_len := packed & 0x7
	u_off := (packed >> 3) & 0x1
	rho := (packed >> 4) & 0xF
	e_1 := (packed >> 8) & 0xF
	e_k := (packed >> 12) & 0xF

	unusedBits := 7 - int(cwd_len)
	if unusedBits > 0 {
		v.bitBuffer |= (bits >> cwd_len) << v.bitCount
		v.bitCount += unusedBits
	}

	return uint8(rho), uint8(u_off), uint8(e_k), uint8(e_1), true
}

// HasMore returns true if there are more bits to decode
func (v *VLCDecoder) HasMore() bool {
	return v.bitCount > 0 || v.pos > 0
}

// DecodeQuad decodes a quad (simplified compatibility method)
// Returns: (significance_pattern, magnitudes, found)
func (v *VLCDecoder) DecodeQuad() (uint8, []uint32, bool) {
	// Simplified implementation - read basic pattern
	// In a full implementation, this would use context and proper VLC decoding

	if !v.HasMore() {
		return 0, nil, false
	}

	// Read significance pattern (4 bits for 2x2 quad)
	sig, ok := v.readBits(4)
	if !ok {
		return 0, nil, false
	}

	// Decode magnitudes for significant samples
	mags := make([]uint32, 0, 4)
	for i := 0; i < 4; i++ {
		if (sig & (1 << i)) != 0 {
			// Sample is significant - decode magnitude
			mag, ok := v.readBits(4) // Simplified - use 4 bits
			if !ok {
				return uint8(sig), mags, true
			}
			mags = append(mags, mag)
		}
	}

	return uint8(sig), mags, true
}

// DecodeQuadWithContext decodes a quad using context-based VLC decoding
// This is the proper HTJ2K implementation using context computation
// Returns: (rho, u_off, e_k, e_1, found)
func (v *VLCDecoder) DecodeQuadWithContext(context uint8, isFirstRow bool) (uint8, uint8, uint8, uint8, bool) {
	if isFirstRow {
		return v.DecodeInitialRow(context)
	} else {
		return v.DecodeNonInitialRow(context)
	}
}
