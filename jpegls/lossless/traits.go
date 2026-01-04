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

// ComputeReconstructedSample matches CharLS fix_reconstructed_value.
func (t Traits) ComputeReconstructedSample(prediction, errorValue int) int {
	reconstructed := prediction + errorValue
	if reconstructed < -t.Near {
		reconstructed += t.Range * (2*t.Near + 1)
	} else if reconstructed > t.MaxVal+t.Near {
		reconstructed -= t.Range * (2*t.Near + 1)
	}
	if reconstructed < 0 {
		reconstructed = 0
	} else if reconstructed > t.MaxVal {
		reconstructed = t.MaxVal
	}
	return reconstructed
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
