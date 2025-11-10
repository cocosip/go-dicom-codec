package lossless

// Context holds the statistical model for a specific context
type Context struct {
	A int // Running sum of prediction errors (for bias estimation)
	N int // Count of samples in this context
	B int // Bias correction term
	C int // Error accumulation counter
}

// NewContext creates a new context with initial values
func NewContext() *Context {
	return &Context{
		A: 0,
		N: 1, // Start with 1 to avoid division by zero
		B: 0,
		C: 0,
	}
}

// ComputeGolombParameter computes the Golomb-Rice coding parameter k
// based on the current context statistics
func (ctx *Context) ComputeGolombParameter() int {
	// Compute N[Q] (the count)
	n := ctx.N

	// Compute A[Q] / N[Q] to estimate the average error magnitude
	if n == 0 {
		return 0
	}

	// Find k such that 2^k <= A[Q]/N[Q] < 2^(k+1)
	// This is optimal for Golomb-Rice coding with geometric distribution
	avgError := ctx.A / n
	if avgError == 0 {
		return 0
	}

	k := 0
	for (1 << uint(k+1)) <= avgError {
		k++
		if k > 16 { // Limit maximum k
			break
		}
	}

	return k
}

// UpdateContext updates context statistics after encoding/decoding a sample
func (ctx *Context) UpdateContext(errValue int) {
	// Update running sum A with absolute error
	ctx.A += abs(errValue)

	// Update count N
	ctx.N++

	// Bias estimation: B accumulates signed errors
	ctx.B += errValue

	// Check if we need to reset to prevent overflow
	// This is part of the JPEG-LS standard
	if ctx.N >= 64 {
		// Halve all statistics (aging mechanism)
		ctx.A = (ctx.A + 1) >> 1
		ctx.B = ctx.B >> 1
		ctx.N = (ctx.N + 1) >> 1
	}
}

// GetBias returns the bias correction for this context
func (ctx *Context) GetBias() int {
	// Bias is used to correct the prediction
	// If B[Q] <= -N[Q], bias = -1
	// If B[Q] > 0, bias = 1
	// Otherwise, bias = 0

	if ctx.B <= -ctx.N {
		return -1
	}
	if ctx.B > 0 {
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

	contexts := make([]*Context, numContexts)
	for i := range contexts {
		contexts[i] = NewContext()
	}

	return &ContextTable{
		contexts: contexts,
		maxVal:   maxVal,
		range_:   maxVal + 1,
	}
}

// GetContext returns the context for given (q1, q2, q3)
func (ct *ContextTable) GetContext(q1, q2, q3 int) *Context {
	id := ContextID(q1, q2, q3)
	if id < 0 || id >= len(ct.contexts) {
		// Safety fallback
		return ct.contexts[0]
	}
	return ct.contexts[id]
}

// MapErrorValue maps a signed error to a non-negative integer for Golomb coding
// This uses a technique called "error mapping" or "modulo reduction"
func MapErrorValue(errValue, range_ int) int {
	// Map signed error to non-negative:
	// 0 -> 0, -1 -> 1, 1 -> 2, -2 -> 3, 2 -> 4, ...
	if errValue >= 0 {
		return 2 * errValue
	}
	return -2*errValue - 1
}

// UnmapErrorValue reverses the error mapping
func UnmapErrorValue(mappedError int) int {
	// Reverse of MapErrorValue
	if mappedError%2 == 0 {
		return mappedError / 2
	}
	return -(mappedError + 1) / 2
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
