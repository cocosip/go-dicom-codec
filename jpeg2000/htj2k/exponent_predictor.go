package htj2k

// ExponentPredictor computes exponent predictors Kq and exponent bounds Uq
// for HTJ2K quads as specified in ISO/IEC 15444-15:2019 Clause 7.3.7
//
// The exponent predictor Kq is computed from neighboring quads' maximum
// magnitude exponents, and combined with the unsigned residual uq to
// produce the exponent bound Uq:
//
//   Uq = Kq + uq
//
// Where Uq bounds all sample magnitude exponents in quad q:
//   En ≤ Uq for all n ∈ {4q, 4q+1, 4q+2, 4q+3}

// ExponentPredictorComputer maintains state for computing exponent predictors
type ExponentPredictorComputer struct {
	width     int      // Width in quads
	height    int      // Height in quads
	exponents [][]int  // Maximum magnitude exponent for each quad
	gamma     [][]bool // Gamma[qx][qy] indicates if quad has >1 significant sample
}

// NewExponentPredictorComputer creates a new exponent predictor computer
func NewExponentPredictorComputer(widthInQuads, heightInQuads int) *ExponentPredictorComputer {
	exponents := make([][]int, heightInQuads)
	gamma := make([][]bool, heightInQuads)
	for i := range exponents {
		exponents[i] = make([]int, widthInQuads)
		gamma[i] = make([]bool, widthInQuads)
	}

	return &ExponentPredictorComputer{
		width:     widthInQuads,
		height:    heightInQuads,
		exponents: exponents,
		gamma:     gamma,
	}
}

// SetQuadExponents sets the maximum magnitude exponent for a quad
// and determines gamma (whether quad has more than one significant sample)
//
// Parameters:
//   - qx, qy: Quad coordinates
//   - maxExponent: Maximum magnitude exponent across all 4 samples in quad
//   - significantCount: Number of significant samples in quad (0-4)
func (e *ExponentPredictorComputer) SetQuadExponents(qx, qy int, maxExponent int, significantCount int) {
	if qx >= 0 && qx < e.width && qy >= 0 && qy < e.height {
		e.exponents[qy][qx] = maxExponent
		// Gamma is 1 if quad has more than one significant sample
		// Formula (6): γq = 1 if |{n ∈ quad q : σn = 1}| > 1
		e.gamma[qy][qx] = significantCount > 1
	}
}

// ComputePredictor computes the exponent predictor Kq for a quad
// Implements Formula (5) from Clause 7.3.7
//
// For first row of quads (qy = 0):
//   Kq = 1
//
// For other quads (qy > 0):
//   Kq = max(E'qL, E'qT) - γq
//
// Where:
//   - E'qL = max exponent of left neighbor quad
//   - E'qT = max exponent of top neighbor quad
//   - γq = 1 if current quad has >1 significant sample, 0 otherwise
//
// Returns:
//   - Kq: The exponent predictor value
func (e *ExponentPredictorComputer) ComputePredictor(qx, qy int) int {
	// First row: Kq = 1 (all quads in first row)
	if qy == 0 {
		return 1
	}

	// Non-first row: Kq = max(E'qL, E'qT) - γq

	// Get left neighbor's max exponent (E'qL)
	var E_left int
	if qx > 0 {
		E_left = e.exponents[qy][qx-1]
	} else {
		// No left neighbor (leftmost column) - use 0
		E_left = 0
	}

	// Get top neighbor's max exponent (E'qT)
	var E_top int
	if qy > 0 {
		E_top = e.exponents[qy-1][qx]
	} else {
		// No top neighbor (should not happen since qy > 0 here)
		E_top = 0
	}

	// Kq = max(E'qL, E'qT) - γq
	maxE := E_left
	if E_top > maxE {
		maxE = E_top
	}

	// Subtract gamma if current quad has more than one significant sample
	Kq := maxE
	if e.gamma[qy][qx] {
		Kq = Kq - 1
	}

	// Ensure Kq is at least 0
	if Kq < 0 {
		Kq = 0
	}

	return Kq
}

// ComputeExponentBound computes the exponent bound Uq for a quad
// Formula: Uq = Kq + uq
//
// Parameters:
//   - qx, qy: Quad coordinates
//   - uq: Unsigned residual value decoded from U-VLC
//
// Returns:
//   - Uq: The exponent bound for this quad
func (e *ExponentPredictorComputer) ComputeExponentBound(qx, qy int, uq uint32) int {
	Kq := e.ComputePredictor(qx, qy)
	Uq := Kq + int(uq)
	return Uq
}

// MagnitudeExponent computes the magnitude exponent En from magnitude μn
// Formula (F.1): En = ⌈log2(μn)⌉ for μn > 0, En = 0 for μn = 0
//
// This is computed by counting the number of bits needed to represent μn-1,
// which is equivalent to counting leading zeros in the binary representation
// of 2*μn-1 and subtracting from the total bit width.
//
// Parameters:
//   - magnitude: The sample magnitude μn (unsigned)
//
// Returns:
//   - En: The magnitude exponent (0 if magnitude is 0)
func MagnitudeExponent(magnitude uint32) int {
	if magnitude == 0 {
		return 0
	}

	// Count leading zeros to find the position of the MSB
	// En = floor(log2(magnitude)) + 1 for magnitude > 0
	exponent := 0
	temp := magnitude
	for temp > 0 {
		exponent++
		temp >>= 1
	}

	return exponent
}

// QuadMaxExponent computes the maximum magnitude exponent for a quad
// Given 4 sample magnitudes, returns max(E0, E1, E2, E3)
//
// Parameters:
//   - mag0, mag1, mag2, mag3: Magnitudes of the 4 samples in the quad
//
// Returns:
//   - maxE: Maximum exponent across all 4 samples
//   - sigCount: Number of significant samples (magnitude > 0)
func QuadMaxExponent(mag0, mag1, mag2, mag3 uint32) (maxE int, sigCount int) {
	exponents := [4]int{
		MagnitudeExponent(mag0),
		MagnitudeExponent(mag1),
		MagnitudeExponent(mag2),
		MagnitudeExponent(mag3),
	}

	maxE = 0
	sigCount = 0

	for _, e := range exponents {
		if e > maxE {
			maxE = e
		}
		if e > 0 {
			sigCount++
		}
	}

	return maxE, sigCount
}
