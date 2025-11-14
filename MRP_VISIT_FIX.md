# MRP VISIT Flag Fix

**Date:** 2025-01-14
**Status:** ✅ PARTIAL FIX - Improved test pass rate

---

## Problem Identified

The Magnitude Refinement Pass (MRP) was **clearing** the VISIT flag instead of **setting** it, causing the Cleanup Pass (CP) to process already-refined coefficients a second time.

### Root Cause

For already-significant coefficients:
1. **SPP (Significance Propagation Pass)**: Skips (already significant)
2. **MRP (Magnitude Refinement Pass)**: Encodes refinement bit, then **clears** VISIT flag
3. **CP (Cleanup Pass)**: Processes coefficient again (VISIT==0), encoding another "refinement" bit

This caused **double encoding** of refinement information.

---

## The Fix

### Encoder (jpeg2000/t1/encoder.go)

**Before** (line 267-271):
```go
// Mark as refined
t1.flags[idx] |= T1_REFINE

// Clear visit flag (ready for next bit-plane)
t1.flags[idx] &^= T1_VISIT
```

**After** (line 267-268):
```go
// Mark as refined and visited (so CP won't refine again)
t1.flags[idx] |= T1_REFINE | T1_VISIT
```

### Decoder (jpeg2000/t1/decoder.go)

**Before** (line 360-364):
```go
// Mark as refined
t1.flags[idx] |= T1_REFINE

// Clear visit flag (ready for next bit-plane)
//t1.flags[idx] &^= T1_VISIT  // Was commented out
```

**After** (line 360-361):
```go
// Mark as refined and visited (so CP won't refine again)
t1.flags[idx] |= T1_REFINE | T1_VISIT
```

---

## Impact

### Before Fix
- **Test Results**: 27 passing / 7 failing subtests
- **RL Gradient Tests**: 100% errors (5x5, 17x17, 32x32 gradients)
- **Issue**: CP was encoding/decoding refinement bits for coefficients already refined in MRP

### After Fix
- **Test Results**: 27 passing / 6 failing subtests (**+1 improvement**)
- **RL Gradient Tests**: Still failing but different error pattern (92% errors for 5x5)
- **Improvement**: Eliminated double refinement encoding

### Tests Still Passing
- ✅ All height=3 tests (2x3, 3x3 uniform, etc.)
- ✅ Uniform patterns (3x3, 16x16)
- ✅ Single coefficient tests
- ✅ BP7-only tests
- ✅ Most core T1 encoding tests

### Tests Still Failing
- ❌ Gradient patterns (5x5, 8x8, 16x16, 17x17, 32x32)
- ❌ TestDetailedTwoValues ([1,0,0,2] → [1,0,1,3] - off by 1 in positions 2,3)
- ❌ TestDiagonalPattern
- ❌ TestSimpler
- ❌ Some TestT1EncodeDecodeRoundTrip subtests

---

## Analysis

The MRP VISIT fix resolves one bug (double refinement) but reveals that there are additional issues:

1. **Gradient patterns still fail**: The error rate improved from 100% to 92% for 5x5, suggesting progress but not complete fix

2. **Off-by-1 errors**: TestDetailedTwoValues shows systematic off-by-1 errors in specific positions, suggesting bit-level encoding/decoding mismatch

3. **Pattern-specific failures**: Uniform values work perfectly, but gradients fail - this suggests the issue is in how **varying** coefficients are encoded

---

## Next Investigation Needed

The remaining failures appear to be related to:

1. **CP Normal path bit encoding**: May still have issues with how it encodes bits for various coefficient values

2. **RL (Run-Length) group handling**: The failures are concentrated in gradient patterns where RL encoding optimizations apply

3. **Context selection**: Different contexts between encoder and decoder for the same coefficient might cause MQ codec desynchronization

---

## Files Modified

1. **jpeg2000/t1/encoder.go** (line 268)
   - Changed MRP to set VISIT flag instead of clearing it

2. **jpeg2000/t1/decoder.go** (line 361)
   - Changed MRP to set VISIT flag instead of clearing it (was commented out)

---

**Status:** MRP VISIT fix complete, but gradient pattern failures remain
**Next Step:** Investigate why gradient patterns still fail with 92% error rate
**Priority:** P1 (blocks 100% test pass rate)

**Last Updated:** 2025-01-14
