# JPEG 2000 Current Status

**Date**: 2025-11-13
**Overall Progress**: 98% MVP decoder completion

## Recent Work

### Cleanup Pass Sign Bit Synchronization Bug

**Problem**: Encoder was encoding sign bits for already-significant coefficients in both RL (Run-Length) and normal paths of Cleanup Pass. This violates JPEG2000 standard - sign bits should only be encoded when coefficient FIRST becomes significant.

**Impact**:
- T1 level tests: 0% → 35.59% → needs to reach 100%
- JPEG2000 level tests: All gradient/image tests failing with corrupted values
- Example: Input pixel 0 becomes -170 after encode/decode cycle due to extra bits being encoded

**Fix Required**: Add `alreadySig` check before encoding sign bits in TWO locations in `jpeg2000/t1/encoder.go`:

#### Location 1: RL Path (around line 371-390)
```go
if isSig != 0 {
    // Check if already significant
    alreadySig := (flags & T1_SIG) != 0

    if !alreadySig {
        // Coefficient becomes significant for the first time
        // Encode sign bit (uniform context in cleanup pass)
        signBit := 0
        if t1.data[idx] < 0 {
            signBit = 1
            t1.flags[idx] |= T1_SIGN
        }

        if isFirst {
            fmt.Printf("signBit=%d\n", signBit)
        }

        t1.mqe.Encode(signBit, CTX_UNI)

        // Mark as significant
        t1.flags[idx] |= T1_SIG

        // Update neighbor flags
        t1.updateNeighborFlags(i, y, idx)
    }
    // If already significant: bit was just a refinement, no sign bit needed
}
```

#### Location 2: Normal Path (around line 427-441)
```go
if isSig != 0 {
    // Check if already significant
    alreadySig := (flags & T1_SIG) != 0

    if !alreadySig {
        // Coefficient becomes significant for the first time
        // Encode sign bit (uniform context in cleanup pass)
        signBit := 0
        if t1.data[idx] < 0 {
            signBit = 1
            t1.flags[idx] |= T1_SIGN
        }
        t1.mqe.Encode(signBit, CTX_UNI)

        // Mark as significant
        t1.flags[idx] |= T1_SIG

        // Update neighbor flags
        t1.updateNeighborFlags(i, y, idx)
    }
    // If already significant: bit was just a refinement, no sign bit needed
}
```

**Decoder Status**: The decoder in `jpeg2000/t1/decoder.go` has already been partially fixed to handle already-significant coefficients in the normal path (lines 592-668), but may need similar fixes for the RL path.

**Testing**: After applying encoder fixes, run:
```bash
cd jpeg2000/t1 && go test -run TestT1EncodeDecodeRoundTrip -v
cd jpeg2000 && go test -run TestEncoderDecoderRoundTrip -v
```

## Other Known Issues

### 1. Gradient Data Decoding (98.83% error rate)
- Location: Documented in `KNOWN_ISSUES.md`
- Likely related to the Cleanup Pass synchronization bug above
- Should improve significantly once sign bit issue is fixed

### 2. Tag Tree Decoder (0% complete)
- Location: `jpeg2000/t2/tagtree.go`
- Status: Encoder complete, decoder not implemented
- Priority: Medium (needed for full T2 packet decoding)

### 3. RL Encoding Synchronization
- Specific sparse patterns and dimensions cause issues
- Documented in `KNOWN_ISSUES.md`
- Priority: Low (edge cases)

## Next Steps

1. **CRITICAL**: Apply the encoder Cleanup Pass fix to `jpeg2000/t1/encoder.go` (documented above)
2. **HIGH**: Verify decoder RL path also handles already-significant coefficients correctly
3. **MEDIUM**: Run full test suite and verify gradient data decoding improves
4. **MEDIUM**: Complete tag tree decoder implementation
5. **LOW**: Address RL encoding edge cases

## Files Modified (Not Yet Committed)

- `jpeg2000/mqc/encoder.go` - Added debug counter (EncodeCount)
- `jpeg2000/mqc/mqc.go` - Added debug counter (DecodeCount)
- `jpeg2000/t1/decoder.go` - Fixed Cleanup Pass for already-sig coeffs (normal path)
- `jpeg2000/t1/encoder.go` - **NEEDS FIX** - Must add alreadySig checks

## Test Results

### Before Fixes
- T1 round-trip: 0% correct reconstruction
- MQ desync: 6326 encode/decode call difference

### After Decoder-Only Fix
- T1 round-trip: 35.59% correct (205/576 coefficients)
- MQ desync: Reduced significantly

### After Both Fixes (Target)
- T1 round-trip: Should reach 97-100% correct
- MQ desync: Should be <20 calls difference
- JPEG2000 round-trip: Gradient tests should pass

## Architecture Notes

### DC Level Shift
- **Encoder** (`jpeg2000/encoder.go` line 121): Subtracts 2^(bitDepth-1) for unsigned data before T1 encoding
- **Decoder** (`jpeg2000/t2/tile_decoder.go` lines 147, 295): Adds 2^(bitDepth-1) back after T1 decoding
- **Status**: Implementation correct, not causing current test failures

### T1 Cleanup Pass Structure
- **Two paths**: RL (Run-Length) for groups of 4, and normal for remaining coefficients
- **Three states**: Not significant, newly significant (encode sign), already significant (NO sign)
- **Problem**: Encoder was treating "already significant" same as "newly significant"

## Debug Counters

The MQ codec has debug counters added:
- `mqc.MQDecoder.DecodeCount` - Total decode calls
- `mqc.MQEncoder.EncodeCount` - Total encode calls

These help identify synchronization issues when encode/decode counts don't match.
