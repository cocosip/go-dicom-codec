package jpeg2000

import (
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
