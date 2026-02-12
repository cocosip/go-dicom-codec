package lossless

import (
	"fmt"

	runmode "github.com/cocosip/go-dicom-codec/jpegls/runmode"
)

// J array (JPEG-LS A.2.1)
var J = runmode.J

// RunModeContext matches CharLS context_run_mode
type RunModeContext struct {
	runInterruptionType int
	A                   int
	N                   int
	NN                  int
}

// NewRunModeContext creates a run-mode context with initial variables derived from RANGE.
func NewRunModeContext(runInterruptionType int, rangeVal int) *RunModeContext {
	aInit := max(2, (rangeVal+32)/64)
	return &RunModeContext{
		runInterruptionType: runInterruptionType,
		A:                   aInit,
		N:                   1,
		NN:                  0,
	}
}

// GetGolombCode computes k for run interruption (CharLS get_golomb_code)
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

// UpdateVariables updates run context after encoding/decoding (A.23)
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

// ComputeMap computes map bit for run interruption (A.21)
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

// ComputeErrorValue reconstructs error value from mapped value
func (ctx *RunModeContext) ComputeErrorValue(temp, k int) int {
	mapBit := temp & 1
	errAbs := (temp + mapBit) / 2
	mapCondition := k != 0 || (2*ctx.NN >= ctx.N)
	if mapCondition == (mapBit != 0) {
		return -errAbs
	}
	return errAbs
}

// sign helper used in run interruption reconstruction
func signInt(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// RunModeScanner encapsulates run-mode helpers (encode/decode)
type RunModeScanner struct {
	RunIndex        int
	RunModeContexts [2]*RunModeContext
	traits          Traits
}

// NewRunModeScanner constructs a scanner to encode/decode run-mode segments.
func NewRunModeScanner(traits Traits) *RunModeScanner {
	rangeVal := traits.Range
	return &RunModeScanner{
		RunIndex:        0,
		RunModeContexts: [2]*RunModeContext{NewRunModeContext(0, rangeVal), NewRunModeContext(1, rangeVal)},
		traits:          traits,
	}
}

// ResetLine is kept for API compatibility but does nothing.
// Per JPEG-LS standard and CharLS implementation (scan.h:520-544),
// RunIndex should NOT be reset at the start of each line.
// It maintains state across lines and is only reset:
// 1. At scan initialization (reset_parameters)
// 2. After a restart marker
func (r *RunModeScanner) ResetLine() {
	// Intentionally empty - do not reset RunIndex here
}
func (r *RunModeScanner) incRunIndex() {
	if r.RunIndex < 31 {
		r.RunIndex++
	}
}

// DecRunIndex decrements the run index with bounds checking.
func (r *RunModeScanner) DecRunIndex() {
	if r.RunIndex > 0 {
		r.RunIndex--
	}
}

// EncodeRunLength encodes a run of given length (CharLS encode_run_pixels)
func (r *RunModeScanner) EncodeRunLength(gw *GolombWriter, runLength int, endOfLine bool) error {
	for runLength >= (1 << uint(J[r.RunIndex])) {
		if err := gw.WriteBit(1); err != nil {
			return err
		}
		runLength -= (1 << uint(J[r.RunIndex]))
		r.incRunIndex()
	}
	if endOfLine {
		if runLength != 0 {
			if err := gw.WriteBit(1); err != nil {
				return err
			}
		}
		// Note: RunIndex is NOT reset here. It should only be reset:
		// 1. At scan initialization (reset_parameters in CharLS)
		// 2. After a restart marker
		// CharLS scan.h:832-852 does not reset run_index_ at end of line
		return nil
	}
	nBits := J[r.RunIndex] + 1
	return gw.WriteBits(uint32(runLength), nBits)
}

// DecodeRunLength decodes run length (CharLS decode_run_pixels)
func (r *RunModeScanner) DecodeRunLength(gr *GolombReader, remainingInLine int) (int, error) {
	runLength := 0
	for {
		bit, err := gr.ReadBit()
		if err != nil {
			return runLength, err
		}
		if bit == 1 {
			count := min(1<<uint(J[r.RunIndex]), remainingInLine-runLength)
			runLength += count
			if count == (1 << uint(J[r.RunIndex])) {
				r.incRunIndex()
			}
			if runLength >= remainingInLine {
				return remainingInLine, nil
			}
		} else {
			break
		}
	}
	if J[r.RunIndex] > 0 {
		val, err := gr.ReadBits(J[r.RunIndex])
		if err != nil {
			return runLength, err
		}
		runLength += int(val)
	}
	if runLength > remainingInLine {
		return 0, fmt.Errorf("run length exceeds line: %d > %d", runLength, remainingInLine)
	}
	return runLength, nil
}

// EncodeRunInterruption encodes interruption error (CharLS encode_run_interruption_error)
func (r *RunModeScanner) EncodeRunInterruption(gw *GolombWriter, ctx *RunModeContext, errorValue int) error {
	k := ctx.GetGolombCode()
	mapBit := ctx.ComputeMap(errorValue, k)
	eMapped := 2*runmode.Abs(errorValue) - ctx.runInterruptionType
	if mapBit {
		eMapped--
	}
	// CharLS scan.h:775 uses limit - J[run_index_] - 1 directly without bounds checking
	limitMinusJ := r.traits.Limit - J[r.RunIndex] - 1
	if err := gw.EncodeMappedValue(k, eMapped, limitMinusJ, r.traits.Qbpp); err != nil {
		return err
	}
	ctx.UpdateVariables(errorValue, eMapped, r.traits.Reset)
	return nil
}

// DecodeRunInterruption decodes interruption error (CharLS decode_run_interruption_error)
func (r *RunModeScanner) DecodeRunInterruption(gr *GolombReader, ctx *RunModeContext) (int, error) {
	k := ctx.GetGolombCode()
	// CharLS scan.h:672 uses limit - J[run_index_] - 1 directly without bounds checking
	limitMinusJ := r.traits.Limit - J[r.RunIndex] - 1
	mapped, err := gr.DecodeValue(k, limitMinusJ, r.traits.Qbpp)
	if err != nil {
		return 0, err
	}
	errVal := ctx.ComputeErrorValue(mapped+ctx.runInterruptionType, k)
	ctx.UpdateVariables(errVal, mapped, r.traits.Reset)
	return errVal, nil
}
