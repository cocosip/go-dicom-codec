# JPEG2000 T1 Codec - Current Status

**Date:** 2025-01-21
**Test Pass Rate:** ~86% (failing on specific gradient patterns)

---

## 🔍 Root Cause Identified

### The Bug
**Location:** `jpeg2000/mqc/mqc.go` lines 144-149
**Operation:** MQ Decoder, LPS path, conditional exchange when `a < qe`

### Evidence
**First Divergence:** Operation #191
- Encoder encodes: bit=1 (LPS), newState=14, newMPS=1 (with switch)
- Decoder decodes: bit=0 (MPS), newState=15, newMPS=0 (wrong!)
- State before: ctx=16, state=14, mps=0, A=0xa402
- Both encoder/decoder perfectly synchronized for operations 1-190

### The Problem

When encoding LPS with conditional exchange (`a < qe`):
```go
// Encoder (CORRECT)
mqe.a -= qe
if mqe.a < qe {
    mqe.c += qe  // Shift C past the smaller MPS interval
}
newState = nlpsTable[state]  // LPS transition
if switchTable[state] == 1 {
    newMPS = 1 - mps  // Switch MPS bit
}
```

When decoding at `(c >> 16) < qe` AND `a < qe`:
```go
// Current Decoder
d = mps  // Returns MPS value (seems contradictory?)
*cx = nmpsTable[state] | (uint8(mps) << 7)  // MPS transition (NO SWITCH!)
```

**The Issue:** Decoder uses MPS state transition (no switch), while encoder used LPS transition (with switch). This causes state desynchronization!

---

## 🤔 The Paradox

**Observation 1:** Current decoder logic matches OpenJPEG reference
- OpenJPEG's `lpsexchange` also returns `mps` when `a < qe`
- OpenJPEG also uses `nmps` state transition

**Observation 2:** But tests still fail
- 5×5, 10×10, 17×17, 18×18, 20×20 all fail
- Divergence confirmed at operation #191

**Observation 3:** Attempted fix made it worse
- Changing to LPS state transition broke ALL tests (including previously passing ones)
- Suggests current logic is "more correct" but still has an issue

---

## ❓ Remaining Questions

1. **Is the encoder wrong instead of decoder?**
   - Should encoder use MPS transition in conditional exchange?
   - Need to verify against ISO standard

2. **Is there a third factor?**
   - Maybe context computation is wrong for specific patterns?
   - Maybe bitplane processing order differs?

3. **OpenJPEG compatibility**
   - Current decoder matches OpenJPEG logic
   - But does encoder also match OpenJPEG?
   - Should test interoperability

---

## 📝 Investigation Log

### Fixes Attempted

1. **updateNeighborFlags boundary check removal** ❌
   - Removed unnecessary boundary checks
   - Did not resolve test failures
   - But likely still a good fix (prevents inconsistency)

2. **MQ Decoder state transition fix** ❌
   - Changed LPS exchange to use LPS state transition
   - Broke ALL tests (0% pass rate)
   - Reverted immediately

### Tools Created

- `check_mq_diverge.go` - Confirms divergence at op #191
- `mq_debug_output.txt` - Complete trace of 250 MQ operations
- MQ debug logging (250 operations)

---

## 🎯 Next Steps

### Priority 1: Verify Encoder
Check if encoder's LPS conditional exchange logic is correct:
- Should it use LPS transition (with switch)?
- Or should it use MPS transition (no switch)?
- Compare with OpenJPEG encoder

### Priority 2: Test Interoperability
- Encode with this implementation, decode with OpenJPEG
- Encode with OpenJPEG, decode with this implementation
- Identify which direction fails

### Priority 3: Consult ISO Standard
- Read ISO/IEC 15444-1:2019 Section C.3.2 carefully
- Understand official conditional exchange semantics
- Verify both encoder and decoder against spec

---

## 📊 Test Results

### Currently Passing
- ✅ 3×3, 4×4, 6×6, 7×7, 8×8, 9×9
- ✅ 11×11, 12×12, 13×13, 14×14, 15×15, 16×16
- ✅ 19×19
- ✅ Uniform patterns
- ✅ Most gradient patterns

### Currently Failing
- ❌ 5×5 (48% error)
- ❌ 10×10 (94% error)
- ❌ 17×17 (99.3% error)
- ❌ 18×18 (100% error)
- ❌ 20×20 (99.5% error)

---

## 💡 Key Insights

1. **MQ Codec is subtle** - Small logic errors cause cascading failures
2. **State synchronization critical** - Encoder/decoder must use same state transitions
3. **Conditional exchange is confusing** - Interval swap affects both value AND state
4. **Simple fixes dangerous** - Must understand theory before修改

---

**Status:** Bug isolated but fix unclear
**Recommendation:** Need encoder/decoder symmetry analysis and ISO standard consultation
