package lossless

// MED (Median Edge Detection) predictor for JPEG-LS
// This is the LOCO-I predictor that detects horizontal or vertical edges

// Predict computes the MED prediction for the current pixel
// a = left pixel (West)
// b = top pixel (North)
// c = top-left pixel (North-West)
func Predict(a, b, c int) int {
	// MED (Median Edge Detection) algorithm:
	// if c >= max(a, b) then return min(a, b)
	// if c <= min(a, b) then return max(a, b)
	// else return a + b - c

	if c >= max(a, b) {
		return min(a, b)
	}
	if c <= min(a, b) {
		return max(a, b)
	}
	return a + b - c
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers (internal)
func min(a, b int) int {
	return Min(a, b)
}

// max returns the maximum of two integers (internal)
func max(a, b int) int {
	return Max(a, b)
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ComputeContext computes the context for the current pixel
// Used to determine which statistical model to use
// Returns gradient quantization values (Q1, Q2, Q3)
func ComputeContext(a, b, c, d int) (int, int, int) {
	// d = North-East pixel (top-right)
	// Compute gradients
	d1 := d - b  // Horizontal gradient (NE - N)
	d2 := b - c  // Diagonal gradient (N - NW)
	d3 := c - a  // Vertical gradient (NW - W)

	// Quantize gradients
	q1 := quantizeGradient(d1)
	q2 := quantizeGradient(d2)
	q3 := quantizeGradient(d3)

	return q1, q2, q3
}

// quantizeGradient quantizes a gradient value into discrete bins
// This reduces the number of contexts
func quantizeGradient(d int) int {
	// JPEG-LS standard quantization thresholds
	// These values are from the T.87 specification
	if d <= -21 {
		return -4
	} else if d <= -7 {
		return -3
	} else if d <= -3 {
		return -2
	} else if d < 0 {
		return -1
	} else if d == 0 {
		return 0
	} else if d < 3 {
		return 1
	} else if d < 7 {
		return 2
	} else if d < 21 {
		return 3
	} else {
		return 4
	}
}

// ComputeContextID converts (Q1, Q2, Q3) to a single context ID
// This matches CharLS: compute_context_id(q1, q2, q3) = (q1 * 9 + q2) * 9 + q3
// The result can be negative (uses sign symmetry to reduce contexts)
func ComputeContextID(q1, q2, q3 int) int {
	return (q1*9 + q2) * 9 + q3
}

// BitwiseSign returns the sign bit of an integer
// Matches CharLS: bit_wise_sign(i) = i >> 31
// Returns -1 if i < 0, else 0
func BitwiseSign(i int) int {
	// Arithmetic right shift propagates sign bit
	return i >> 31
}

// ApplySign applies sign symmetry
// Matches CharLS: apply_sign(i, sign) = (sign ^ i) - sign
// This maps negative context IDs to positive ones
func ApplySign(i, sign int) int {
	return (sign ^ i) - sign
}

// EdgeDetection determines if there's a horizontal or vertical edge
// Returns true if we should use run mode
func EdgeDetection(a, b, c, d int, threshold int) bool {
	// Check if values are close enough to trigger run mode
	return abs(a-b) <= threshold && abs(b-c) <= threshold && abs(c-d) <= threshold
}
