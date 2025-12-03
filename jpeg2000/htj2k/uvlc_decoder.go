package htj2k

import "fmt"

// UVLCDecoder implements U-VLC (Unsigned Variable Length Code) decoding
// for HTJ2K as specified in ISO/IEC 15444-15:2019 Clause 7.3.6
//
// The U-VLC code is used to decode unsigned residual values uq for quads
// where the unsigned residual offset ulf is 1 (i.e., quad has non-zero residual).
//
// The decoding is done in three stages:
// 1. Prefix decoding (decodeUPrefix) - variable length prefix
// 2. Suffix decoding (decodeUSuffix) - fixed length suffix based on prefix
// 3. Extension decoding (decodeUExtension) - optional 4-bit extension
//
// Formula: u = u_pfx + u_sfx + 4*u_ext  (Equation 3)
//
// Special case for initial line-pair quad-pairs where both quads have ulf=1:
// Formula: u = 2 + u_pfx + u_sfx + 4*u_ext  (Equation 4)
type UVLCDecoder struct {
	bitReader BitReader
}

// NewUVLCDecoder creates a new U-VLC decoder with the given bit reader
func NewUVLCDecoder(reader BitReader) *UVLCDecoder {
	return &UVLCDecoder{
		bitReader: reader,
	}
}

// BitReader interface for reading bits from VLC stream
type BitReader interface {
	// ReadBit reads a single bit (returns 0 or 1)
	ReadBit() (uint8, error)
	// ReadBits reads n bits in little-endian order (LSB first)
	ReadBitsLE(n int) (uint32, error)
}

// DecodeUnsignedResidual decodes an unsigned residual value uq
// This is the main entry point for U-VLC decoding (Clause 7.3.6)
//
// Returns:
//   - u: The decoded unsigned residual value (1-based)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidual() (uint32, error) {
	// Step 1: Decode U-VLC prefix
	u_pfx, err := d.decodeUPrefix()
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC prefix: %w", err)
	}

	// Step 2: Decode U-VLC suffix (depends on prefix)
	u_sfx, err := d.decodeUSuffix(u_pfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC suffix: %w", err)
	}

	// Step 3: Decode U-VLC extension (depends on suffix)
	u_ext, err := d.decodeUExtension(u_sfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC extension: %w", err)
	}

	// Formula (3): u = u_pfx + u_sfx + 4*u_ext
	u := uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext)

	return u, nil
}

// DecodeUnsignedResidualInitialPair decodes unsigned residual for initial line-pair
// where both quads in the pair have ulf=1 (Equation 4)
//
// Returns:
//   - u: The decoded unsigned residual value (2-based for initial pair)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidualInitialPair() (uint32, error) {
	// Step 1: Decode U-VLC prefix
	u_pfx, err := d.decodeUPrefix()
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC prefix: %w", err)
	}

	// Step 2: Decode U-VLC suffix (depends on prefix)
	u_sfx, err := d.decodeUSuffix(u_pfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC suffix: %w", err)
	}

	// Step 3: Decode U-VLC extension (depends on suffix)
	u_ext, err := d.decodeUExtension(u_sfx)
	if err != nil {
		return 0, fmt.Errorf("failed to decode U-VLC extension: %w", err)
	}

	// Formula (4): u = 2 + u_pfx + u_sfx + 4*u_ext
	u := 2 + uint32(u_pfx) + uint32(u_sfx) + 4*uint32(u_ext)

	return u, nil
}

// DecodeUnsignedResidualSecondQuad decodes the second quad's residual when first quad has uq1 > 2
// In this special case, the prefix decoding is simplified to a single bit (Clause 7.3.6)
//
// Returns:
//   - u: The decoded unsigned residual value (1 or 2)
//   - error: Error if decoding fails
func (d *UVLCDecoder) DecodeUnsignedResidualSecondQuad() (uint32, error) {
	// Read single bit ubit
	ubit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, fmt.Errorf("failed to read ubit: %w", err)
	}

	// u_pfx = ubit + 1, and final value is also ubit + 1
	u := uint32(ubit) + 1

	return u, nil
}

// decodeUPrefix decodes the U-VLC prefix component
// Implements the decodeUPrefix procedure from Clause 7.3.6
//
// From spec procedure:
//   bit = importVLCBit
//   if(bit == 1) return 1
//   bit = importVLCBit
//   if(bit == 1) return 2
//   bit = importVLCBit
//   return (bit == 1) ? 3 : 5
//
// This means:
//   prefix="1"   -> u_pfx=1
//   prefix="01"  -> u_pfx=2
//   prefix="001" -> u_pfx=3
//   prefix="000" -> u_pfx=5
//
// Note: u_pfx can ONLY be 1, 2, 3, or 5 (never 4)
//
// Returns:
//   - u_pfx: Prefix value (1, 2, 3, or 5)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUPrefix() (uint8, error) {
	// Read first bit
	bit, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "1" -> u_pfx = 1
		return 1, nil
	}

	// First bit was 0, read second bit
	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "01" -> u_pfx = 2
		return 2, nil
	}

	// First two bits were "00", read third bit
	bit, err = d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	if bit == 1 {
		// Prefix = "001" -> u_pfx = 3
		return 3, nil
	}

	// Prefix = "000" -> u_pfx = 5
	return 5, nil
}

// decodeUSuffix decodes the U-VLC suffix component
// Implements the decodeUSuffix procedure from Clause 7.3.6
//
// From spec procedure:
//   if(u_pfx < 3) return 0
//   val = importVLCBit
//   if(u_pfx == 3) return val
//   for(i=1; i<5; i++)
//     bit = importVLCBit
//     val = val + (bit << i)
//   return val
//
// This means:
//   u_pfx=1,2: no suffix (return 0)
//   u_pfx=3:   1-bit suffix
//   u_pfx=5:   5-bit suffix
//
// Suffix is read in little-endian order (LSB first)
//
// Returns:
//   - u_sfx: Suffix value (0 if no suffix needed)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUSuffix(u_pfx uint8) (uint8, error) {
	// if(u_pfx < 3) return 0
	if u_pfx < 3 {
		return 0, nil
	}

	// val = importVLCBit
	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	// if(u_pfx == 3) return val
	if u_pfx == 3 {
		return val, nil
	}

	// for(i=1; i<5; i++)
	//   bit = importVLCBit
	//   val = val + (bit << i)
	for i := 1; i < 5; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}

// decodeUExtension decodes the U-VLC extension component
// Implements the decodeUExtension procedure from Clause 7.3.6
//
// From spec procedure:
//   if(u_sfx < 28) return 0
//   val = importVLCBit
//   for(i=1; i<4; i++)
//     bit = importVLCBit
//     val = val + (bit << i)
//   return val
//
// Extension is only present when u_sfx >= 28 (4-bit extension in little-endian)
// This handles very large magnitude values that exceed 2^37
//
// Returns:
//   - u_ext: Extension value (0 if no extension needed)
//   - error: Error if decoding fails
func (d *UVLCDecoder) decodeUExtension(u_sfx uint8) (uint8, error) {
	// if(u_sfx < 28) return 0
	if u_sfx < 28 {
		return 0, nil
	}

	// val = importVLCBit
	val, err := d.bitReader.ReadBit()
	if err != nil {
		return 0, err
	}

	// for(i=1; i<4; i++)
	//   bit = importVLCBit
	//   val = val + (bit << i)
	for i := 1; i < 4; i++ {
		bit, err := d.bitReader.ReadBit()
		if err != nil {
			return 0, err
		}
		val = val + (bit << i)
	}

	return val, nil
}
