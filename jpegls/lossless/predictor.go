package lossless

import "github.com/cocosip/go-dicom-codec/jpegls/common"

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

// GradientQuantizer encapsulates adaptive thresholds (T1/T2/T3) and NEAR.
// It mirrors the CharLS quantization logic but allows thresholds to come
// from the computed defaults or the LSE segment.
type GradientQuantizer struct {
	T1   int
	T2   int
	T3   int
	Near int
}

// ComputeContext computes the context for the current pixel using fixed (lossless) thresholds.
// Returns gradient quantization values (Q1, Q2, Q3)
func ComputeContext(a, b, c, d int) (int, int, int) {
	// d = North-East pixel (top-right)
	// Compute gradients
	d1 := d - b // Horizontal gradient (NE - N)
	d2 := b - c // Diagonal gradient (N - NW)
	d3 := c - a // Vertical gradient (NW - W)

	// Quantize gradients using default JPEG-LS thresholds
	q1 := quantizeGradient(d1)
	q2 := quantizeGradient(d2)
	q3 := quantizeGradient(d3)

	return q1, q2, q3
}

// quantizeGradient quantizes a gradient value into discrete bins (default JPEG-LS thresholds).
func quantizeGradient(d int) int {
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

// NewGradientQuantizer creates a quantizer with thresholds and NEAR.
func NewGradientQuantizer(t1, t2, t3, near int) *GradientQuantizer {
	return &GradientQuantizer{
		T1:   t1,
		T2:   t2,
		T3:   t3,
		Near: near,
	}
}

// ComputeContext computes the context for the current pixel using adaptive thresholds.
// Returns gradient quantization values (Q1, Q2, Q3)
func (g *GradientQuantizer) ComputeContext(a, b, c, d int) (int, int, int) {
	// d = North-East pixel (top-right)
	// Compute gradients
	d1 := d - b // Horizontal gradient (NE - N)
	d2 := b - c // Diagonal gradient (N - NW)
	d3 := c - a // Vertical gradient (NW - W)

	// Quantize gradients
	q1 := g.quantizeGradient(d1)
	q2 := g.quantizeGradient(d2)
	q3 := g.quantizeGradient(d3)

	return q1, q2, q3
}

// quantizeGradient quantizes a gradient value into discrete bins using thresholds T1/T2/T3.
func (g *GradientQuantizer) quantizeGradient(d int) int {
	near := g.Near
	t1, t2, t3 := g.T1, g.T2, g.T3

	switch {
	case d <= -t3:
		return -4
	case d <= -t2:
		return -3
	case d <= -t1:
		return -2
	case d < -near:
		return -1
	case d <= near:
		return 0
	case d < t1:
		return 1
	case d < t2:
		return 2
	case d < t3:
		return 3
	default:
		return 4
	}
}

// ComputeContextID converts (Q1, Q2, Q3) to a single context ID
// This matches CharLS: compute_context_id(q1, q2, q3) = (q1 * 9 + q2) * 9 + q3
// The result can be negative (uses sign symmetry to reduce contexts)
func ComputeContextID(q1, q2, q3 int) int {
	return (q1*9+q2)*9 + q3
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
	return common.Abs(a-b) <= threshold && common.Abs(b-c) <= threshold && common.Abs(c-d) <= threshold
}
