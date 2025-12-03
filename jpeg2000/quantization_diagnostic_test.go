package jpeg2000

import (
	"math"
	"testing"
)

// TestDiagnosticHighQuality analyzes quantization behavior at high quality levels
func TestDiagnosticHighQuality(t *testing.T) {
	t.Log("=== Diagnostic Test: High Quality Quantization Parameters ===\n")

	qualities := []int{80, 85, 90, 92, 94, 95, 96, 97, 98, 99}
	numLevels := 5
	bitDepth := 16

	t.Logf("Configuration: numLevels=%d, bitDepth=%d\n", numLevels, bitDepth)
	t.Logf("%-8s | %-12s | %-12s | %-12s | %-12s | %-12s\n",
		"Quality", "BaseStep", "LL Step", "HL1 Step", "HH1 Step", "HL1/LL Ratio")
	t.Log("---------|--------------|--------------|--------------|--------------|---------------")

	var prevLLStep float64
	for _, quality := range qualities {
		p := CalculateQuantizationParams(quality, numLevels, bitDepth)

		if p.Style == 0 {
			t.Logf("%-8d | LOSSLESS MODE (no quantization)\n", quality)
			continue
		}

		// Calculate base step for this quality using the formula from quantization.go
		var baseStep float64
		if quality >= 95 && quality < 100 {
			// Linear interpolation for near-lossless range
			baseStep = 0.01 + (100.0-float64(quality))/100.0
		} else {
			baseStep = math.Pow(2.0, (100.0-float64(quality))/12.5)
			if baseStep < 0.01 {
				baseStep = 0.01
			}
			baseStep *= 0.9
		}

		llStep := p.StepSizes[0]
		hl1Step := p.StepSizes[1]
		hh1Step := p.StepSizes[3]
		ratio := hl1Step / llStep

		t.Logf("%-8d | %12.8f | %12.8f | %12.8f | %12.8f | %12.4f\n",
			quality, baseStep, llStep, hl1Step, hh1Step, ratio)

		// Check for anomalies
		if prevLLStep > 0 && llStep > prevLLStep {
			t.Errorf("  ⚠️  ANOMALY: LL step increased from %.8f to %.8f (should decrease)", prevLLStep, llStep)
		}

		prevLLStep = llStep
	}

	t.Log("\n=== Analysis of Formula Change at Quality 95 ===\n")

	// Analyze the transition point
	for q := 93; q <= 97; q++ {
		var baseStep float64
		if q >= 95 && q < 100 {
			baseStep = 0.01 + (100.0-float64(q))/100.0
			t.Logf("Q=%d: Using LINEAR formula: 0.01 + (100-%d)/100 = %.8f\n", q, q, baseStep)
		} else {
			baseStep = math.Pow(2.0, (100.0-float64(q))/12.5)
			if baseStep < 0.01 {
				baseStep = 0.01
			}
			baseStep *= 0.9
			t.Logf("Q=%d: Using EXPONENTIAL formula: 2^((100-%d)/12.5) * 0.9 = %.8f\n", q, q, baseStep)
		}
	}
}

// TestDiagnosticEncodingPipeline tests the full encoding pipeline at high quality
func TestDiagnosticEncodingPipeline(t *testing.T) {
	t.Log("=== Diagnostic Test: Full Encoding Pipeline at High Quality ===\n")

	// Create a simple 4x4 test pattern
	width := 4
	height := 4
	data := []int32{
		0, 50, 100, 150,
		50, 100, 150, 200,
		100, 150, 200, 250,
		150, 200, 250, 255,
	}

	qualities := []int{80, 90, 95, 99}
	bitDepth := 16

	t.Logf("Test data: %d x %d gradient pattern\n", width, height)
	t.Logf("Values: min=%d, max=%d\n\n", 0, 255)

	for _, quality := range qualities {
		t.Logf("--- Quality %d ---\n", quality)

		p := CalculateQuantizationParams(quality, 2, bitDepth)

		if p.Style == 0 {
			t.Logf("  Mode: LOSSLESS (no quantization)\n\n")
			continue
		}

		t.Logf("  Base step size for LL: %.8f\n", p.StepSizes[0])

		// Quantize using LL step size
		quantized := QuantizeCoefficients(data, p.StepSizes[0])
		dequantized := DequantizeCoefficients(quantized, p.StepSizes[0])

		// Calculate error statistics
		var maxError int32
		var totalError int64
		var errorCount int
		for i := range data {
			err := data[i] - dequantized[i]
			if err < 0 {
				err = -err
			}
			if err > 0 {
				errorCount++
			}
			if err > maxError {
				maxError = err
			}
			totalError += int64(err)
		}

		avgError := float64(totalError) / float64(len(data))
		psnr := calculatePSNRInt32(data, dequantized, 255.0)

		t.Logf("  Max error: %d\n", maxError)
		t.Logf("  Avg error: %.4f\n", avgError)
		t.Logf("  Error pixels: %d / %d (%.1f%%)\n", errorCount, len(data), float64(errorCount)*100/float64(len(data)))
		t.Logf("  PSNR: %.2f dB\n", psnr)

		// Show sample values
		t.Logf("  Sample original:     [%d, %d, %d, %d]\n", data[0], data[1], data[2], data[3])
		t.Logf("  Sample quantized:    [%d, %d, %d, %d]\n", quantized[0], quantized[1], quantized[2], quantized[3])
		t.Logf("  Sample dequantized:  [%d, %d, %d, %d]\n\n", dequantized[0], dequantized[1], dequantized[2], dequantized[3])
	}
}

// Helper function to calculate PSNR for int32 slices
func calculatePSNRInt32(original, decoded []int32, maxValue float64) float64 {
	if len(original) != len(decoded) {
		return 0
	}

	var mse float64
	for i := range original {
		diff := float64(original[i] - decoded[i])
		mse += diff * diff
	}
	mse /= float64(len(original))

	if mse == 0 {
		return math.Inf(1)
	}

	return 10 * math.Log10((maxValue * maxValue) / mse)
}

// TestDiagnosticFormulaComparison compares the two formula branches
func TestDiagnosticFormulaComparison(t *testing.T) {
	t.Log("=== Diagnostic Test: Formula Comparison ===\n")

	t.Logf("%-8s | %-20s | %-20s | %-12s\n", "Quality", "Exponential (old)", "Linear (new)", "Ratio")
	t.Log("---------|----------------------|----------------------|---------------")

	for q := 90; q <= 99; q++ {
		// Exponential formula (used for q < 95)
		expFormula := math.Pow(2.0, (100.0-float64(q))/12.5) * 0.9

		// Linear formula (used for q >= 95)
		linFormula := 0.01 + (100.0-float64(q))/100.0

		// Which one is actually used?
		var formulaUsed string
		if q >= 95 && q < 100 {
			formulaUsed = "LINEAR"
		} else {
			formulaUsed = "EXPONENTIAL"
		}

		ratio := linFormula / expFormula

		t.Logf("%-8d | %20.10f | %20.10f | %12.4f  [%s]\n",
			q, expFormula, linFormula, ratio, formulaUsed)
	}

	t.Log("\n=== Issue Analysis ===")
	t.Log("At Q=95, there is a formula switch from exponential to linear.")
	t.Log("This can cause a discontinuity in the baseStep values.\n")

	// Calculate the discontinuity
	q94exp := math.Pow(2.0, (100.0-94.0)/12.5) * 0.9
	q95lin := 0.01 + (100.0-95.0)/100.0

	t.Logf("Q=94 (exp): %.10f\n", q94exp)
	t.Logf("Q=95 (lin): %.10f\n", q95lin)
	t.Logf("Jump: %.10f (%.1f%% increase)\n", q95lin-q94exp, (q95lin/q94exp-1)*100)

	if q95lin > q94exp {
		t.Errorf("\n⚠️  PROBLEM IDENTIFIED: baseStep INCREASES from Q=94 to Q=95!")
		t.Errorf("   This violates the principle that higher quality should have smaller steps.")
		t.Errorf("   This explains why PSNR decreases at Q=95 and Q=99.")
	}
}

// TestDiagnosticProposedFix tests a potential fix
func TestDiagnosticProposedFix(t *testing.T) {
	t.Log("=== Diagnostic Test: Proposed Fix ===\n")

	t.Log("Current implementation has a discontinuity at Q=95.")
	t.Log("Proposed fix: Use consistent exponential formula throughout.\n")

	t.Logf("%-8s | %-20s | %-20s | %-12s\n", "Quality", "Current", "Proposed Fix", "Improvement")
	t.Log("---------|----------------------|----------------------|---------------")

	for q := 90; q <= 99; q++ {
		// Current formula
		var currentFormula float64
		if q >= 95 && q < 100 {
			currentFormula = 0.01 + (100.0-float64(q))/100.0
		} else {
			currentFormula = math.Pow(2.0, (100.0-float64(q))/12.5) * 0.9
		}

		// Proposed fix: always use exponential
		proposedFormula := math.Pow(2.0, (100.0-float64(q))/12.5)
		if proposedFormula < 0.01 {
			proposedFormula = 0.01
		}
		proposedFormula *= 0.9

		improvement := (currentFormula - proposedFormula) / currentFormula * 100

		marker := ""
		if q >= 95 {
			marker = " ⬅ CHANGED"
		}

		t.Logf("%-8d | %20.10f | %20.10f | %11.2f%%%s\n",
			q, currentFormula, proposedFormula, improvement, marker)
	}

	t.Log("\n=== Summary ===")
	t.Log("The linear formula at Q>=95 creates larger step sizes than the exponential formula.")
	t.Log("Recommendation: Remove the special case for Q>=95 and use exponential throughout.")
}
