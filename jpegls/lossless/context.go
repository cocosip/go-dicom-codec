package lossless

// Context holds the statistical model for a specific context
type Context struct {
	A int // Running sum of prediction errors (for bias estimation)
	N int // Count of samples in this context
	B int // Bias correction term
	C int // Error accumulation counter
}

// NewContext creates a new context with initial values
// The range parameter is the dynamic range (MAXVAL + 1)
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

// ComputeGolombParameter computes the Golomb-Rice coding parameter k
// based on the current context statistics
// This matches the CharLS implementation (ISO 14495-1, code segment A.10)
func (ctx *Context) ComputeGolombParameter() int {
	// Find k such that N * 2^k < A
	// This avoids precision loss from integer division
	const maxK = 16 // Maximum k value to prevent overflow

	k := 0
	for k < maxK && (ctx.N<<uint(k)) < ctx.A {
		k++
	}

	return k
}

// UpdateContext updates context statistics after encoding/decoding a sample
// This implements ISO 14495-1, Code segment A.12 – Variables update
func (ctx *Context) UpdateContext(errValue int) {
	const nearLossless = 0 // For lossless mode, NEAR = 0
	const resetThreshold = 64
	const maxC = 127
	const minC = -128

	// A.12: Update A[Q]
	ctx.A += abs(errValue)

	// A.12: Update B[Q] with (2*NEAR + 1) * Errval
	// For lossless (NEAR=0), this is just Errval
	ctx.B += errValue * (2*nearLossless + 1)

	// A.12: Check for reset (when N reaches threshold, halve all values)
	if ctx.N == resetThreshold {
		ctx.A >>= 1
		ctx.B >>= 1
		ctx.N >>= 1
	}

	// A.12: Increment N[Q]
	ctx.N++

	// A.13: Update of bias-related variables B[Q] and C[Q]
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

// GetPredictionCorrection returns the C value used to correct predictions
// In JPEG-LS, C is used to bias the prediction, not B
func (ctx *Context) GetPredictionCorrection() int {
	return ctx.C
}

// GetErrorCorrection returns the error correction value for the given Golomb parameter k
// This implements the CharLS get_error_correction function
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
	} else if val > 0 {
		return 1
	}
	return 0
}

// ContextTable holds all contexts for encoding/decoding
type ContextTable struct {
	contexts []*Context
	maxVal   int // Maximum sample value (e.g., 255 for 8-bit)
	range_   int // Dynamic range
}

// NewContextTable creates a new context table
func NewContextTable(maxVal int) *ContextTable {
	// Total number of contexts = 9 * 9 * 9 = 729
	numContexts := 729
	range_ := maxVal + 1

	contexts := make([]*Context, numContexts)
	for i := range contexts {
		contexts[i] = NewContext(range_)
	}

	return &ContextTable{
		contexts: contexts,
		maxVal:   maxVal,
		range_:   range_,
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

// MapErrorValue maps a signed error to a non-negative integer for Golomb coding
// This matches CharLS implementation (ISO/IEC 14495-1, A.5.2, Code Segment A.11)
// Formula: (error_value >> 30) ^ (2 * error_value)
func MapErrorValue(errValue int) int {
	// Map error value to non-negative range
	// JPEG-LS standard: MErrval = 2*|Errval|-1 if Errval<0, else 2*Errval
	// Using bit tricks for efficiency
	errValue32 := int32(errValue)
	const intBitCount = 32
	mappedError := int((errValue32 >> (intBitCount - 2)) ^ (2 * errValue32))
	return mappedError
}

// UnmapErrorValue reverses the error mapping
// This is the inverse of MapErrorValue (ISO/IEC 14495-1, A.5.2)
func UnmapErrorValue(mappedError int) int {
	// Use int32 to ensure 32-bit semantics
	mapped32 := int32(mappedError)
	const intBitCount = 32
	// Extract sign bit from LSB
	sign := int32(uint32(mapped32)<<(intBitCount-1)) >> (intBitCount - 1)
	// Reconstruct error value
	errValue := int(sign ^ (mapped32 >> 1))
	return errValue
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
