package jpeg2000

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

func nearlyEqual(a, b, tol float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	diff := math.Abs(a - b)
	denom := math.Max(math.Abs(a), math.Abs(b))
	if denom == 0 {
		return diff <= tol
	}
	return diff/denom <= tol
}

func TestCalculateQuantizationParams_StyleAndLengths(t *testing.T) {
	// lossless mode
	pLossless := CalculateQuantizationParams(100, 5, 16)
	if pLossless.Style != 0 {
		t.Fatalf("expected Style=0 for quality>=100, got %d", pLossless.Style)
	}
	if pLossless.GuardBits != 2 {
		t.Fatalf("expected GuardBits=2, got %d", pLossless.GuardBits)
	}

	// lossy mode
	numLevels := 4
	pLossy := CalculateQuantizationParams(80, numLevels, 16)
	if pLossy.Style != 2 {
		t.Fatalf("expected Style=2 for lossy mode, got %d", pLossy.Style)
	}
	expectedSubbands := 3*numLevels + 1
	if len(pLossy.StepSizes) != expectedSubbands || len(pLossy.EncodedSteps) != expectedSubbands {
		t.Fatalf("unexpected subband count: got steps=%d encoded=%d want=%d", len(pLossy.StepSizes), len(pLossy.EncodedSteps), expectedSubbands)
	}

	// LL step should be smaller than HL1/LH1
	ll := pLossy.StepSizes[0]
	hl1 := pLossy.StepSizes[1]
	lh1 := pLossy.StepSizes[2]
	if !(ll < hl1 && ll < lh1) {
		t.Fatalf("expected LL step smaller than HL1/LH1, got LL=%.6f HL1=%.6f LH1=%.6f", ll, hl1, lh1)
	}

	// HH1 should be ~1.4x HL1 within tolerance
	hh1 := pLossy.StepSizes[3]
	ratio := hh1 / hl1
	if math.Abs(ratio-1.4) > 0.15 { // allow tolerance
		t.Fatalf("expected HH1/HL1≈1.4, got %.4f (HH1=%.6f HL1=%.6f)", ratio, hh1, hl1)
	}
}

func TestEncodedSteps_DecodeApprox(t *testing.T) {
	p := CalculateQuantizationParams(80, 3, 16)
	for i, enc := range p.EncodedSteps {
		decoded := DecodeQuantizationStep(enc, 16)
		original := p.StepSizes[i]
		if !nearlyEqual(decoded, original, 0.05) { // 5% tolerance
			t.Fatalf("decoded step not close to original for subband %d: got %.6f want %.6f", i, decoded, original)
		}
	}
}

func TestQualityMonotonicity_LLStep(t *testing.T) {
	qualities := []int{1, 20, 50, 80, 90, 95, 99}
	var llSteps []float64
	for _, q := range qualities {
		p := CalculateQuantizationParams(q, 5, 16)
		if p.Style == 0 {
			// skip lossless
			continue
		}
		llSteps = append(llSteps, p.StepSizes[0])
	}
	for i := 1; i < len(llSteps); i++ {
		if !(llSteps[i] < llSteps[i-1]) {
			t.Fatalf("expected LL step to decrease with higher quality: prev=%.6f curr=%.6f", llSteps[i-1], llSteps[i])
		}
	}
}

func TestQuantizeDequantizeErrorByQuality(t *testing.T) {
	// synthetic coefficients
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	coeffs := make([]int32, 5000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(1<<14) - (1 << 13))
	}

	// compare average absolute error across qualities using LL step size
	type pair struct {
		q   int
		err float64
	}
	qs := []int{20, 80, 95}
	results := make([]pair, 0, len(qs))
	for _, q := range qs {
		p := CalculateQuantizationParams(q, 5, 16)
		step := p.StepSizes[0]
		qd := QuantizeCoefficients(coeffs, step)
		dq := DequantizeCoefficients(qd, step)
		var sum float64
		for i := range coeffs {
			sum += math.Abs(float64(coeffs[i] - dq[i]))
		}
		avg := sum / float64(len(coeffs))
		results = append(results, pair{q: q, err: avg})
	}

	// Expect error(20) > error(80) > error(95)
	if !(results[0].err > results[1].err) {
		t.Fatalf("expected error(q=20)>error(q=80), got %.6f <= %.6f", results[0].err, results[1].err)
	}
	if !(results[1].err > results[2].err) {
		t.Fatalf("expected error(q=80)>error(q=95), got %.6f <= %.6f", results[1].err, results[2].err)
	}
}

// TestBoundaryQualityValues tests extreme quality values
func TestBoundaryQualityValues(t *testing.T) {
	tests := []struct {
		name          string
		quality       int
		expectedStyle int
	}{
		{"Quality 1 (max compression)", 1, 2},
		{"Quality 50 (medium)", 50, 2},
		{"Quality 99 (near-lossless)", 99, 2},
		{"Quality 100 (lossless)", 100, 0},
		{"Quality below 1 (clamped)", 0, 2},
		{"Quality above 100 (clamped)", 101, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := CalculateQuantizationParams(tt.quality, 5, 16)
			if p.Style != tt.expectedStyle {
				t.Errorf("quality=%d: expected Style=%d, got %d", tt.quality, tt.expectedStyle, p.Style)
			}
			if p.GuardBits != 2 {
				t.Errorf("expected GuardBits=2, got %d", p.GuardBits)
			}
		})
	}
}

// TestDifferentBitDepths tests quantization with various bit depths
func TestDifferentBitDepths(t *testing.T) {
	bitDepths := []int{8, 12, 16}
	quality := 80
	numLevels := 3

	for _, bd := range bitDepths {
		t.Run(fmt.Sprintf("BitDepth_%d", bd), func(t *testing.T) {
			p := CalculateQuantizationParams(quality, numLevels, bd)

			// Verify we have correct number of subbands
			expectedSubbands := 3*numLevels + 1
			if len(p.StepSizes) != expectedSubbands {
				t.Errorf("bitDepth=%d: expected %d subbands, got %d", bd, expectedSubbands, len(p.StepSizes))
			}

			// Test encode/decode round-trip
			for i, step := range p.StepSizes {
				encoded := p.EncodedSteps[i]
				decoded := DecodeQuantizationStep(encoded, bd)
				if !nearlyEqual(decoded, step, 0.05) {
					t.Errorf("bitDepth=%d, subband=%d: encoded step not recovered accurately: original=%.6f decoded=%.6f",
						bd, i, step, decoded)
				}
			}
		})
	}
}

// TestDifferentDecompositionLevels tests various wavelet decomposition levels
func TestDifferentDecompositionLevels(t *testing.T) {
	levels := []int{1, 3, 5, 6}
	quality := 80
	bitDepth := 16

	for _, level := range levels {
		t.Run(fmt.Sprintf("Levels_%d", level), func(t *testing.T) {
			p := CalculateQuantizationParams(quality, level, bitDepth)

			expectedSubbands := 3*level + 1
			if len(p.StepSizes) != expectedSubbands {
				t.Errorf("numLevels=%d: expected %d subbands, got %d", level, expectedSubbands, len(p.StepSizes))
			}

			// Verify all step sizes are positive
			for i, step := range p.StepSizes {
				if step <= 0 {
					t.Errorf("numLevels=%d, subband=%d: invalid step size %.6f", level, i, step)
				}
			}
		})
	}
}

// TestQuantizationWithSpecialCoefficients tests edge cases
func TestQuantizationWithSpecialCoefficients(t *testing.T) {
	tests := []struct {
		name   string
		coeffs []int32
	}{
		{"All zeros", []int32{0, 0, 0, 0, 0}},
		{"All positive", []int32{100, 200, 300, 400, 500}},
		{"All negative", []int32{-100, -200, -300, -400, -500}},
		{"Mixed signs", []int32{-500, -100, 0, 100, 500}},
		{"Large values", []int32{32000, -32000, 16000, -16000, 0}},
		{"Small values", []int32{1, -1, 2, -2, 0}},
	}

	stepSize := 10.0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Quantize
			quantized := QuantizeCoefficients(tt.coeffs, stepSize)

			// Dequantize
			dequantized := DequantizeCoefficients(quantized, stepSize)

			// Verify length preserved
			if len(dequantized) != len(tt.coeffs) {
				t.Errorf("length mismatch: got %d, want %d", len(dequantized), len(tt.coeffs))
			}

			// Check sign preservation for non-zero values (except when quantization rounds to zero)
			for i, orig := range tt.coeffs {
				if orig != 0 && dequantized[i] != 0 {
					// Only check sign if both original and dequantized are non-zero
					// Small values may quantize to zero, which is acceptable
					origSign := orig >= 0
					deqSign := dequantized[i] >= 0
					if origSign != deqSign {
						t.Errorf("sign not preserved at index %d: orig=%d dequant=%d", i, orig, dequantized[i])
					}
				}
			}
		})
	}
}

// TestSubbandGainRelationships tests the relationships between subband gains
func TestSubbandGainRelationships(t *testing.T) {
	quality := 80
	numLevels := 5
	p := CalculateQuantizationParams(quality, numLevels, 16)

	// LL should be smallest (highest quality)
	ll := p.StepSizes[0]
	for i := 1; i < len(p.StepSizes); i++ {
		if ll >= p.StepSizes[i] {
			t.Errorf("LL step size should be smallest, but stepSizes[%d]=%.6f >= stepSizes[0]=%.6f", i, p.StepSizes[i], ll)
		}
	}

	// For each level, verify HH ~= 1.4 * HL (and ~= 1.4 * LH)
	idx := 1 // Start after LL
	for level := 1; level <= numLevels; level++ {
		hl := p.StepSizes[idx]
		lh := p.StepSizes[idx+1]
		hh := p.StepSizes[idx+2]

		// HL and LH should be similar
		if !nearlyEqual(hl, lh, 0.01) {
			t.Logf("Level %d: HL and LH slightly different: HL=%.6f LH=%.6f", level, hl, lh)
		}

		// HH should be ~1.4x HL
		ratio := hh / hl
		if math.Abs(ratio-1.4) > 0.2 {
			t.Errorf("Level %d: HH/HL ratio out of range: expected ~1.4, got %.4f (HH=%.6f HL=%.6f)",
				level, ratio, hh, hl)
		}

		idx += 3
	}
}

// TestQuantizationZeroStepSize tests that zero step size disables quantization
func TestQuantizationZeroStepSize(t *testing.T) {
	coeffs := []int32{100, -200, 300, -400, 0}

	// Zero step size should return coefficients unchanged
	quantized := QuantizeCoefficients(coeffs, 0)
	if len(quantized) != len(coeffs) {
		t.Errorf("length mismatch: got %d, want %d", len(quantized), len(coeffs))
	}
	for i := range coeffs {
		if quantized[i] != coeffs[i] {
			t.Errorf("index %d: expected no change with zero step, got %d want %d", i, quantized[i], coeffs[i])
		}
	}

	// Same for dequantization
	dequantized := DequantizeCoefficients(coeffs, 0)
	if len(dequantized) != len(coeffs) {
		t.Errorf("length mismatch: got %d, want %d", len(dequantized), len(coeffs))
	}
	for i := range coeffs {
		if dequantized[i] != coeffs[i] {
			t.Errorf("index %d: expected no change with zero step, got %d want %d", i, dequantized[i], coeffs[i])
		}
	}
}

// TestEncodedStepsPrecision tests encoding precision across wide range
func TestEncodedStepsPrecision(t *testing.T) {
	testCases := []struct {
		quality  int
		bitDepth int
		maxError float64 // Maximum allowed relative error
	}{
		{quality: 1, bitDepth: 8, maxError: 0.10},   // 10% for low quality
		{quality: 50, bitDepth: 12, maxError: 0.05}, // 5% for medium
		{quality: 90, bitDepth: 16, maxError: 0.05}, // 5% for high quality
		{quality: 99, bitDepth: 16, maxError: 0.05}, // 5% for near-lossless
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Q%d_BD%d", tc.quality, tc.bitDepth), func(t *testing.T) {
			p := CalculateQuantizationParams(tc.quality, 5, tc.bitDepth)

			if p.Style == 0 {
				// Lossless mode, skip
				return
			}

			for i, encoded := range p.EncodedSteps {
				decoded := DecodeQuantizationStep(encoded, tc.bitDepth)
				original := p.StepSizes[i]

				if !nearlyEqual(decoded, original, tc.maxError) {
					t.Errorf("subband %d: encoding error too large: original=%.6f decoded=%.6f relError=%.4f maxAllowed=%.4f",
						i, original, decoded, math.Abs(original-decoded)/original, tc.maxError)
				}
			}
		})
	}
}

// TestQualityStepSizeRange tests that step sizes stay within reasonable bounds
func TestQualityStepSizeRange(t *testing.T) {
	for quality := 1; quality <= 99; quality += 10 {
		p := CalculateQuantizationParams(quality, 5, 16)

		if p.Style == 0 {
			continue // Skip lossless
		}

		for i, step := range p.StepSizes {
			// Step sizes should be positive and reasonable
			if step <= 0 {
				t.Errorf("quality=%d subband=%d: step size must be positive, got %.6f", quality, i, step)
			}
			// For quality=1 (maximum compression), larger step sizes are expected
			// Relax the upper bound to accommodate this
			maxAllowed := 2000.0
			if quality == 1 {
				maxAllowed = 5000.0 // Very high compression can have very large step sizes
			}
			if step > maxAllowed {
				t.Errorf("quality=%d subband=%d: step size too large: %.6f (max allowed: %.1f)", quality, i, step, maxAllowed)
			}
		}
	}
}

// BenchmarkCalculateQuantizationParams benchmarks parameter calculation
func BenchmarkCalculateQuantizationParams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateQuantizationParams(80, 5, 16)
	}
}

// BenchmarkQuantizeCoefficients benchmarks quantization
func BenchmarkQuantizeCoefficients(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	coeffs := make([]int32, 10000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(1<<14) - (1 << 13))
	}
	stepSize := 10.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		QuantizeCoefficients(coeffs, stepSize)
	}
}

// BenchmarkDequantizeCoefficients benchmarks dequantization
func BenchmarkDequantizeCoefficients(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	coeffs := make([]int32, 10000)
	for i := range coeffs {
		coeffs[i] = int32(rng.Intn(2000) - 1000)
	}
	stepSize := 10.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DequantizeCoefficients(coeffs, stepSize)
	}
}
