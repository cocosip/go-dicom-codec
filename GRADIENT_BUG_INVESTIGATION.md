# Gradient Pattern Bug Investigation

**Date:** 2025-01-14
**Status:** 🔍 Under Investigation
**Current Test Pass Rate:** 94.7% (36/38 original tests)

---

## Summary

Testing revealed that specific square sizes with gradient pattern `i%256-128` fail encode/decode round-trip tests.

### Failing Sizes (Newly Discovered)
| Size | Error Rate | Status |
|------|------------|--------|
| 5×5  | 48.0% | ❌ Known |
| 10×10 | 94.0% | ❌ Newly found |
| 17×17 | 99.3% | ❌ Known |
| 18×18 | 100.0% | ❌ Newly found |
| 20×20 | 99.5% | ❌ Newly found |
| 32×32 | 97.3% | ❌ Known |

### Passing Sizes
2×2, 3×3, 4×4, 6×6, 7×7, 8×8, 9×9, 11×11, 12×12, 13×13, 14×14, 15×15, 16×16, 19×19

---

## Key Findings

### 1. NOT Related to Partial Row Groups
- 5×1: Pass ✅
- 1×5: Pass ✅
- 5×4: Pass ✅ (4 complete rows, width 5)
- 5×5: Fail ❌ (square)

**Conclusion:** Problem is specific to square dimensions, not partial RL groups.

### 2. NOT Related to Value Range
- 4×4 with values [-128 to -113]: Pass ✅
- 5×5 with values [-128 to -104]: Fail ❌

**Conclusion:** Same value ranges work in 4×4 but fail in 5×5. Problem is size-dependent, not value-dependent.

### 3. NOT Related to Prime Numbers
- 3 (prime): Pass ✅
- 5 (prime): Fail ❌
- 7 (prime): Pass ✅
- 11 (prime): Pass ✅
- 13 (prime): Pass ✅
- 17 (prime): Fail ❌

**Conclusion:** Some primes pass, some fail. Not purely a prime number issue.

### 4. NOT Simply Multiple of 5
- 5: Fail ❌
- 10: Fail ❌
- 15: Pass ✅
- 20: Fail ❌

**Conclusion:** Not all multiples of 5 fail (15 passes).

---

## Error Pattern Analysis

### 5×5 Error Locations
```
Row 0: positions 1, 3 have errors
Row 1: positions 0, 1, 3, 4 have errors
Row 2: positions 1, 3 have errors
Row 3: ALL CORRECT ✅
Row 4: positions 1, 2, 3, 4 have errors (most errors)
```

**Observation:**
- Row 3 (last row of first RL group) is perfect
- Errors mostly in odd columns (x=1, x=3)
- Row 4 (partial group) has most errors

### Error Values
Typical errors are ±1 or ±2, suggesting bit-level encoding/decoding issues:
- `[1,0] input=-127 decoded=-126` (off by 1)
- `[3,0] input=-125 decoded=-124` (off by 1)
- etc.

---

## What We've Ruled Out

1. ❌ VISIT flag lifecycle - Fixed in previous session, issue persists
2. ❌ Partial row groups - 5×4 passes, 5×5 fails
3. ❌ Value ranges - Same values work in 4×4
4. ❌ Prime numbers - Some pass, some fail
5. ❌ Simple multiple of 5 - 15×15 passes

---

## Remaining Hypotheses

### Hypothesis 1: Column Processing Order Issue
The RL encoding processes in column-first order (vertical groups of 4). Perhaps there's an issue with how columns are processed for specific widths?

- Odd columns (x=1, x=3) have more errors
- Width 5 means columns 0,1,2,3,4
- Some pattern in column processing?

### Hypothesis 2: Flag Propagation Across Boundaries
When width is 5, flag updates from one column might incorrectly affect adjacent columns in ways that don't happen with other widths.

### Hypothesis 3: Context Calculation Edge Case
The zero-coding context uses neighbor flags. Perhaps for specific widths, the context calculation has edge cases?

### Hypothesis 4: MQ Codec State Issue
For specific combinations of width and encoded bit patterns, the MQ codec might accumulate state issues?

---

## Test Files Created

- `test_5x5_trace_test.go` - Detailed 5×5 tracing
- `test_value_range_test.go` - Tests if value range matters
- `test_partial_group_test.go` - Tests partial vs complete row groups
- `test_square_sizes_test.go` - Tests all square sizes 2-20

---

## Next Steps

1. **Compare working size (4×4) vs failing size (5×5) bit-by-bit**
   - Add logging to trace every MQ encode/decode call
   - Compare encoder and decoder flag states
   - Find exact point where they diverge

2. **Test non-square rectangles**
   - 5×6, 5×7, 5×8, etc. to isolate width vs height
   - 6×5, 7×5, 8×5, etc.

3. **Simplify test case**
   - Create minimal 5×5 with just a few different values
   - Reduce to smallest case that still fails

4. **Compare with OpenJPEG**
   - Encode same data with OpenJPEG
   - Compare bit streams
   - Check if OpenJPEG handles same pattern correctly

---

## Code Changes This Session

### Removed Additional VISIT Clearing
Found and removed two more incorrect VISIT flag clears that were missed:
- `encoder.go:403` - In RL path after processing coefficient
- `encoder.go:473` - In Normal path after processing coefficient

These were clearing VISIT within the same bitplane, which is incorrect.

---

**Status:** Investigation ongoing. Pattern identified but root cause not yet found.

**Last Updated:** 2025-01-14
