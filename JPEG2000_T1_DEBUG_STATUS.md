# JPEG2000 T1 Codec Debug Status

**Date**: 2025-01-24 (Updated)
**Issue**: 5x5 grid decoding errors (12/25 coefficients incorrect)

---

## Problem Summary

The JPEG2000 T1 (EBCOT Tier-1) encoder/decoder has a bug that manifests specifically when encoding/decoding grids with **both 5+ rows AND 5+ columns**. The bug results in approximately 48% of coefficients being decoded incorrectly.

### Test Results

| Grid Size | Result | Error Count |
|-----------|--------|-------------|
| 1x1       | ✅ PASS | 0/1 |
| 2x1       | ✅ PASS | 0/2 |
| 5x1       | ✅ PASS | 0/5 |
| 1x5       | ✅ PASS | 0/5 |
| 2x2       | ✅ PASS | 0/4 |
| 4x4       | ✅ PASS | 0/16 |
| 4x5       | ✅ PASS | 0/20 |
| 5x4       | ✅ PASS | 0/20 |
| **5x5**   | ❌ **FAIL** | **12/25 (48%)** |

### Error Pattern

Errors are concentrated in:
- **Odd columns** (x=1, 3) across multiple rows
- **Row 4** (the partial row group) - all positions except x=0
- Error magnitudes: typically ±1 or ±2 in coefficient values

Example errors from 5x5 test:
```
Position | Expected | Got  | Diff
---------|----------|------|-----
(1,0)    | -127     | -126 | +1
(3,0)    | -125     | -124 | +1
(0,1)    | -123     | -122 | +1
(1,1)    | -122     | -123 | -1
(3,1)    | -120     | -121 | -1
(4,1)    | -119     | -118 | +1
(1,2)    | -117     | -116 | +1
(3,2)    | -115     | -114 | +1
(1,4)    | -107     | -105 | +2
(2,4)    | -106     | -104 | +2
(3,4)    | -105     | -104 | +1
(4,4)    | -104     | -105 | -1
```

---

## FINAL BREAKTHROUGH - Deep MQ State Analysis

### Critical Discovery: MQC Operation #191

Through extensive MQ encoder/decoder state logging, the exact point of failure has been identified:

**MQ Encoder #191 BEFORE**:
- Encoding: bit=1 (LPS, since mps=0)
- Context: 16
- State: 14, MPS: 0
- A: 0xa402
- C: 0x0005ff32
- ct: 7

**MQ Decoder #191 BEFORE**:
- Context: 16 ✓ (matches)
- State: 14, MPS: 0 ✓ (matches)
- A: 0xa402 ✓ (matches)
- C: 0x53ee0000 ❌ (WRONG!)
- ct: 3 (different, but this is expected)

**MQ Decoder #191 Judgment**:
- Qe (state 14) = 0x1601
- Condition: `(C >> 16) < qe` → `0x53ee < 0x1601`?
- Result: FALSE (0x53ee = 21486 > 0x1601 = 5633)
- Therefore: Decoder takes MPS path
- Decoded bit: 0 (WRONG! Should be 1)

### Root Cause Analysis

The decoder's C register has an **incorrect value** at operation #191. The C register should contain a value where `(C >> 16) < 0x1601`, but instead it has 0x53ee0000, causing the wrong decoding path.

### C Register Divergence Tracking

Traced back through all operations to find where C register first diverges:

**At MQC #1 (First Operation)**:
- **Encoder BEFORE**: C=0x00000000, ct=12 ✓ (correct initial state)
- **Decoder BEFORE**: C=0x5cde8000, ct=1 ✓ (correct after initialization)

The encoder and decoder C registers are **supposed to be different** - this is by design:
- Encoder: Starts with empty C, accumulates code during encoding
- Decoder: Loads initial code from bitstream during initialization

**However**, the decoder's C register evolution through operations should lead to correct LPS/MPS decisions. The fact that it doesn't suggests a systematic error in how the C register is updated or how bytes are read from the bitstream.

### Initialization Verification

**Decoder Initialization** (verified correct):
1. Read firstByte=0xb9 → C=0x00b90000
2. Call bytein() → Read secondByte=0xbd → C=0x00b9bd00, ct=8
3. Shift left by 7 → C=0x5cde8000, ct=1

This matches the standard MQ decoder initialization.

### Byte I/O Analysis

**Encoded Bitstream** (24 bytes):
```
b9 bd 11 6a 57 fa c5 d7 d6 20 25 c3 8d c1 b3 f0 ae 3e 46 65 ff 19 00 00
```

**I/O Operation Counts**:
- Encoder: 94 byteout calls, 23 actual byte writes (+ 1 in Flush = 24 bytes)
- Decoder: 97 bytein calls, 24 actual byte reads (+ 2 sentinel bytes)

**0xFF Byte Stuffing**:
- Position 20: 0xff followed by 0x19
  - 0x19 < 0x8F → Should consume both bytes ✓
  - Decoder correctly handles: C += 0x3200, ct=7

### MQ Roundtrip Test: PASSES

Isolated test of MQ encoder/decoder (without T1):
- Encoded 25 bits successfully
- Decoded all 25 bits correctly
- **Conclusion**: MQ codec itself is correct

### Key Insight

The problem is **not in the MQ codec implementation** but in how the T1 encoder/decoder interacts with it, specifically:
1. Context state management
2. Timing of when MQ operations occur
3. Potentially in flag state synchronization between encoder and decoder

---

## Investigation History

### What We've Confirmed ✅

1. **MQ Codec Implementation is Correct**
   - Standalone MQ roundtrip test passes perfectly
   - bytein/byteout handle 0xFF stuffing correctly
   - renormalization logic is consistent

2. **Bitplanes BP=7 through BP=2 Decode Correctly**
   - Coefficient values progress correctly
   - refBit sequences match between encoder and decoder
   - No divergence in these bitplanes

3. **BP=1 MRP is Where Visible Errors Begin**
   - Row 4 positions show wrong refBit values
   - But the actual C register divergence started earlier

4. **Context States Match at Critical Point**
   - At MQC #191, both encoder and decoder have:
     - Same context (16)
     - Same state (14)
     - Same MPS (0)
   - Yet decoder makes wrong LPS/MPS decision due to wrong C value

### What We've Ruled Out ❌

1. ❌ MQ arithmetic codec implementation bug
2. ❌ 0xFF byte stuffing handling error
3. ❌ Initialization sequence error
4. ❌ renormalization order issue (tested both orders)
5. ❌ Simple off-by-one error in byte reading

---

## Current Hypothesis

The most likely cause is **context state desynchronization** between encoder and decoder at the T1 level:

### Hypothesis A: Context State History Mismatch

The MQ encoder and decoder each maintain context states (state + MPS for each context). If at any point during encoding/decoding, the encoder and decoder update context states differently, subsequent operations will use different states, leading to:
- Different Qe values
- Different state transitions
- Eventually, different C register values

**Evidence**:
- MQ codec itself works perfectly in isolation
- Error manifests only in complex T1 encoding with 5x5 grids
- Row 4 (partial group) is particularly affected
- Context 16 is used for MRP operations

**Potential Root Causes**:
1. A coefficient is processed by encoder but skipped by decoder (or vice versa)
2. Flag states (T1_SIG, T1_VISIT, T1_REFINE) differ at some point
3. Context reset behavior differs
4. Early termination or pass skipping logic differs

### Hypothesis B: Subtle T1 Pass Logic Difference

In one of the passes (likely SPP or CP at BP=6 or earlier), the encoder and decoder:
- Process coefficients in slightly different order
- Skip/include different sets of coefficients
- Update flags differently

This causes them to call MQ encode/decode with different context state histories, eventually leading to the C register mismatch.

---

## Next Steps

### Immediate Actions

1. **Add Context State Logging to T1 Encoder/Decoder**
   ```
   Action: Log context state (state, MPS) before each MQ encode/decode call
   Goal: Find the first point where encoder and decoder use different states
   ```

2. **Compare Flag States After Each Pass**
   ```
   Action: Dump all coefficient flags (SIG, VISIT, REFINE) after BP=7, BP=6, BP=5
   Goal: Find where flag synchronization breaks
   ```

3. **Verify Coefficient Processing Order**
   ```
   Action: Log every coefficient processed in SPP/MRP/CP at BP=6 and BP=5
   Goal: Ensure encoder and decoder process exactly the same coefficients
   ```

4. **Review OpenJPEG Reference Implementation**
   ```
   Action: Compare against opj_t1_enc_sigpass, opj_t1_enc_refpass, opj_t1_enc_clnpass
   Focus: Context usage, flag updates, skip logic
   ```

### Debug Tools Available

1. **MQ Operation Counters** - Track operations by pass/bitplane
2. **Detailed MQ State Logging** - A, C, ct, state, mps for each operation
3. **T1 Pass Logging** - Coefficient processing in SPP/MRP/CP
4. **Flag State Dumps** - SIG, VISIT, REFINE for all coefficients

---

## Code Locations to Investigate

### Priority 1: Context Usage in T1

| File | Function | Lines | Focus |
|------|----------|-------|-------|
| `t1/encoder.go` | `encodeSigPropPass()` | 221-291 | Context selection, flag updates |
| `t1/decoder.go` | `decodeSigPropPass()` | 244-347 | Match encoder logic exactly |
| `t1/encoder.go` | `encodeMagRefPass()` | 293-347 | Context 16 usage for MRP |
| `t1/decoder.go` | `decodeMagRefPass()` | 355-430 | Context 16 usage for MRP |

### Priority 2: Flag State Management

| File | Function | Lines | Focus |
|------|----------|-------|-------|
| `t1/encoder.go` | `updateNeighborFlags()` | - | Verify identical to decoder |
| `t1/decoder.go` | `updateNeighborFlags()` | - | Verify identical to encoder |
| `t1/encoder.go` | VISIT flag clearing | 126 | Timing of flag clear |
| `t1/decoder.go` | VISIT flag clearing | 84 | Timing of flag clear |

### Priority 3: Context Functions

| File | Function | Purpose |
|------|----------|---------|
| `t1/context.go` | `getZeroCodingContext()` | SPP context selection |
| `t1/context.go` | `getMagRefinementContext()` | MRP context (returns 16) |
| `t1/context.go` | `getSignCodingContext()` | Sign bit context |

---

## Summary

After extensive debugging with detailed MQ state logging, the problem has been **precisely located**:

**Location**: MQ Decoder operation #191 (BP=1 MRP, position 1,4)

**Symptom**: Decoder's C register = 0x53ee0000, causing wrong LPS/MPS decision

**Root Cause**: Not in MQ codec (verified by passing roundtrip test), but in **T1 encoder/decoder context state synchronization**

**Next Critical Step**: Add context state logging to every MQ encode/decode call in T1 layer to find where context states first diverge between encoder and decoder.

The bug is subtle and systematic - it's not a simple coding error but a deeper synchronization issue in how the T1 layer manages and passes context states to the MQ codec.

---

## Files Modified (For Reference)

### Source Files with Debug Code
- `jpeg2000/t1/encoder.go` - MQ operation counters, detailed pass logging
- `jpeg2000/t1/decoder.go` - MQ operation counters, detailed pass logging, coefficient value dumps
- `jpeg2000/mqc/encoder.go` - Detailed MQ state logging (A, C, ct, state, mps)
- `jpeg2000/mqc/mqc.go` - Detailed MQ state logging, bytein/byteout tracking

### Test Files Created
- `test_simple_5x5_debug_test.go` - Main debug test with full MQ logging
- `test_bp1_only_test.go` - Test encoding only to BP=1 (skip BP=0)
- `test_bitstream_hex_test.go` - Examine bitstream bytes
- `test_mq_ops_counter_test.go` - Compare MQ operation counts
- Multiple grid size tests (1x1, 2x2, 4x4, 4x5, 5x4, 5x5)

**Note**: All debug logging should be removed after fixing the bug.
