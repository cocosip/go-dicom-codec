package jpeg2000

import (
	"math"
)

// QuantizationParams holds quantization parameters for all subbands
type QuantizationParams struct {
	// Quantization style
	// 0 = no quantization (lossless)
	// 1 = scalar derived (single base step size)
	// 2 = scalar expounded (explicit step size for each subband)
	Style int

	// Guard bits (0-7)
	GuardBits int

	// Step sizes for each subband
	// For scalar expounded: one entry per subband
	// Index order: LL, HL1, LH1, HH1, HL2, LH2, HH2, ..., HLn, LHn, HHn
	StepSizes []float64

	// Encoded step sizes (exponent + mantissa for each subband)
	// Format: bits 0-10 = mantissa (11 bits), bits 11-15 = exponent (5 bits)
	EncodedSteps []uint16
}

// CalculateQuantizationParams calculates quantization parameters based on quality
// quality: 1-100 (1 = maximum compression, 100 = minimal quantization/near-lossless)
// numLevels: number of wavelet decomposition levels
// bitDepth: original bit depth of the image
func CalculateQuantizationParams(quality, numLevels, bitDepth int) *QuantizationParams {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	// For lossless (quality >= 100), return no quantization
	if quality >= 100 {
		return &QuantizationParams{
			Style:     0, // No quantization
			GuardBits: 2,
		}
	}

	// Calculate number of subbands: LL + 3 * numLevels (HL, LH, HH per level)
	numSubbands := 3*numLevels + 1

	params := &QuantizationParams{
		Style:        2, // Scalar expounded (explicit step size per subband)
		GuardBits:    2, // Standard guard bits
		StepSizes:    make([]float64, numSubbands),
		EncodedSteps: make([]uint16, numSubbands),
	}

	// Convert quality (1-100) to base quantization step size
	// Use a more conservative formula for better quality at high values:
	//
	// Quality 100 → baseStep = 0 (no quantization, handled above)
	// Quality 95  → baseStep ≈ 0.1 (minimal quantization)
	// Quality 80  → baseStep ≈ 0.5 (small quantization)
	// Quality 50  → baseStep ≈ 4.0 (moderate quantization)
	// Quality 1   → baseStep ≈ 128.0 (heavy quantization)
	//
	// Use formula: baseStep = 2^((100 - quality) / 15) - 1
	// This gives a gentler curve for high quality values
	baseStep := math.Pow(2.0, (100.0-float64(quality))/15.0) - 1.0
	if baseStep < 0.01 {
		baseStep = 0.01 // Minimum step size for lossy mode
	}

	// Calculate subband gains for 9/7 irreversible wavelet
	// Use conservative gains to preserve quality
	//
	// For 9/7 wavelet, use moderate gain scaling:
	// - LL: minimal step size
	// - HL, LH, HH: gradually increasing step size based on level
	//
	// We use a more conservative formula: gain = 1 + level * 0.5
	// This prevents excessive quantization in higher frequency subbands

	idx := 0

	// LL subband (lowest frequency, highest energy)
	// Use the base step size directly for LL
	params.StepSizes[idx] = baseStep
	idx++

	// For each decomposition level (from finest to coarsest)
	for level := 1; level <= numLevels; level++ {
		// Moderate gain for high-frequency subbands
		// Use formula: gain = 1 + level * 0.3 (much more conservative)
		gain := 1.0 + float64(level)*0.3

		// HL subband (horizontal detail)
		params.StepSizes[idx] = baseStep * gain
		idx++

		// LH subband (vertical detail)
		params.StepSizes[idx] = baseStep * gain
		idx++

		// HH subband (diagonal detail)
		// Slightly higher gain for diagonal
		params.StepSizes[idx] = baseStep * gain * 1.2
		idx++
	}

	// Encode step sizes to JPEG 2000 format
	// Format: 16-bit value with 5-bit exponent and 11-bit mantissa
	// stepSize = 2^(exponent - bias) * (1 + mantissa / 2048)
	// where bias = bitDepth - 1
	bias := bitDepth - 1

	for i, stepSize := range params.StepSizes {
		// Calculate exponent and mantissa
		// stepSize = 2^exp * mantissa
		exp := int(math.Floor(math.Log2(stepSize))) + bias
		mantissa := stepSize / math.Pow(2.0, float64(exp-bias))

		// Clamp exponent to 5 bits (0-31)
		if exp < 0 {
			exp = 0
		}
		if exp > 31 {
			exp = 31
		}

		// Convert mantissa to 11-bit integer (0-2047)
		// mantissa range is [1, 2), we encode (mantissa - 1) * 2048
		mantissaInt := int((mantissa - 1.0) * 2048.0)
		if mantissaInt < 0 {
			mantissaInt = 0
		}
		if mantissaInt > 2047 {
			mantissaInt = 2047
		}

		// Combine: bits 0-10 = mantissa, bits 11-15 = exponent
		params.EncodedSteps[i] = uint16((exp << 11) | mantissaInt)
	}

	return params
}

// DecodeQuantizationStep decodes a JPEG 2000 quantization step from 16-bit encoded format
// encoded: 16-bit value with bits 11-15 = exponent, bits 0-10 = mantissa
// bitDepth: original bit depth of the image
func DecodeQuantizationStep(encoded uint16, bitDepth int) float64 {
	exponent := int((encoded >> 11) & 0x1F)
	mantissa := float64(encoded & 0x7FF)

	bias := bitDepth - 1

	// stepSize = 2^(exponent - bias) * (1 + mantissa / 2048)
	stepSize := math.Pow(2.0, float64(exponent-bias)) * (1.0 + mantissa/2048.0)

	return stepSize
}

// QuantizeCoefficients applies quantization to wavelet coefficients
// coefficients: input wavelet coefficients
// stepSize: quantization step size
// Returns: quantized coefficients
func QuantizeCoefficients(coefficients []int32, stepSize float64) []int32 {
	if stepSize <= 0 {
		// No quantization
		return coefficients
	}

	quantized := make([]int32, len(coefficients))
	for i, coeff := range coefficients {
		// Quantize: round(coeff / stepSize)
		quantized[i] = int32(math.Round(float64(coeff) / stepSize))
	}
	return quantized
}

// DequantizeCoefficients applies dequantization to coefficients
// coefficients: quantized coefficients
// stepSize: quantization step size
// Returns: dequantized coefficients
func DequantizeCoefficients(coefficients []int32, stepSize float64) []int32 {
	if stepSize <= 0 {
		// No dequantization needed
		return coefficients
	}

	dequantized := make([]int32, len(coefficients))
	for i, coeff := range coefficients {
		// Dequantize: coeff * stepSize
		dequantized[i] = int32(math.Round(float64(coeff) * stepSize))
	}
	return dequantized
}
