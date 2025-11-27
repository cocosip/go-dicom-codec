package jpeg2000

import (
	"math"
	"testing"
)

func TestCalculateQuantizationParams(t *testing.T) {
	tests := []struct {
		name      string
		quality   int
		numLevels int
		bitDepth  int
		wantStyle int
	}{
		{
			name:      "Quality 100 (lossless)",
			quality:   100,
			numLevels: 5,
			bitDepth:  8,
			wantStyle: 0, // No quantization
		},
		{
			name:      "Quality 80 (high quality)",
			quality:   80,
			numLevels: 5,
			bitDepth:  8,
			wantStyle: 2, // Scalar expounded
		},
		{
			name:      "Quality 50 (medium)",
			quality:   50,
			numLevels: 5,
			bitDepth:  8,
			wantStyle: 2,
		},
		{
			name:      "Quality 1 (max compression)",
			quality:   1,
			numLevels: 5,
			bitDepth:  8,
			wantStyle: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := CalculateQuantizationParams(tt.quality, tt.numLevels, tt.bitDepth)

			if params.Style != tt.wantStyle {
				t.Errorf("Style = %d, want %d", params.Style, tt.wantStyle)
			}

			if params.GuardBits < 0 || params.GuardBits > 7 {
				t.Errorf("GuardBits = %d, want 0-7", params.GuardBits)
			}

			if tt.wantStyle == 2 {
				// Check we have correct number of subbands
				expectedSubbands := 3*tt.numLevels + 1
				if len(params.StepSizes) != expectedSubbands {
					t.Errorf("StepSizes length = %d, want %d", len(params.StepSizes), expectedSubbands)
				}
				if len(params.EncodedSteps) != expectedSubbands {
					t.Errorf("EncodedSteps length = %d, want %d", len(params.EncodedSteps), expectedSubbands)
				}

				// Check all step sizes are positive
				for i, step := range params.StepSizes {
					if step <= 0 {
						t.Errorf("StepSizes[%d] = %f, want > 0", i, step)
					}
				}

				// Check LL has smallest step size (least quantization)
				llStep := params.StepSizes[0]
				for i := 1; i < len(params.StepSizes); i++ {
					if params.StepSizes[i] < llStep {
						t.Logf("Warning: StepSizes[%d] = %f < LL step = %f", i, params.StepSizes[i], llStep)
					}
				}

				// Log step sizes for inspection
				t.Logf("Quality %d step sizes:", tt.quality)
				t.Logf("  LL: %.4f", params.StepSizes[0])
				idx := 1
				for level := 1; level <= tt.numLevels; level++ {
					t.Logf("  Level %d: HL=%.4f, LH=%.4f, HH=%.4f",
						level, params.StepSizes[idx], params.StepSizes[idx+1], params.StepSizes[idx+2])
					idx += 3
				}
			}
		})
	}
}

func TestQualityToStepSizeMapping(t *testing.T) {
	// Test that higher quality → smaller base step size
	params100 := CalculateQuantizationParams(100, 5, 8)
	params80 := CalculateQuantizationParams(80, 5, 8)
	params50 := CalculateQuantizationParams(50, 5, 8)
	params1 := CalculateQuantizationParams(1, 5, 8)

	// Quality 100 should have no quantization
	if params100.Style != 0 {
		t.Errorf("Quality 100 should have no quantization (style 0), got %d", params100.Style)
	}

	// For lossy qualities, check step size increases as quality decreases
	// Use HL1 subband as reference (index 1)
	if params80.StepSizes[1] >= params50.StepSizes[1] {
		t.Errorf("Quality 80 step size (%f) should be < Quality 50 (%f)",
			params80.StepSizes[1], params50.StepSizes[1])
	}

	if params50.StepSizes[1] >= params1.StepSizes[1] {
		t.Errorf("Quality 50 step size (%f) should be < Quality 1 (%f)",
			params50.StepSizes[1], params1.StepSizes[1])
	}

	t.Logf("Step size progression (HL1 subband):")
	t.Logf("  Quality 80: %.4f", params80.StepSizes[1])
	t.Logf("  Quality 50: %.4f", params50.StepSizes[1])
	t.Logf("  Quality  1: %.4f", params1.StepSizes[1])
}

func TestDecodeQuantizationStep(t *testing.T) {
	// Test encoding/decoding round trip
	params := CalculateQuantizationParams(50, 3, 8)

	for i, originalStep := range params.StepSizes {
		encoded := params.EncodedSteps[i]
		decoded := DecodeQuantizationStep(encoded, 8)

		// Allow small error due to quantization of mantissa
		relativeError := math.Abs(decoded-originalStep) / originalStep
		if relativeError > 0.01 { // 1% tolerance
			t.Errorf("Step %d: encoded/decoded mismatch: original=%.6f, decoded=%.6f, error=%.2f%%",
				i, originalStep, decoded, relativeError*100)
		}
	}
}

func TestQuantizeCoefficients(t *testing.T) {
	coefficients := []int32{100, 200, 300, 400, 500}
	stepSize := 10.0

	quantized := QuantizeCoefficients(coefficients, stepSize)

	expected := []int32{10, 20, 30, 40, 50}
	for i, q := range quantized {
		if q != expected[i] {
			t.Errorf("Quantized[%d] = %d, want %d", i, q, expected[i])
		}
	}
}

func TestDequantizeCoefficients(t *testing.T) {
	quantized := []int32{10, 20, 30, 40, 50}
	stepSize := 10.0

	dequantized := DequantizeCoefficients(quantized, stepSize)

	expected := []int32{100, 200, 300, 400, 500}
	for i, d := range dequantized {
		if d != expected[i] {
			t.Errorf("Dequantized[%d] = %d, want %d", i, d, expected[i])
		}
	}
}

func TestQuantizeDequantizeRoundTrip(t *testing.T) {
	original := []int32{127, 255, 511, 1023, 2047}
	stepSize := 8.0

	// Quantize
	quantized := QuantizeCoefficients(original, stepSize)
	t.Logf("Original:   %v", original)
	t.Logf("Quantized:  %v", quantized)

	// Dequantize
	dequantized := DequantizeCoefficients(quantized, stepSize)
	t.Logf("Dequantized: %v", dequantized)

	// Check error is within step size range
	for i := range original {
		error := math.Abs(float64(dequantized[i] - original[i]))
		if error > stepSize {
			t.Errorf("Index %d: error %f exceeds step size %f", i, error, stepSize)
		}
	}
}

func TestNoQuantization(t *testing.T) {
	coefficients := []int32{100, 200, 300}

	// Test with stepSize = 0 (no quantization)
	quantized := QuantizeCoefficients(coefficients, 0)
	if len(quantized) != len(coefficients) {
		t.Fatalf("Length mismatch")
	}
	for i := range coefficients {
		if quantized[i] != coefficients[i] {
			t.Errorf("No quantization failed: got %d, want %d", quantized[i], coefficients[i])
		}
	}

	// Test dequantization with stepSize = 0
	dequantized := DequantizeCoefficients(coefficients, 0)
	for i := range coefficients {
		if dequantized[i] != coefficients[i] {
			t.Errorf("No dequantization failed: got %d, want %d", dequantized[i], coefficients[i])
		}
	}
}
