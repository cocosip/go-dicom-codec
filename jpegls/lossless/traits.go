package lossless

// Traits captures derived JPEG-LS parameters (CharLS defaults).
type Traits struct {
	MaxVal int
	Near   int
	Range  int
	Qbpp   int
	Limit  int
	Reset  int
	T1     int
	T2     int
	T3     int
}

// NewTraits computes derived parameters using ComputeCodingParameters (Annex C defaults).
func NewTraits(maxVal, near, reset int) Traits {
	params := ComputeCodingParameters(maxVal, near, reset)
	return Traits{
		MaxVal: maxVal,
		Near:   near,
		Range:  params.Range,
		Qbpp:   params.Qbpp,
		Limit:  params.Limit,
		Reset:  params.Reset,
		T1:     params.T1,
		T2:     params.T2,
		T3:     params.T3,
	}
}

// ComputeReconstructedSample matches CharLS compute_reconstructed_sample.
// CharLS: fix_reconstructed_value(predicted_value + dequantize(error_value))
func (t Traits) ComputeReconstructedSample(prediction, errorValue int) int {
	// First dequantize the error
	dequantized := t.dequantize(errorValue)

	// Then call fix_reconstructed_value
	return t.fixReconstructedValue(prediction + dequantized)
}

// fixReconstructedValue matches CharLS fix_reconstructed_value (default_traits.h:140-152)
func (t Traits) fixReconstructedValue(value int) int {
	// Lossless power-of-two optimization
	rangeIsPowerOfTwo := (t.MaxVal+1)&t.MaxVal == 0
	if t.Near == 0 && rangeIsPowerOfTwo {
		return value & t.MaxVal
	}

	// if (value < -near_lossless)
	if value < -t.Near {
		value = value + t.Range*(2*t.Near+1)
	} else if value > t.MaxVal+t.Near {
		// else if (value > maximum_sample_value + near_lossless)
		value = value - t.Range*(2*t.Near+1)
	}

	// return correct_prediction(value)
	return t.correctPrediction(value)
}

// dequantize matches CharLS dequantize (default_traits.h:135-138)
func (t Traits) dequantize(errorValue int) int {
	return errorValue * (2*t.Near + 1)
}

// correctPrediction matches CharLS correct_prediction (default_traits.h:83-89)
func (t Traits) correctPrediction(predicted int) int {
	// if ((predicted & maximum_sample_value) == predicted) return predicted;
	if (predicted & t.MaxVal) == predicted {
		return predicted
	}

	// return (~(predicted >> (int32_t_bit_count - 1))) & maximum_sample_value;
	// Handles negative by returning 0, and > maxVal by returning maxVal
	if predicted < 0 {
		return 0
	}
	return t.MaxVal
}

// CorrectPrediction clamps prediction to [0, MaxVal] (CharLS correct_prediction).
func (t Traits) CorrectPrediction(pred int) int {
	// CharLS uses clamping, not wrapping, even for power-of-two ranges
	if pred < 0 {
		return 0
	}
	if pred > t.MaxVal {
		return t.MaxVal
	}
	return pred
}

// ModuloRange applies modulo reduction for near-lossless error values.
func (t Traits) ModuloRange(errorValue int) int {
	if errorValue < 0 {
		errorValue += t.Range
	}
	if errorValue >= (t.Range+1)/2 {
		errorValue -= t.Range
	}
	return errorValue
}

// MapErrorValue maps signed error to non-negative (CharLS map_error_value).
func (t Traits) MapErrorValue(err int) int {
	if err >= 0 {
		return 2 * err
	}
	return -2*err - 1
}

// UnmapErrorValue reverses MapErrorValue.
func (t Traits) UnmapErrorValue(val int) int {
	if (val & 1) == 0 {
		return val >> 1
	}
	return -((val + 1) >> 1)
}

// QuantizeGradient quantizes gradient differences using T1/T2/T3 and NEAR.
func (t Traits) QuantizeGradient(d int) int {
	if d <= -t.T3 {
		return -4
	}
	if d <= -t.T2 {
		return -3
	}
	if d <= -t.T1 {
		return -2
	}
	if d < -t.Near {
		return -1
	}
	if d <= t.Near {
		return 0
	}
	if d < t.T1 {
		return 1
	}
	if d < t.T2 {
		return 2
	}
	if d < t.T3 {
		return 3
	}
	return 4
}

// IsNear checks if two values are within NEAR distance.
// Matches CharLS is_near (default_traits.h:66-69)
func (t Traits) IsNear(lhs, rhs int) bool {
	diff := lhs - rhs
	if diff < 0 {
		diff = -diff
	}
	return diff <= t.Near
}

// ComputeErrorValue matches CharLS compute_error_value (default_traits.h:55-58)
// Returns: modulo_range(quantize(e))
func (t Traits) ComputeErrorValue(e int) int {
	// quantize
	quantized := t.quantize(e)
	// modulo_range
	return t.ModuloRange(quantized)
}

// quantize matches CharLS quantize (default_traits.h:127-133)
func (t Traits) quantize(errorValue int) int {
	if t.Near == 0 {
		return errorValue
	}
	if errorValue > 0 {
		return (errorValue + t.Near) / (2*t.Near + 1)
	}
	return -(t.Near - errorValue) / (2*t.Near + 1)
}
