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
// Indexed by: (h << 2) | v, where:
//   h = horizontal significance contribution (0-2)
//   v = vertical significance contribution (0-2)
// Currently unused - context is computed dynamically in getZeroCodingContext
// var lut_ctxno_zc = [16]uint8{
// 	0, 1, 1, 2, 1, 2, 2, 2,
// 	1, 2, 2, 2, 2, 2, 2, 2,
// }

// lut_ctxno_sc - Sign Coding context lookup
// Maps neighbor signs to context 9-13
// Indexed by neighbor sign configuration
var lut_ctxno_sc = [256]uint8{
	// Pre-computed based on horizontal and vertical sign contributions
	// This table is filled during initialization
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
var lut_spb = [256]int{
	// Pre-computed based on horizontal and vertical sign contributions
	// 0 = predict positive, 1 = predict negative
}

// lut_nmsedec_sig - Normalized Mean Square Error reduction for significance
// Used in rate-distortion optimization (for future encoder implementation)
// var lut_nmsedec_sig = [1 << 14]int32{}

// lut_nmsedec_ref - Normalized Mean Square Error reduction for refinement
// Used in rate-distortion optimization (for future encoder implementation)
// var lut_nmsedec_ref = [1 << 14]int32{}

// init initializes the lookup tables
func init() {
	initSignContextLUT()
	initSignPredictionLUT()
}

// initSignContextLUT initializes the sign coding context lookup table
func initSignContextLUT() {
	for i := 0; i < 256; i++ {
		// Extract neighbor sign contributions
		// Horizontal contribution: east and west neighbors
		h := 0
		if i&1 != 0 { // East positive
			h++
		}
		if i&2 != 0 { // East negative
			h--
		}
		if i&4 != 0 { // West positive
			h++
		}
		if i&8 != 0 { // West negative
			h--
		}

		// Vertical contribution: north and south neighbors
		v := 0
		if i&16 != 0 { // North positive
			v++
		}
		if i&32 != 0 { // North negative
			v--
		}
		if i&64 != 0 { // South positive
			v++
		}
		if i&128 != 0 { // South negative
			v--
		}

		// Map (h, v) to context 0-4 (relative to CTX_SC_START)
		if h < 0 {
			// h is negative, use absolute value for context determination
			if v < 0 {
				lut_ctxno_sc[i] = 4 // Absolute: 13
			} else if v == 0 {
				lut_ctxno_sc[i] = 3 // Absolute: 12
			} else {
				lut_ctxno_sc[i] = 2 // Absolute: 11
			}
		} else if h == 0 {
			if v < 0 {
				lut_ctxno_sc[i] = 3 // Absolute: 12
			} else if v == 0 {
				lut_ctxno_sc[i] = 2 // Absolute: 11
			} else {
				lut_ctxno_sc[i] = 1 // Absolute: 10
			}
		} else {
			if v < 0 {
				lut_ctxno_sc[i] = 2 // Absolute: 11
			} else if v == 0 {
				lut_ctxno_sc[i] = 1 // Absolute: 10
			} else {
				lut_ctxno_sc[i] = 0 // Absolute: 9
			}
		}
	}
}

// initSignPredictionLUT initializes the sign prediction lookup table
func initSignPredictionLUT() {
	for i := 0; i < 256; i++ {
		// Extract neighbor sign contributions
		h := 0
		if i&1 != 0 { // East positive
			h++
		}
		if i&2 != 0 { // East negative
			h--
		}
		if i&4 != 0 { // West positive
			h++
		}
		if i&8 != 0 { // West negative
			h--
		}

		v := 0
		if i&16 != 0 { // North positive
			v++
		}
		if i&32 != 0 { // North negative
			v--
		}
		if i&64 != 0 { // South positive
			v++
		}
		if i&128 != 0 { // South negative
			v--
		}

		// Sign prediction based on neighbor signs
		// Predict the most common sign
		if h+v < 0 {
			lut_spb[i] = 1 // Predict negative
		} else {
			lut_spb[i] = 0 // Predict positive
		}
	}
}

// getSignCodingContext returns the sign coding context for a coefficient
// based on its neighbor signs
func getSignCodingContext(flags uint32) uint8 {
	// Extract neighbor sign bits
	idx := 0
	if flags&T1_SIGN_E != 0 {
		if flags&T1_SIG_E != 0 {
			idx |= 1 // East positive
		} else {
			idx |= 2 // East negative
		}
	}
	if flags&T1_SIGN_W != 0 {
		if flags&T1_SIG_W != 0 {
			idx |= 4 // West positive
		} else {
			idx |= 8 // West negative
		}
	}
	if flags&T1_SIGN_N != 0 {
		if flags&T1_SIG_N != 0 {
			idx |= 16 // North positive
		} else {
			idx |= 32 // North negative
		}
	}
	if flags&T1_SIGN_S != 0 {
		if flags&T1_SIG_S != 0 {
			idx |= 64 // South positive
		} else {
			idx |= 128 // South negative
		}
	}

	return lut_ctxno_sc[idx] + CTX_SC_START
}

// getZeroCodingContext returns the zero coding context for a coefficient
// based on its neighbor significance
func getZeroCodingContext(flags uint32) uint8 {
	// Count significant neighbors in each direction
	h := 0 // Horizontal
	v := 0 // Vertical
	d := 0 // Diagonal

	if flags&T1_SIG_E != 0 {
		h++
	}
	if flags&T1_SIG_W != 0 {
		h++
	}
	if flags&T1_SIG_N != 0 {
		v++
	}
	if flags&T1_SIG_S != 0 {
		v++
	}

	if flags&T1_SIG_NE != 0 {
		d++
	}
	if flags&T1_SIG_NW != 0 {
		d++
	}
	if flags&T1_SIG_SE != 0 {
		d++
	}
	if flags&T1_SIG_SW != 0 {
		d++
	}

	// Compute context based on weighted contributions
	// h, v have weight 2, d has weight 1
	sum := h + h + v + v + d

	if sum >= 9 {
		return 8 + CTX_ZC_START
	}
	if sum >= 7 {
		return 7 + CTX_ZC_START
	}
	if sum >= 5 {
		return 6 + CTX_ZC_START
	}
	if sum >= 4 {
		return 5 + CTX_ZC_START
	}
	if sum >= 3 {
		return 4 + CTX_ZC_START
	}
	if sum >= 2 {
		return 3 + CTX_ZC_START
	}
	if sum == 1 {
		if h != 0 {
			return 2 + CTX_ZC_START
		}
		return 1 + CTX_ZC_START
	}
	return 0 + CTX_ZC_START
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
func getSignPrediction(flags uint32) int {
	// Extract neighbor sign bits (same as in getSignCodingContext)
	idx := 0
	if flags&T1_SIGN_E != 0 {
		if flags&T1_SIG_E != 0 {
			idx |= 1
		} else {
			idx |= 2
		}
	}
	if flags&T1_SIGN_W != 0 {
		if flags&T1_SIG_W != 0 {
			idx |= 4
		} else {
			idx |= 8
		}
	}
	if flags&T1_SIGN_N != 0 {
		if flags&T1_SIG_N != 0 {
			idx |= 16
		} else {
			idx |= 32
		}
	}
	if flags&T1_SIGN_S != 0 {
		if flags&T1_SIG_S != 0 {
			idx |= 64
		} else {
			idx |= 128
		}
	}

	return lut_spb[idx]
}
