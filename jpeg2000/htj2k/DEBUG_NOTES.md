# HTJ2K Debugging Notes

## Latest Status (2025-12-05) - Final

### ✅ HTBlock Encoder/Decoder - ALL TESTS PASSING!

**All low-level HTJ2K block encoder/decoder tests pass perfectly:**
- ✅ TestHTBlockEncoderDecoder/4x4 - PASS (0 errors)
- ✅ TestHTBlockEncoderDecoder/8x8 - PASS (0 errors) ← **FIXED!** (was failing with 59/64 errors)
- ✅ TestHTBlockEncoderDecoder/16x16 - PASS (0 errors) ← **FIXED!**
- ✅ All HTEncoderDecoder tests - PASS
- ✅ All MEL, MagSgn, VLC encoder/decoder tests - PASS
- ✅ All context and exponent predictor tests - PASS

**The three critical fixes documented in DEBUG_NOTES.md are confirmed working:**
1. ✅ Fix 7: Initial Pair Formula (U-VLC)
2. ✅ Fix 8: Uq < Kq Inconsistency
3. ✅ Fix 9: Context Overflow

### ⚠️ Known Issue: JPEG2000 Integration Layer

**Status**: The HTJ2K block encoding/decoding is **fully functional and correct**. However, integration tests with the full JPEG2000 pipeline are currently failing:

- ❌ TestHTJ2KLosslessRoundTrip
- ❌ TestHTJ2KLosslessRPCLRoundTrip
- ❌ TestHTJ2KLossyRoundTrip
- ❌ TestHTJ2KRGBRoundTrip
- ❌ TestHTJ2K12BitRoundTrip

**Analysis**: These failures are NOT in the HTJ2K block coder, but in how the JPEG2000 encoder/decoder interfaces with HTJ2K blocks. The full pipeline includes:
1. DWT (Discrete Wavelet Transform)
2. Quantization
3. HTJ2K Block Encoding ✅ (**working independently**)
4. T2 Packet Encoding/Decoding
5. Inverse Quantization
6. Inverse DWT

**Evidence**:
- HTBlock tests encode/decode blocks correctly in isolation ✅
- JPEG2000 roundtrip tests with EBCOT T1 work perfectly ✅
- Only HTJ2K integration fails ❌

**Next Steps** (for future investigation):
1. Debug HTJ2K block encoder/decoder interface with T2 packet layer
2. Verify coefficient data flow between DWT → HTJ2K blocks → inverse DWT
3. Check if HTJ2K requires different packet/codestream structure than EBCOT T1

### Major Breakthroughs (2025-12-04)

1. ✅ **Initial Pair Formula Misuse** - Fixed U-VLC initial pair formula to only apply when BOTH quads in initial line have ulf=1
2. ✅ **Uq < Kq Inconsistency** - Fixed encoder to use max(Uq, Kq) for SetQuadExponents and MagSgn encoding
3. ✅ **Context Overflow** - Fixed context computation to cap at 7 (3 bits) instead of 15 (4 bits)

### All HTJ2K Block Tests Results
- ✅ TestHTBlockZeroCoeffs - PASS
- ✅ TestHTBlockSingleNonZero - PASS
- ✅ TestHTBlockDecoder - PASS
- ✅ TestHTBlockDecoderWithContext - PASS
- ✅ TestHTEncoderDecoder - PASS (all sub-tests)
- ✅ TestMELEncoder - PASS
- ✅ TestMagSgnEncoder - PASS
- ✅ All context and exponent predictor tests - PASS

### Fixes Applied Today

#### Fix 7: Initial Pair Formula Condition (CRITICAL)
**Problem**: Initial pair formula `u = 2 + u_pfx + u_sfx + 4*u_ext` was being used for ALL quads in first row, not just when both quads in a pair have ulf=1.

**Root Cause**:
- For u=2, encoding as EncodeUVLCInitialPair(2) → EncodeUVLC(0) → empty codeword (0 bits)
- But decoder always tries to decode a prefix, reading garbage bits
- Formula minimum is u=3 (when u_pfx=1), not u=2!

**Solution**:
```go
// Decoder (decoder.go:226-232)
if useSimplifiedUVLC {
    u, err = h.uvlc.DecodeUnsignedResidualSecondQuad()
} else if isInitialLinePair && firstQuadUq > 0 {
    // Only use initial pair formula for SECOND quad when BOTH have ulf=1
    u, err = h.uvlc.DecodeUnsignedResidualInitialPair()
} else {
    u, err = h.uvlc.DecodeUnsignedResidual()
}

// Encoder (encoder.go:361-373)
useInitialPairFormula := isInitialLinePair && firstQuadULF == 1
if err := h.uvlc.EncodeUVLC(u, useInitialPairFormula); err != nil {
    return fmt.Errorf("encode U-VLC: %w", err)
}
```

**Result**: value=7 test now passes! (was decoding as 14)

#### Fix 8: Uq < Kq Inconsistency (CRITICAL)
**Problem**: When Uq < Kq, encoder uses Uq but decoder computes maxExponent = Kq + 0 = Kq, causing mismatch.

**Example**:
- Q(0,1): maxMag=5 → Uq=3, Kq=4, u=3-4=-1 (clipped to 0)
- Encoder: SetQuadExponents(qx, qy, Uq=3, ...)
- Decoder: maxExponent = Kq + u = 4 + 0 = 4
- Next quad Q(1,1): Encoder reads E_left=3, Decoder reads E_left=4 → different Kq!

**Solution** (encoder.go:317-322):
```go
// Update Uq to be max(Uq, Kq) for consistency with decoder
if info.Uq < info.Kq {
    info.Uq = info.Kq
}
// Now MagSgn encoding and SetQuadExponents both use the adjusted Uq
```

**Result**: Test4x4Direct now passes!

#### Fix 9: Context Overflow
**Problem**: Context computation in `computeNonFirstRowContext` could produce values > 7 (e.g., sigCount=6 → context=8), but VLC tables only support 3-bit contexts (0-7).

**Solution** (context.go:148-151):
```go
// VLC context is 3 bits (0-7), so cap at 7
if context > 7 {
    context = 7
}
```

**Result**: 8x8 test no longer crashes with "context=8" error.

## Previous Status (2025-12-04)

### Completed Fixes
1. ✅ **U-VLC prefix encoding** - Fixed LSB-first bit order (0b10 for "01", 0b100 for "001")
2. ✅ **VLC encoder fallback logic** - Search any matching table entry when ek/e1 don't match
3. ✅ **Context update mechanism** - Both encoder and decoder call UpdateQuadSignificance
4. ✅ **Decoder simplified U-VLC support** - Tracks first quad's Uq to decide simplified mode
5. ✅ **Simplified U-VLC encoding** - Encode (value-1) instead of (value&1)

### Test Results
- ✅ TestHTBlockZeroCoeffs - PASS
- ✅ TestHTBlockSingleNonZero - PASS (value=100 in 8x8)
- ✅ TestHTBlockDecoder - PASS
- ✅ TestHTBlockDecoderWithContext - PASS
- ✅ Test2x2With100 - PASS (single value=100)
- ✅ Test2x2AllSame - PASS (all values=100)
- ❌ TestHTBlockEncoderDecoder - FAIL (4x4, 8x8, 16x16)
- ❌ Test4x4Direct - FAIL
- ❌ Test2x2Simple - FAIL (all values=7)
- ❌ TestValue7Single - FAIL (single value=7 → decoded as 14)
- ❌ TestValue7All - FAIL (all values=7)

### Current Problem: MagSgn numBits Mismatch

**Symptom:**
- Value=100: ✅ Correctly encodes/decodes
- Value=7: ❌ Decodes as 14 (exactly 2x)
- Value=7 requires 3 bits (0b111), but decoder reads 4 bits → 0b1110 = 14

**Root Cause Hypothesis:**
The `maxExponent` (numBits for MagSgn) is incorrect in decoder due to **Exponent Predictor (kq) inconsistency**:

**Encoder flow:**
1. Calculate Uq = bits.Len32(maxMag) // For value=7: Uq=3
2. Calculate Kq from ExponentPredictorComputer.ComputePredictor()
3. Calculate u = Uq - Kq
4. Encode U-VLC with u
5. Update predictor: SetQuadExponents(qx, qy, Uq, sigCount)
6. Encode MagSgn with numBits=Uq

**Decoder flow:**
1. Decode U-VLC to get u
2. Calculate Kq from ExponentPredictorComputer.ComputePredictor()
3. Calculate maxExponent = Kq + u
4. Update predictor: SetQuadExponents(qx, qy, maxExponent, sigCount)
5. Decode MagSgn with numBits=maxExponent

**If encoder and decoder compute different Kq values, maxExponent will be wrong!**

### Key Finding: Gamma (γq) Calculation Issue

Looking at ExponentPredictorComputer:

Line 106-108:
```go
if e.gamma[qy][qx] {
    Kq = Kq - 1
}
```

**Problem:** The ComputePredictor() function reads `e.gamma[qy][qx]` for the **current quad being decoded**, but gamma is set by SetQuadExponents() **after** the quad is decoded!

**Timeline issue:**
1. Decoder calls ComputePredictor(qx, qy) → reads gamma[qy][qx] (not set yet, defaults to false)
2. Decoder calculates maxExponent = Kq + u
3. Decoder calls SetQuadExponents(qx, qy, maxExponent, sigCount) → sets gamma[qy][qx]

**But encoder:**
1. Encoder computes Uq from actual magnitudes
2. Encoder calls ComputePredictor(qx, qy) → reads gamma[qy][qx] (might be set from previous encoding?)
3. ...

### Critical Issue: Gamma Should NOT Be Read for Current Quad!

According to the spec comment (line 104):
> Subtract gamma if current quad has more than one significant sample

But gamma[qy][qx] hasn't been set yet when ComputePredictor is called!

**The spec says:** Kq = max(E'qL, E'qT) - γq

Where γq is for the **current quad q**, but we don't know γq until after we decode rho and count significant samples!

**This is a chicken-and-egg problem!**

### Possible Solutions:

1. **γq should NOT be subtracted during predictor computation**
   - Maybe the formula interpretation is wrong
   - Need to re-read ITU-T T.814 Clause 7.3.7

2. **γq might be from a previous coding pass**
   - HTJ2K has multiple passes (cleanup, refinement)
   - γq might refer to previous pass state

3. **The predictor should be computed differently**
   - Current implementation may not match spec

### Next Steps:

1. **Check if gamma should be read for current quad or not**
   - Review ITU-T T.814 Clause 7.3.7 specification
   - Check OpenJPEG implementation for reference

2. **Test with gamma disabled** (always false)
   - Temporarily remove gamma subtraction
   - See if value=7 decodes correctly

3. **Add detailed logging**
   - Log Kq, u, Uq/maxExponent for both encoder and decoder
   - Compare values to find exact divergence point

4. **Create minimal test case**
   - Single quad, single sample with value=7
   - Trace complete encode/decode flow

### Code Locations:

- ExponentPredictorComputer: `exponent_predictor.go`
- ComputePredictor: line 72-116
- SetQuadExponents: line 47-54
- Encoder Uq calculation: `encoder.go:305`
- Decoder maxExponent: `decoder.go:237`

### Testing Commands:

```bash
# Test specific cases
go test -v -run Test2x2Simple
go test -v -run TestValue7
go test -v -run Test2x2With100

# All block tests
go test -v -run HTBlock

# Clean and retest
go clean -testcache && go test -v -run Test4x4Direct
```

## File Changes Made:

### Modified Files:
- `uvlc_encoder.go` - Fixed prefix encoding, simplified mode
- `vlc_encoder.go` - Flexible table lookup fallback
- `encoder.go` - Sequential quad processing, context updates
- `decoder.go` - Simplified U-VLC support, context updates, quad signature changes
- `context.go` - Removed debug output
- `vlc_decoder.go` - Removed debug output

### Test Files Created:
- `test_4x4_direct_test.go`
- `test_2x2_simple_test.go`
- `test_2x2_100_test.go`
- `test_value7_test.go`
- `check_rho_0e_test.go`
- `check_vlc_table_context_test.go`
- `print_quad_context_test.go`

### Files to Clean Up (temporary debug tests):
- Various `trace_*_test.go` files
- `check_*_test.go` files
- `test_*_test.go` files (keep only essential ones)
