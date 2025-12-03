package htj2k

// UVLCEncoder implements U-VLC (Unsigned Variable Length Code) encoding
// for HTJ2K as specified in ISO/IEC 15444-15:2019 Annex F.3
//
// The U-VLC encoder converts unsigned residual values into prefix,
// suffix, and extension components according to Table 3.

// UVLCCodeword represents a complete U-VLC codeword
type UVLCCodeword struct {
	Prefix    uint8 // u_pfx: prefix value (1, 2, 3, or 5)
	Suffix    uint8 // u_sfx: suffix value
	Extension uint8 // u_ext: extension value
	PrefixLen int   // lp: number of prefix bits
	SuffixLen int   // ls: number of suffix bits
	ExtLen    int   // le: number of extension bits
}

// EncodeUVLC encodes an unsigned residual value according to Table 3
//
// Parameters:
//   - u: Unsigned residual value (0 for no encoding needed)
//
// Returns:
//   - UVLCCodeword with prefix, suffix, extension components
//
// Reference: ISO/IEC 15444-15:2019, Clause 7.3.6, Table 3
func EncodeUVLC(u uint32) UVLCCodeword {
	if u == 0 {
		// No encoding needed (ulf=0 case)
		return UVLCCodeword{}
	}

	var cwd UVLCCodeword

	// Determine encoding based on Table 3
	if u == 1 {
		// u=1: prefix="1", u_pfx=1, no suffix/ext
		cwd.Prefix = 1
		cwd.PrefixLen = 1
		return cwd
	}

	if u == 2 {
		// u=2: prefix="01", u_pfx=2, no suffix/ext
		cwd.Prefix = 2
		cwd.PrefixLen = 2
		return cwd
	}

	if u >= 3 && u <= 4 {
		// u=3,4: prefix="001", u_pfx=3, 1-bit suffix
		cwd.Prefix = 3
		cwd.PrefixLen = 3
		cwd.Suffix = uint8(u - 3) // u_sfx = u - 3
		cwd.SuffixLen = 1
		return cwd
	}

	if u >= 5 && u <= 32 {
		// u=5-32: prefix="000", u_pfx=5, 5-bit suffix
		cwd.Prefix = 5
		cwd.PrefixLen = 3
		cwd.Suffix = uint8(u - 5) // u_sfx = u - 5
		cwd.SuffixLen = 5
		return cwd
	}

	// u >= 33: prefix="000", u_pfx=5, 5-bit suffix, 4-bit extension
	// Formula: u = 5 + u_sfx + 4*u_ext
	// Where u_sfx >= 28 when extension is needed
	cwd.Prefix = 5
	cwd.PrefixLen = 3

	// For u >= 33:
	// u = 5 + u_sfx + 4*u_ext
	// u - 5 = u_sfx + 4*u_ext
	// Since u_sfx can be at most 31 (5 bits), and extension starts at u_sfx=28:
	// For u=33: u_sfx=28, u_ext=0
	// For u=34: u_sfx=29, u_ext=0
	// ...
	// For u=36: u_sfx=31, u_ext=0
	// For u=37: u_sfx=28, u_ext=1 (wraps around)

	uMinus5 := u - 5

	if uMinus5 < 28 {
		// No extension needed
		cwd.Suffix = uint8(uMinus5)
		cwd.SuffixLen = 5
	} else {
		// Extension needed (u_sfx >= 28)
		// u - 5 = u_sfx + 4*u_ext
		// Solve for u_sfx and u_ext
		cwd.Extension = uint8((uMinus5 - 28) / 4)
		cwd.Suffix = 28 + uint8((uMinus5-28)%4)
		cwd.SuffixLen = 5
		cwd.ExtLen = 4
	}

	return cwd
}

// EncodeUVLCInitialPair encodes unsigned residual for initial line-pair
// where both quads have ulf=1 (uses Formula 4)
//
// Formula (4): u_in = u - 2 where u = 2 + u_pfx + u_sfx + 4*u_ext
//
// Parameters:
//   - u: Unsigned residual value (should be >= 2 for initial pair)
//
// Returns:
//   - UVLCCodeword with encoding for (u - 2)
func EncodeUVLCInitialPair(u uint32) UVLCCodeword {
	if u < 2 {
		// Invalid: initial pair should have u >= 2
		return UVLCCodeword{}
	}

	// Encode (u - 2) using standard U-VLC table
	return EncodeUVLC(u - 2)
}

// EncodePrefixBits converts u_pfx value to prefix bit pattern
//
// Prefix patterns:
//   - u_pfx=1: "1"    (1 bit)
//   - u_pfx=2: "01"   (2 bits)
//   - u_pfx=3: "001"  (3 bits)
//   - u_pfx=5: "000"  (3 bits)
//
// Returns bits in little-endian order (LSB first) for emitVLCBits
func EncodePrefixBits(u_pfx uint8) (bits uint32, length int) {
	switch u_pfx {
	case 1:
		return 1, 1 // "1"
	case 2:
		return 0b01, 2 // "01" (read right-to-left: 1, then 0)
	case 3:
		return 0b001, 3 // "001"
	case 5:
		return 0b000, 3 // "000"
	default:
		return 0, 0
	}
}

// TotalLength returns the total bit length of a U-VLC codeword
func (c *UVLCCodeword) TotalLength() int {
	return c.PrefixLen + c.SuffixLen + c.ExtLen
}

// EncodeToStream encodes the U-VLC codeword to a bit stream writer
//
// The encoding order is:
// 1. Prefix bits
// 2. Suffix bits (if any, in little-endian)
// 3. Extension bits (if any, in little-endian)
//
// Parameters:
//   - writer: Bit stream writer (e.g., VLC encoder's emitVLCBits)
func (c *UVLCCodeword) EncodeToStream(writer BitStreamWriter) error {
	// Encode prefix
	if c.PrefixLen > 0 {
		prefixBits, prefixLen := EncodePrefixBits(c.Prefix)
		if err := writer.WriteBits(prefixBits, prefixLen); err != nil {
			return err
		}
	}

	// Encode suffix (little-endian)
	if c.SuffixLen > 0 {
		if err := writer.WriteBits(uint32(c.Suffix), c.SuffixLen); err != nil {
			return err
		}
	}

	// Encode extension (little-endian)
	if c.ExtLen > 0 {
		if err := writer.WriteBits(uint32(c.Extension), c.ExtLen); err != nil {
			return err
		}
	}

	return nil
}

// BitStreamWriter interface for writing bits to VLC stream
type BitStreamWriter interface {
	WriteBits(bits uint32, length int) error
}
