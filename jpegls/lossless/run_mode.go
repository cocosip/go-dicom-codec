package lossless

// J array for run mode encoding
// Used to determine how large runs should be encoded at a time
// Defined by JPEG-LS standard, A.2.1, Initialization step 3
var J = [32]int{
	0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3,
	4, 4, 5, 5, 6, 6, 7, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

// RunModeContext holds the statistical model for run mode interruption
// This matches CharLS context_run_mode
type RunModeContext struct {
	runInterruptionType int // 0 or 1, depending on the interruption condition
	A                   int // Running sum
	N                   int // Count
	NN                  int // Negative error count
}

// NewRunModeContext creates a new run mode context
func NewRunModeContext(runInterruptionType int, range_ int) *RunModeContext {
	// ISO/IEC 14495-1, A.8, step 1.d and A.2.1
	aInit := max(2, (range_+32)/64)

	return &RunModeContext{
		runInterruptionType: runInterruptionType,
		A:                   aInit,
		N:                   1,
		NN:                  0,
	}
}

// GetGolombCode computes the Golomb parameter for run interruption
// This matches CharLS get_golomb_code
func (ctx *RunModeContext) GetGolombCode() int {
	temp := ctx.A + (ctx.N>>1)*ctx.runInterruptionType
	nTest := ctx.N
	k := 0
	for nTest < temp {
		nTest <<= 1
		k++
		if k > 32 {
			break
		}
	}
	return k
}

// UpdateVariables updates run mode context after encoding/decoding
// Code segment A.23 – Update of variables for run interruption sample
func (ctx *RunModeContext) UpdateVariables(errorValue, eMappedErrorValue int, resetThreshold int) {
	if errorValue < 0 {
		ctx.NN++
	}

	ctx.A += (eMappedErrorValue + 1 - ctx.runInterruptionType) >> 1

	if ctx.N == resetThreshold {
		ctx.A >>= 1
		ctx.N >>= 1
		ctx.NN >>= 1
	}

	ctx.N++
}

// ComputeMap computes the map for error value mapping in run mode
// Code segment A.21 – Computation of map for Errval mapping
func (ctx *RunModeContext) ComputeMap(errorValue, k int) bool {
	if k == 0 && errorValue > 0 && 2*ctx.NN < ctx.N {
		return true
	}

	if errorValue < 0 && 2*ctx.NN >= ctx.N {
		return true
	}

	if errorValue < 0 && k != 0 {
		return true
	}

	return false
}

// ComputeErrorValue reconstructs error value from mapped value (for decoder)
func (ctx *RunModeContext) ComputeErrorValue(temp, k int) int {
	mapBit := temp & 1
	errorValueAbs := (temp + mapBit) / 2

	mapCondition := (k != 0 || (2*ctx.NN >= ctx.N))
	if mapCondition == (mapBit != 0) {
		return -errorValueAbs
	}

	return errorValueAbs
}

// ComputeMapNegativeE is an optimized version for negative errors
func (ctx *RunModeContext) ComputeMapNegativeE(k int) bool {
	return k != 0 || 2*ctx.NN >= ctx.N
}

// sign returns 1 if n >= 0, -1 if n < 0
// This matches CharLS sign() function
func sign(n int) int {
	if n >= 0 {
		return 1
	}
	return -1
}
