package lossless

import "github.com/cocosip/go-dicom-codec/jpegls/common"

// Context holds the statistical model for a specific context
type Context struct {
	A int // Running sum of prediction errors (for bias estimation)
	N int // Count of samples in this context
	B int // Bias correction term
	C int // Error accumulation counter
}

// NewContext creates a new context with initial values.
// The range parameter is the dynamic range (can be adjusted for NEAR>0).
func NewContext(range_ int) *Context {
	// ISO/IEC 14495-1, A.8, step 1.d and A.2.1
	// A_init = max(2, (RANGE + 32) / 64)
	aInit := max(2, (range_+32)/64)

	return &Context{
		A: aInit,
		N: 1, // Start with 1 to avoid division by zero
		B: 0,
		C: 0,
	}
}

// ComputeGolombParameter computes the Golomb-Rice coding parameter k.
// This matches the CharLS implementation (ISO 14495-1, code segment A.10)
func (ctx *Context) ComputeGolombParameter() int {
	k := 0
	for (ctx.N<<uint(k)) < ctx.A && k < 16 {
		k++
	}
	return k
}

// UpdateContext updates context statistics after encoding/decoding a sample.
// This implements ISO 14495-1, Code segment A.12 — Variables update.
func (ctx *Context) UpdateContext(errValue, nearLossless, resetThreshold int) {
	const maxC = 127
	const minC = -128
	const overflowLimit = 65536 * 256 // CharLS overflow protection

	ctx.A += common.Abs(errValue)
	ctx.B += errValue * (2*nearLossless + 1)

	// CharLS overflow protection: check if context values exceed limits
	// This indicates corrupted or invalid encoded data
	if ctx.A >= overflowLimit || common.Abs(ctx.B) >= overflowLimit {
		// In Go we can't throw errors from this function, but this should never
		// happen with valid JPEG-LS data. Just clamp to prevent overflow.
		if ctx.A >= overflowLimit {
			ctx.A = overflowLimit - 1
		}
		if ctx.B >= overflowLimit {
			ctx.B = overflowLimit - 1
		} else if ctx.B <= -overflowLimit {
			ctx.B = -overflowLimit + 1
		}
	}

	if ctx.N == resetThreshold {
		ctx.A >>= 1
		ctx.B >>= 1
		ctx.N >>= 1
	}

	ctx.N++

	if ctx.B+ctx.N <= 0 {
		ctx.B += ctx.N
		if ctx.B <= -ctx.N {
			ctx.B = -ctx.N + 1
		}
		if ctx.C > minC {
			ctx.C--
		}
	} else if ctx.B > 0 {
		ctx.B -= ctx.N
		if ctx.B > 0 {
			ctx.B = 0
		}
		if ctx.C < maxC {
			ctx.C++
		}
	}
}

// GetPredictionCorrection returns the C value used to correct predictions.
func (ctx *Context) GetPredictionCorrection() int {
	return ctx.C
}

// GetErrorCorrection returns the error correction value for the given Golomb parameter k.
// For lossless mode (NEAR=0):
//   - If k != 0: returns 0
//   - If k == 0: returns sign(2*B + N - 1)
func (ctx *Context) GetErrorCorrection(k int, nearLossless int) int {
	// If k is non-zero or using near-lossless mode, no correction
	if k != 0 || nearLossless != 0 {
		return 0
	}

	// Return sign of (2*B + N - 1)
	val := 2*ctx.B + ctx.N - 1
	if val < 0 {
		return -1
	}
	return 0
}

// ContextTable holds all contexts for encoding/decoding
type ContextTable struct {
	contexts []*Context
	maxVal   int // Maximum sample value (e.g., 255 for 8-bit)
	range_   int // Dynamic range
	near     int // NEAR parameter
	reset    int // RESET interval
}

// NewContextTable creates a new context table.
func NewContextTable(maxVal, near, reset int) *ContextTable {
	// CharLS uses 365 contexts after sign symmetry (|ID| <= 364)
	numContexts := 365
	range_ := maxVal + 1
	if near > 0 {
		range_ = (maxVal+2*near)/(2*near+1) + 1
	}

	contexts := make([]*Context, numContexts)
	for i := range contexts {
		contexts[i] = NewContext(range_)
	}

	if reset == 0 {
		reset = 64
	}

	return &ContextTable{
		contexts: contexts,
		maxVal:   maxVal,
		range_:   range_,
		near:     near,
		reset:    reset,
	}
}

// GetContext returns the context for given (q1, q2, q3)
// Note: This is the old interface, kept for compatibility
// New code should use ComputeContextID and apply sign symmetry
func (ct *ContextTable) GetContext(q1, q2, q3 int) *Context {
	id := ComputeContextID(q1, q2, q3)
	// Apply sign symmetry
	sign := BitwiseSign(id)
	id = ApplySign(id, sign)

	if id < 0 || id >= len(ct.contexts) {
		// Safety fallback
		return ct.contexts[0]
	}
	return ct.contexts[id]
}

// CodingParameters capture derived values needed for JPEG-LS coding.
type CodingParameters struct {
	MaxVal int
	Near   int
	Range  int
	Qbpp   int
	Limit  int
	T1     int
	T2     int
	T3     int
	Reset  int
}

// ComputeCodingParameters computes derived JPEG-LS parameters matching CharLS defaults.
// CharLS default_traits.h:
// - range = compute_range_parameter(maximum_sample_value, near_lossless)
// - quantized_bits_per_pixel = log2_ceil(range)
// - bits_per_pixel = log2_ceil(maximum_sample_value)
// - limit = compute_limit_parameter(bits_per_pixel)  // Uses bits_per_pixel, NOT qbpp!
func ComputeCodingParameters(maxVal, near int, reset int) CodingParameters {
	range_ := maxVal + 1
	if near > 0 {
		range_ = (maxVal+2*near)/(2*near+1) + 1
	}

	// qbpp (quantized_bits_per_pixel) = ceil(log2(range))
	qbpp := bitsLen(range_)

	// bitsPerPixel = ceil(log2(maximum_sample_value))
	// CharLS uses this for limit calculation, NOT qbpp!
	bitsPerPixel := bitsLen(maxVal)

	// LIMIT: CharLS uses bits_per_pixel (based on maxVal), not qbpp (based on range)
	// See default_traits.h line 43: limit{compute_limit_parameter(bits_per_pixel)}
	limit := 2 * (bitsPerPixel + max(8, bitsPerPixel))

	t1, t2, t3 := computeThresholds(maxVal, near)

	if reset == 0 {
		reset = 64
	}

	return CodingParameters{
		MaxVal: maxVal,
		Near:   near,
		Range:  range_,
		Qbpp:   qbpp,
		Limit:  limit,
		T1:     t1,
		T2:     t2,
		T3:     t3,
		Reset:  reset,
	}
}

// clamp helper to keep thresholds within bounds.
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// computeThresholds matches CharLS compute_default (ISO 14495-1, C.2.4.1.1.1 / Table C.3).
func computeThresholds(maxVal, near int) (int, int, int) {
	// defaults for MAXVAL=255 NEAR=0
	const (
		t1Default = 3
		t2Default = 7
		t3Default = 21
	)

	if maxVal >= 128 {
		factor := (min(maxVal, 4095) + 128) / 256
		t1 := clamp(factor*(t1Default-2)+2+3*near, near+1, maxVal)
		t2 := clamp(factor*(t2Default-3)+3+5*near, t1, maxVal)
		t3 := clamp(factor*(t3Default-4)+4+7*near, t2, maxVal)
		return t1, t2, t3
	}

	// maxVal < 128 branch from CharLS
	factor := 256 / (maxVal + 1)
	t1 := clamp(max(2, t1Default/factor+3*near), near+1, maxVal)
	t2 := clamp(max(3, t2Default/factor+5*near), t1, maxVal)
	t3 := clamp(max(4, t3Default/factor+7*near), t2, maxVal)
	return t1, t2, t3
}

// bitsLen returns ceil(log2(n)).
func bitsLen(n int) int {
	if n <= 1 {
		return 1
	}
	length := 0
	n--
	for n > 0 {
		n >>= 1
		length++
	}
	return length
}

// MapErrorValue maps signed err to non-negative (ISO/IEC 14495-1 A.5.2, CharLS apply_sign logic).
// Equivalent to (err << 1) ^ (err >> 31).
func MapErrorValue(err int) int {
	return (err << 1) ^ (err >> 31)
}

// UnmapErrorValue reverses MapErrorValue.
// Equivalent to (val >> 1) ^ (-(val & 1))
func UnmapErrorValue(val int) int {
	return (val >> 1) ^ (-(val & 1))
}

// CorrectPrediction applies bias correction and modulo reduction to prediction
func CorrectPrediction(prediction, bias, range_ int) int {
	prediction += bias

	// Apply modulo reduction to keep in valid range
	if prediction < 0 {
		prediction += range_
	} else if prediction >= range_ {
		prediction -= range_
	}

	return prediction
}
