# T1 Context Alignment Report

## Executive Summary

The T1 (EBCOT) context calculation implementation in go-dicom-codec has **significant differences** from OpenJPEG's reference implementation. While the overall structure is correct (19 contexts, three coding passes), the lookup table indexing schemes and some calculation formulas are different.

**Test Results**: 3/6 context tests FAILED

## Detailed Findings

### 1. Sign Context Lookup Table (lut_ctxno_sc) - ❌ FAILED

**Issue**: Different LUT indexing schemes

#### OpenJPEG Indexing (8-bit index)
```
Bit layout for lut_ctxno_sc[i]:
- Bit 0 (T1_LUT_SGN_W): West neighbor sign
- Bit 1 (T1_LUT_SIG_N): North neighbor significance
- Bit 2 (T1_LUT_SGN_E): East neighbor sign
- Bit 3 (T1_LUT_SIG_W): West neighbor significance
- Bit 4 (T1_LUT_SGN_N): North neighbor sign
- Bit 5 (T1_LUT_SIG_E): East neighbor significance
- Bit 6 (T1_LUT_SGN_S): South neighbor sign
- Bit 7 (T1_LUT_SIG_S): South neighbor significance
```

**Key Point**: Interleaved significance and sign bits

#### go-dicom-codec Indexing (8-bit index)
```
Bit layout for lut_ctxno_sc[i]:
- Bit 0: East positive
- Bit 1: East negative
- Bit 2: West positive
- Bit 3: West negative
- Bit 4: North positive
- Bit 5: North negative
- Bit 6: South positive
- Bit 7: South negative
```

**Key Point**: Paired positive/negative bits for each direction

#### Comparison Example
For i=0 (no neighbor signs):

| Implementation | Index 0 Meaning | LUT Value | Final Context |
|---------------|-----------------|-----------|---------------|
| OpenJPEG | No sign bits, no sig bits | 0x9 (9) | 9 |
| go-dicom-codec | No positive/negative neighbors | 2 | 2 + 9 = 11 |

**Discrepancy**: Different context values for same neighbor configuration

#### Test Failures
```
lut_ctxno_sc[0]: expected 0x9, got 0xb (11)
lut_ctxno_sc[1]: expected 0x9, got 0xa (10)
lut_ctxno_sc[2]: expected 0xa, got 0xc (12)
... (many more mismatches)
```

#### Root Cause
The LUT initialization in `context.go:initSignContextLUT()` uses a different mapping formula than OpenJPEG. The (h, v) → context mapping is:

**go-dicom-codec mapping**:
```go
if h < 0 {
    if v < 0 { ctx = 4 }      // Context 13
    else if v == 0 { ctx = 3 } // Context 12
    else { ctx = 2 }           // Context 11
} else if h == 0 {
    if v < 0 { ctx = 3 }       // Context 12
    else if v == 0 { ctx = 2 } // Context 11  <-- h=0, v=0 maps to 11
    else { ctx = 1 }           // Context 10
} else {
    if v < 0 { ctx = 2 }       // Context 11
    else if v == 0 { ctx = 1 } // Context 10
    else { ctx = 0 }           // Context 9
}
```

**OpenJPEG mapping**: Uses pre-computed table with different formula (not explicitly visible in code)

#### Fix Required
1. **Option A (Recommended)**: Rewrite `getSignCodingContext()` and `initSignContextLUT()` to use OpenJPEG's indexing scheme and directly copy OpenJPEG's table values
2. **Option B**: Keep current indexing but reverse-engineer OpenJPEG's mapping formula and update initialization logic

### 2. Sign Bit Extraction Logic - ❌ PARTIAL FAILURE

**Issue**: Incorrect interpretation of T1_SIGN_* flags

#### Current Implementation (context.go:232-264)
```go
idx := 0
if flags&T1_SIGN_E != 0 {
    if flags&T1_SIG_E != 0 {
        idx |= 1 // East positive
    } else {
        idx |= 2 // East negative
    }
}
```

**Problem**: This checks `flags&T1_SIGN_E != 0` first, which is backwards. The correct interpretation should be:
- If neighbor is NOT significant (`T1_SIG_E == 0`): Skip (no contribution)
- If neighbor IS significant (`T1_SIG_E != 0`):
  - If `T1_SIGN_E == 0`: Positive sign
  - If `T1_SIGN_E != 0`: Negative sign

#### Correct Logic
```go
if flags&T1_SIG_E != 0 {  // Check significance first
    if flags&T1_SIGN_E != 0 {
        idx |= 2 // East negative
    } else {
        idx |= 1 // East positive
    }
}
```

#### Test Failure Example
```
Test: TestSignCodingContextExtraction/东负西负_h=-2
Expected context: 12
Got context: 11
```

This suggests the sign extraction is not correctly identifying negative neighbors.

### 3. Zero Coding Context Calculation - ❌ FAILED

**Issue**: Formula differences between implementations

#### OpenJPEG Implementation
Uses pre-computed table `lut_ctxno_zc[2048]`:
- 2048 entries = 4 orientations × 512 neighbor configurations
- Oriented by subband type (HL, LH, HH, LL)
- Direct lookup: `return mqc->lut_ctxno_zc_orient[(f & T1_SIGMA_NEIGHBOURS)];`
- Index is 9-bit neighbor significance mask (bits 0-8)

#### go-dicom-codec Implementation
Dynamic calculation (context.go:269-330):
```go
h := 0  // Horizontal neighbors (E, W)
v := 0  // Vertical neighbors (N, S)
d := 0  // Diagonal neighbors (NE, NW, SE, SW)

// Count each type
sum := h + h + v + v + d  // h, v weighted 2x, d weighted 1x

// Map sum to context 0-8
if sum >= 9 { return 8 }
if sum >= 7 { return 7 }
...
```

#### Test Failures
```
Test: 仅左邻居 (only left neighbor)
  Actual: h=1, v=0, d=0 → sum=2 → context 3
  Expected: context 2

Test: 仅上邻居 (only up neighbor)
  Actual: h=0, v=1, d=0 → sum=2 → context 3
  Expected: context 1

Test: 左右邻居 (left+right neighbors)
  Actual: h=2, v=0, d=0 → sum=4 → context 5
  Expected: context 3
```

#### Root Cause
The weighting formula `sum := h + h + v + v + d` doesn't match OpenJPEG's context mapping. OpenJPEG's table has different context values for horizontal vs. vertical neighbors.

**Example Analysis** (from OpenJPEG lut_ctxno_zc):
```
lut_ctxno_zc[0] = 0    // No neighbors
lut_ctxno_zc[1] = 1    // NW neighbor only (bit 0)
lut_ctxno_zc[2] = 3    // N neighbor only (bit 1)
lut_ctxno_zc[4] = ?    // NE neighbor only (bit 2)
lut_ctxno_zc[8] = 1    // W neighbor only (bit 3)
```

The table shows different context values for different neighbor positions, not just based on count.

#### Fix Required
1. **Option A (Recommended)**: Import OpenJPEG's lut_ctxno_zc table and use table lookup
2. **Option B**: Reverse-engineer the exact formula from OpenJPEG's table and implement dynamic calculation

### 4. Magnitude Refinement Context - ✅ PASSED

**Status**: All tests passed

The magnitude refinement context calculation is correct and matches OpenJPEG:
- Counts all 8 neighbors (orthogonal + diagonal)
- Maps to contexts 14-16 based on count
- Formula: sum >= 3 → 16, sum >= 1 → 15, else → 14

### 5. Sign Prediction LUT (lut_spb) - ⚠️ NOT FULLY TESTED

**Status**: Basic structure correct, but depends on sign context indexing fix

The sign prediction LUT uses the same indexing scheme as sign context LUT, so it will need to be updated if sign context indexing is changed.

Current implementation (context.go:188-228):
```go
if h+v < 0 {
    lut_spb[i] = 1 // Predict negative
} else {
    lut_spb[i] = 0 // Predict positive
}
```

This logic is simple and likely correct, but the indexing must match OpenJPEG.

### 6. Context Constants - ✅ PASSED

**Status**: All constant definitions correct

The 19 context ranges are correctly defined:
- Zero Coding: 0-8 ✓
- Sign Coding: 9-13 ✓
- Magnitude Refinement: 14-16 ✓
- Run-Length: 17 ✓
- Uniform: 18 ✓

## OpenJPEG Flag Encoding Scheme

OpenJPEG uses a sophisticated bit-packed flag system:

```c
// 32-bit flag word packs 4 stripe rows:
// Bits 0-8: SIGMA (significance) for 9 positions
//   T1_SIGMA_0-8: NW, N, NE, W, THIS, E, SW, S, SE
// Bits 18-31: CHI (sign) bits
//   T1_CHI_0-5: Sign bits for 6 positions

// Stripe-based processing (4 rows at a time)
flags >> (ci * 3U)  // Shift to current coefficient's stripe position
```

go-dicom-codec uses simpler per-coefficient flags:
```go
// Separate flag bits for each direction
T1_SIG_N, T1_SIG_S, T1_SIG_W, T1_SIG_E,     // Significance
T1_SIG_NW, T1_SIG_NE, T1_SIG_SW, T1_SIG_SE, // Diagonal
T1_SIGN_N, T1_SIGN_S, T1_SIGN_W, T1_SIGN_E  // Signs
```

This is a valid simplification for a different implementation approach.

## Impact Assessment

### Critical Issues
1. **Sign Context LUT**: Produces different context values → Different MQ encoder states → **Incompatible bitstream**
2. **Zero Coding Context**: Produces different context values → **Incompatible bitstream**

### Compatibility Impact
- **Decoding OpenJPEG streams**: Will likely FAIL due to context mismatches
- **Encoding for OpenJPEG**: Will likely FAIL to decode correctly
- **Round-trip (go-dicom-codec only)**: May still WORK if internal consistency is maintained

### Performance Impact
- Current dynamic calculation may be slower than table lookup
- Table lookup requires ~2KB of data (lut_ctxno_zc: 2048 bytes)

## Recommendations

### Priority 1: Fix Sign Context (HIGH PRIORITY)

**Action Items**:
1. Read OpenJPEG's lut_ctxno_sc table from t1_luts.h
2. Update `initSignContextLUT()` to use OpenJPEG's indexing:
   ```go
   // Bit layout:
   // 0: T1_LUT_SGN_W, 1: T1_LUT_SIG_N
   // 2: T1_LUT_SGN_E, 3: T1_LUT_SIG_W
   // 4: T1_LUT_SGN_N, 5: T1_LUT_SIG_E
   // 6: T1_LUT_SGN_S, 7: T1_LUT_SIG_S
   ```
3. Update `getSignCodingContext()` to build index using OpenJPEG's bit layout
4. Copy OpenJPEG's 256-entry table values directly

**Estimated Effort**: 2-3 hours

### Priority 2: Fix Zero Coding Context (HIGH PRIORITY)

**Action Items**:
1. Import OpenJPEG's lut_ctxno_zc[2048] table
2. Determine subband orientation strategy:
   - **Option A**: Always use orientation 0 (simpler, may lose some compression efficiency)
   - **Option B**: Implement full orientation support (HL, LH, HH)
3. Update `getZeroCodingContext()` to use table lookup
4. Add orientation parameter to context computer if using Option B

**Estimated Effort**: 3-4 hours

### Priority 3: Fix Sign Bit Extraction (MEDIUM PRIORITY)

**Action Items**:
1. Correct the logic in `getSignCodingContext()` (lines 232-264)
2. Check significance first, then check sign
3. Update similar logic in `getSignPrediction()` (lines 371-404)

**Estimated Effort**: 1 hour

### Priority 4: Comprehensive Testing (MEDIUM PRIORITY)

**Action Items**:
1. Create test cases with known OpenJPEG input/output pairs
2. Test each context type independently
3. Verify round-trip encoding/decoding
4. Performance benchmarking

**Estimated Effort**: 2-3 hours

## Test Summary

| Test | Status | Issue |
|------|--------|-------|
| TestSignContextLUTAlignment | ❌ FAIL | Indexing scheme mismatch |
| TestSignPredictionLUTAlignment | ⚠️ SKIP | Depends on sign context fix |
| TestZeroCodingContextLogic | ❌ FAIL | Formula/table mismatch |
| TestMagnitudeRefinementContext | ✅ PASS | Correct implementation |
| TestSignCodingContextExtraction | ❌ FAIL | Sign bit extraction logic |
| TestContextConstants | ✅ PASS | Correct definitions |
| TestContextModeling | ✅ PASS | General structure correct |
| TestContextTables | ✅ PASS | Table initialization works |

**Overall**: 3/8 major tests FAILED, alignment is **incomplete**

## Conclusion

The T1 context implementation in go-dicom-codec has the correct **structure** (19 contexts, three passes, proper separation of concerns) but uses different **calculation methods** and **lookup table indexing** compared to OpenJPEG.

**For 100% bit-level compatibility with OpenJPEG**, the following changes are REQUIRED:
1. Adopt OpenJPEG's sign context LUT indexing and table values
2. Import OpenJPEG's zero-coding context table
3. Fix sign bit extraction logic

**Estimated Total Effort**: 8-10 hours of focused development + testing

---

*Report Generated*: 2026-01-09
*Tested Against*: OpenJPEG t1_luts.h and t1.c
*go-dicom-codec Version*: Current development branch
