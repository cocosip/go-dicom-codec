package t1

// EBCOT Tier-1 Context Modeling
// Reference: ISO/IEC 15444-1:2019 Annex D
// Based on OpenJPEG t1.c implementation

// Context labels for EBCOT encoding/decoding
// There are 19 context types (0-18) used in the three coding passes
const (
	// Zero Coding contexts (0-8)
	// Used in Significance Propagation and Cleanup passes
	CTX_ZC_START = 0
	CTX_ZC_END   = 8

	// Sign Coding contexts (9-13)
	// Used when encoding/decoding sign bits
	CTX_SC_START = 9
	CTX_SC_END   = 13

	// Magnitude Refinement contexts (14-16)
	// Used in Magnitude Refinement pass
	CTX_MR_START = 14
	CTX_MR_END   = 16

	// Run-Length context (17)
	// Used for run-length coding in Cleanup pass
	CTX_RL = 17

	// Uniform context (18)
	// Used for sign magnitude bits
	CTX_UNI = 18

	// Total number of contexts
	NUM_CONTEXTS = 19
)

// Code-block style flags (ISO/IEC 15444-1 Table A.18)
const (
	CblkStyleLazy   = 0x01
	CblkStyleReset  = 0x02
	CblkStyleTermAll = 0x04
	CblkStyleVSC    = 0x08
	CblkStylePterm  = 0x10
	CblkStyleSegsym = 0x20
)

// Coefficient state flags
// Each coefficient in a code-block has associated state flags
const (
	// Significance flag - coefficient is significant (non-zero)
	T1_SIG = 0x0001

	// Refinement flag - coefficient has been refined
	T1_REFINE = 0x0002

	// Visit flag - coefficient has been visited in current pass
	T1_VISIT = 0x0004

	// Neighbor significance flags (8 directions)
	T1_SIG_N  = 0x0010 // North (above)
	T1_SIG_S  = 0x0020 // South (below)
	T1_SIG_W  = 0x0040 // West (left)
	T1_SIG_E  = 0x0080 // East (right)
	T1_SIG_NW = 0x0100 // Northwest
	T1_SIG_NE = 0x0200 // Northeast
	T1_SIG_SW = 0x0400 // Southwest
	T1_SIG_SE = 0x0800 // Southeast

	// Mask for all neighbor significance flags
	T1_SIG_NEIGHBORS = T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E |
		T1_SIG_NW | T1_SIG_NE | T1_SIG_SW | T1_SIG_SE

	// Sign flag - coefficient sign (0 = positive, 1 = negative)
	T1_SIGN = 0x1000

	// Neighbor sign flags
	T1_SIGN_N = 0x2000
	T1_SIGN_S = 0x4000
	T1_SIGN_W = 0x8000
	T1_SIGN_E = 0x10000
)

// Context lookup tables
// These tables map neighbor configurations to context labels

// lut_ctxno_zc - Zero Coding context lookup
// Maps neighbor significance to context 0-8
// Full table size: 2048 entries = 4 orientations × 512 neighbor configurations
// Each orientation is 512 entries for all 9-bit neighbor significance patterns
// Index: orientation_offset + (flags & T1_SIGMA_NEIGHBOURS)
//   where orientation_offset = orientation * 512
// Reference: OpenJPEG t1_luts.h
var lut_ctxno_zc = [2048]uint8{
	// Orientation 0 (0-511)
	0, 1, 3, 3, 1, 2, 3, 3, 5, 6, 7, 7, 6, 6, 7, 7, 0, 1, 3, 3, 1, 2, 3, 3, 5, 6, 7, 7, 6, 6, 7, 7,
	5, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 5, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	2, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 2, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	// Orientation 1 (512-1023)
	0, 1, 5, 6, 1, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7, 0, 1, 5, 6, 1, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7,
	3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7, 3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7,
	1, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7, 1, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7,
	3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7, 3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7,
	5, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 5, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	1, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7, 1, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7,
	3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7, 3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7,
	2, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7, 2, 2, 6, 6, 2, 2, 6, 6, 3, 3, 7, 7, 3, 3, 7, 7,
	3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7, 3, 3, 7, 7, 3, 3, 7, 7, 4, 4, 7, 7, 4, 4, 7, 7,
	6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 6, 6, 8, 8, 6, 6, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8, 7, 7, 8, 8,
	// Orientation 2 (1024-1535)
	0, 1, 3, 3, 1, 2, 3, 3, 5, 6, 7, 7, 6, 6, 7, 7, 0, 1, 3, 3, 1, 2, 3, 3, 5, 6, 7, 7, 6, 6, 7, 7,
	5, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 5, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 1, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	2, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7, 2, 2, 3, 3, 2, 2, 3, 3, 6, 6, 7, 7, 6, 6, 7, 7,
	6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 6, 6, 7, 7, 6, 6, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7, 3, 3, 4, 4, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8, 7, 7, 7, 7, 7, 7, 7, 7, 8, 8, 8, 8, 8, 8, 8, 8,
	// Orientation 3 (1536-2047)
	0, 3, 1, 4, 3, 6, 4, 7, 1, 4, 2, 5, 4, 7, 5, 7, 0, 3, 1, 4, 3, 6, 4, 7, 1, 4, 2, 5, 4, 7, 5, 7,
	1, 4, 2, 5, 4, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7, 1, 4, 2, 5, 4, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7,
	3, 6, 4, 7, 6, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 3, 6, 4, 7, 6, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8,
	4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	1, 4, 2, 5, 4, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7, 1, 4, 2, 5, 4, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7,
	2, 5, 2, 5, 5, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7, 2, 5, 2, 5, 5, 7, 5, 7,
	4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	3, 6, 4, 7, 6, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 3, 6, 4, 7, 6, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8,
	4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	6, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 6, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8,
	7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8,
	4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 4, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8, 5, 7, 5, 7, 7, 8, 7, 8,
	7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8,
	7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8, 7, 8, 7, 8, 8, 8, 8, 8,
}

// lut_ctxno_sc - Sign Coding context lookup
// Maps neighbor signs to context 9-13
// Indexed by OpenJPEG bit layout (8 bits):
//   Bit 0: T1_LUT_SGN_W - West neighbor sign
//   Bit 1: T1_LUT_SIG_N - North neighbor significance
//   Bit 2: T1_LUT_SGN_E - East neighbor sign
//   Bit 3: T1_LUT_SIG_W - West neighbor significance
//   Bit 4: T1_LUT_SGN_N - North neighbor sign
//   Bit 5: T1_LUT_SIG_E - East neighbor significance
//   Bit 6: T1_LUT_SGN_S - South neighbor sign
//   Bit 7: T1_LUT_SIG_S - South neighbor significance
// Reference: OpenJPEG t1_luts.h
var lut_ctxno_sc = [256]uint8{
	0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb,
	0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd,
	0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0x9, 0xd, 0xa, 0x9, 0xc, 0xa, 0xb,
	0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0x9, 0xb, 0xa, 0x9, 0xc, 0xa, 0xd,
	0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb,
	0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd,
	0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0x9, 0xd, 0xa, 0x9, 0xc, 0xa, 0xb,
	0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0x9, 0xb, 0xa, 0x9, 0xc, 0xa, 0xd,
	0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb,
	0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc,
	0xd, 0xd, 0xd, 0xd, 0xb, 0xb, 0xb, 0xb, 0xd, 0xa, 0xd, 0xa, 0xa, 0xb, 0xa, 0xb,
	0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xa, 0xc, 0x9, 0xa, 0xb, 0x9, 0xc,
	0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc,
	0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd,
	0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xa, 0xc, 0x9, 0xa, 0xd, 0x9, 0xc,
	0xb, 0xb, 0xb, 0xb, 0xd, 0xd, 0xd, 0xd, 0xb, 0xa, 0xb, 0xa, 0xa, 0xd, 0xa, 0xd,
}

// lut_ctxno_mag - Magnitude Refinement context lookup
// Maps neighbor significance to context 14-16
// Currently unused - context is computed dynamically in getMagRefinementContext
// var lut_ctxno_mag = [16]uint8{
// 	14, 15, 15, 15, 15, 16, 16, 16,
// 	15, 16, 16, 16, 16, 16, 16, 16,
// }

// lut_spb - Sign bit prediction lookup
// Predicts the sign bit based on neighbor signs
// Uses same indexing as lut_ctxno_sc (OpenJPEG bit layout)
// 0 = predict positive, 1 = predict negative
// Reference: OpenJPEG t1_luts.h
var lut_spb = [256]int{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
	0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
	0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1,
	0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1,
	1, 1, 0, 0, 1, 1, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
	0, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1,
}

// lut_nmsedec_sig - Normalized Mean Square Error reduction for significance
// Used in rate-distortion optimization (for future encoder implementation)
// var lut_nmsedec_sig = [1 << 14]int32{}

// lut_nmsedec_ref - Normalized Mean Square Error reduction for refinement
// Used in rate-distortion optimization (for future encoder implementation)
// var lut_nmsedec_ref = [1 << 14]int32{}

// Note: lut_ctxno_sc and lut_spb are now pre-initialized with OpenJPEG values
// No init() function needed

// getSignCodingContext returns the sign coding context for a coefficient
// based on its neighbor signs and significance
// Uses OpenJPEG bit layout for LUT indexing:
//   Bit 0: West sign, Bit 1: North sig, Bit 2: East sign, Bit 3: West sig
//   Bit 4: North sign, Bit 5: East sig, Bit 6: South sign, Bit 7: South sig
func getSignCodingContext(flags uint32) uint8 {
	idx := uint8(0)

	// West neighbor: Bit 0 (sign), Bit 3 (significance)
	if flags&T1_SIG_W != 0 {
		idx |= (1 << 3) // West is significant
		if flags&T1_SIGN_W != 0 {
			idx |= (1 << 0) // West sign (negative)
		}
	}

	// North neighbor: Bit 4 (sign), Bit 1 (significance)
	if flags&T1_SIG_N != 0 {
		idx |= (1 << 1) // North is significant
		if flags&T1_SIGN_N != 0 {
			idx |= (1 << 4) // North sign (negative)
		}
	}

	// East neighbor: Bit 2 (sign), Bit 5 (significance)
	if flags&T1_SIG_E != 0 {
		idx |= (1 << 5) // East is significant
		if flags&T1_SIGN_E != 0 {
			idx |= (1 << 2) // East sign (negative)
		}
	}

	// South neighbor: Bit 6 (sign), Bit 7 (significance)
	if flags&T1_SIG_S != 0 {
		idx |= (1 << 7) // South is significant
		if flags&T1_SIGN_S != 0 {
			idx |= (1 << 6) // South sign (negative)
		}
	}

	return lut_ctxno_sc[idx]
}

// getZeroCodingContext returns the zero coding context for a coefficient
// based on its neighbor significance and subband orientation.
// 9-bit neighbor significance index layout:
//   Bit 0: NW, Bit 1: N, Bit 2: NE
//   Bit 3: W,  Bit 4: (unused), Bit 5: E
//   Bit 6: SW, Bit 7: S, Bit 8: SE
func getZeroCodingContext(flags uint32, orient int) uint8 {
	// Build 9-bit index according to OpenJPEG layout
	idx := uint16(0)

	if flags&T1_SIG_NW != 0 {
		idx |= (1 << 0)
	}
	if flags&T1_SIG_N != 0 {
		idx |= (1 << 1)
	}
	if flags&T1_SIG_NE != 0 {
		idx |= (1 << 2)
	}
	if flags&T1_SIG_W != 0 {
		idx |= (1 << 3)
	}
	// Bit 4 is unused (current position)
	if flags&T1_SIG_E != 0 {
		idx |= (1 << 5)
	}
	if flags&T1_SIG_SW != 0 {
		idx |= (1 << 6)
	}
	if flags&T1_SIG_S != 0 {
		idx |= (1 << 7)
	}
	if flags&T1_SIG_SE != 0 {
		idx |= (1 << 8)
	}

	if orient < 0 || orient > 3 {
		orient = 0
	}
	offset := uint16(orient) * 512
	return lut_ctxno_zc[offset+idx]
}

// getMagRefinementContext returns the magnitude refinement context
func getMagRefinementContext(flags uint32) uint8 {
	// Count significant neighbors
	sum := 0
	if flags&T1_SIG_E != 0 {
		sum++
	}
	if flags&T1_SIG_W != 0 {
		sum++
	}
	if flags&T1_SIG_N != 0 {
		sum++
	}
	if flags&T1_SIG_S != 0 {
		sum++
	}
	if flags&T1_SIG_NE != 0 {
		sum++
	}
	if flags&T1_SIG_NW != 0 {
		sum++
	}
	if flags&T1_SIG_SE != 0 {
		sum++
	}
	if flags&T1_SIG_SW != 0 {
		sum++
	}

	if sum >= 3 {
		return 2 + CTX_MR_START
	}
	if sum >= 1 {
		return 1 + CTX_MR_START
	}
	return 0 + CTX_MR_START
}

// getSignPrediction returns the predicted sign based on neighbor signs
// Uses same OpenJPEG bit layout as getSignCodingContext
func getSignPrediction(flags uint32) int {
	idx := uint8(0)

	// West neighbor: Bit 0 (sign), Bit 3 (significance)
	if flags&T1_SIG_W != 0 {
		idx |= (1 << 3) // West is significant
		if flags&T1_SIGN_W != 0 {
			idx |= (1 << 0) // West sign (negative)
		}
	}

	// North neighbor: Bit 4 (sign), Bit 1 (significance)
	if flags&T1_SIG_N != 0 {
		idx |= (1 << 1) // North is significant
		if flags&T1_SIGN_N != 0 {
			idx |= (1 << 4) // North sign (negative)
		}
	}

	// East neighbor: Bit 2 (sign), Bit 5 (significance)
	if flags&T1_SIG_E != 0 {
		idx |= (1 << 5) // East is significant
		if flags&T1_SIGN_E != 0 {
			idx |= (1 << 2) // East sign (negative)
		}
	}

	// South neighbor: Bit 6 (sign), Bit 7 (significance)
	if flags&T1_SIG_S != 0 {
		idx |= (1 << 7) // South is significant
		if flags&T1_SIGN_S != 0 {
			idx |= (1 << 6) // South sign (negative)
		}
	}

	return lut_spb[idx]
}

// GetSignContextLUT returns the sign context lookup table for validation
func GetSignContextLUT() [256]uint8 {
	return lut_ctxno_sc
}

// GetZeroCodingLUT returns the zero coding context lookup table for validation
func GetZeroCodingLUT() [2048]uint8 {
	return lut_ctxno_zc
}

// GetSignPredictionLUT returns the sign prediction lookup table for validation
func GetSignPredictionLUT() [256]int {
	return lut_spb
}
