# Diagonal Corruption Bug Investigation

**Date:** 2025-01-14
**Status:** 🔍 UNDER INVESTIGATION
**Priority:** P1 (blocks 100% test pass rate)

---

## Problem Description

When encoding/decoding 2x2 image blocks with **two different non-zero values** in diagonal or non-adjacent positions, zero-valued coefficients get incorrectly set to 1.

### Failing Test Pattern

**Input:** `[1, 2, 0, 0]` or `[1, 0, 0, 2]`
**Expected:** Same as input
**Actual:** Zero positions get bit 0 set incorrectly

### Test Results

| Pattern | Layout | Result | Status |
|---------|--------|--------|--------|
| `[1, 1, 0, 0]` | Same values | Pass | ✅ |
| `[2, 2, 0, 0]` | Same values | Pass | ✅ |
| `[1, 3, 0, 0]` | Adjacent, different | Pass | ✅ |
| `[3, 1, 0, 0]` | Adjacent, different | Pass | ✅ |
| `[1, 0, 1, 0]` | Same column | Pass | ✅ |
| **`[1, 2, 0, 0]`** | **Row 0, different** | **Fail** | ❌ |
| **`[1, 0, 0, 2]`** | **Diagonal, different** | **Fail** | ❌ |
| **`[2, 0, 0, 4]`** | **Diagonal, different** | **Fail** | ❌ |
| **`[3, 0, 0, 6]`** | **Diagonal, different** | **Fail** | ❌ |

### Specific Failures

**Test:** `[1, 2, 0, 0]`
- Position 0: 1 → 1 ✓
- Position 1: 2 → 3 ✗ (bit 0 incorrectly set)
- Position 2: 0 → 1 ✗ (bit 0 incorrectly set)
- Position 3: 0 → 0 ✓

**Test:** `[1, 0, 0, 2]`
- Position 0: 1 → 1 ✓
- Position 1: 0 → 0 ✓
- Position 2: 0 → 1 ✗ (bit 0 incorrectly set)
- Position 3: 2 → 3 ✗ (bit 0 incorrectly set)

**Pattern:** When different values (like 1 and 2) are present, zeros at specific positions get bit 0 set.

---

## Technical Analysis

### Why This Matters

This bug affects **all gradient pattern tests** because gradients naturally produce patterns with different coefficient values across the image:
- 5x5 gradient: 92% errors
- 8x8, 16x16, 17x17, 32x32 gradients: 97-99% errors

### Bit-Plane Encoding Theory

For `[1, 0, 0, 2]`:
- Value 1 = `0b01`: becomes significant at BP0 (bit 1 is 0, bit 0 is 1)
- Value 2 = `0b10`: becomes significant at BP1 (bit 1 is 1, bit 0 is 0)
- Value 0 = `0b00`: never becomes significant

**Expected Encoding:**
- **BP1**: pos3 becomes significant (bit1=1), others stay insignificant
- **BP0**: pos0 becomes significant (bit0=1), pos3 refines (bit0=0)

**Actual Behavior:**
- Positions 2 and 3 both get bit 0 set incorrectly → values become 1 and 3

### MQ Codec Desynchronization Hypothesis

The most likely cause is **MQ arithmetic codec bit stream desynchronization** between encoder and decoder:

1. Encoder encodes bits for all 4 positions (even though only position 0 logs are visible)
2. Decoder reads the bit stream
3. Something causes decoder to read wrong bits for positions 2 and 3
4. Result: incorrect bit values decoded

### Before vs After MRP VISIT Fix

**Before MRP VISIT Fix** (commit fe0ffcd):
- `[1, 2, 0, 0]` → `[1, 3, -1, 1]` (even worse!)

**After MRP VISIT Fix** (commit 936d155):
- `[1, 2, 0, 0]` → `[1, 3, 1, 0]` (improved but still wrong)

The MRP VISIT fix helped but didn't fully resolve the issue.

---

## Investigation Findings

### What We Know

1. **Single values work perfectly**: `[0, 0, 0, 1]`, `[0, 0, 0, 2]`, `[1, 0, 0, 0]` all pass
2. **Same values work**: `[1, 1, 0, 0]`, `[2, 2, 0, 0]` pass
3. **Adjacent different values in row 0**: Pass if 1 & 3, fail if 1 & 2
4. **Diagonal different values**: Consistently fail
5. **Logging limitation**: Only position 0 (first coefficient) logs are visible, making it hard to trace other positions

### What We Don't Know

1. How positions 1, 2, 3 are actually being encoded (no logs)
2. Whether RL (Run-Length) encoding is involved
3. Exact MQ codec call sequence for all positions
4. Why specifically values like (1, 2) fail but (1, 3) pass

---

## Potential Root Causes

### Theory 1: CP Still Processing Already-Refined Coefficients

Even with the MRP VISIT fix, there might be edge cases where CP still processes coefficients that were refined in MRP.

**Evidence:** MRP clears VISIT was the bug, now fixed, but maybe incomplete.

### Theory 2: Context Selection Error

Different contexts are used for:
- SPP: Zero-coding context based on neighbor flags
- MRP: Magnitude refinement context
- CP: Zero-coding context (same as SPP?)

If encoder and decoder use different contexts for the same coefficient, MQ codec will desynchronize.

### Theory 3: RL Encoding Path Issue

The RL encoding path handles groups of 4 vertical coefficients together. If there's a bug in how it handles mixed zero/non-zero values, it could cause this pattern.

**Evidence:** 2x2 images process in column-first order with RL groups.

### Theory 4: Neighbor Flag Propagation

When position 0 or position 1 becomes significant, it sets neighbor flags for positions 2 and 3. If these flags are set incorrectly or at the wrong time, it could cause positions 2 and 3 to be processed when they shouldn't be.

---

## Next Steps

1. **Add comprehensive logging** for all positions (not just position 0)
2. **Trace exact MQ encode/decode sequence** for failing test case
3. **Compare working case** `[1, 3, 0, 0]` vs failing case `[1, 2, 0, 0]` bit by bit
4. **Check RL encoding logic** for 2x2 blocks
5. **Verify context usage** is consistent between encoder and decoder

---

## Files Involved

- `jpeg2000/t1/encoder.go` - T1 encoder, MRP and CP passes
- `jpeg2000/t1/decoder.go` - T1 decoder, MRP and CP passes
- `jpeg2000/mqc/encoder.go` - MQ arithmetic encoder
- `jpeg2000/mqc/mqc.go` - MQ arithmetic decoder

---

## Test Files Created

- `test_simple_combo_test.go` - Matrix of combination tests
- `test_diagonal_debug_test.go` - Detailed `[1,0,0,2]` trace
- `test_layout_test.go` - Verify index/position mapping

---

**Status:** Bug partially understood but root cause not yet identified
**Impact:** Blocks 18% of T1 tests (6 out of 33 subtests)
**Workaround:** None

**Last Updated:** 2025-01-14
