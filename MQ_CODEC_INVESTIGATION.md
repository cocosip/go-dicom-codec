# MQ Codec Investigation - Operation #191 Divergence

## Summary

**Status:** Investigation in progress - root cause identified but correct fix not yet determined

**Test Pass Rate:** ~86% (32/38 subtests)

**Failing Tests:** 5×5, 10×10, 17×17, 18×18, 20×20

---

## Root Cause: MQ Encoder/Decoder State Desynchronization

### Operation #191 Divergence

**Encoder (encodes bit=1, which is LPS since mps=0):**
```
BEFORE: bit=1 ctx=16 state=14 mps=0 A=0xa402 C=0x0005ff32 ct=7
  qe=0x5601
  a -= qe → 0x4e01
  a < qe? YES (0x4e01 < 0x5601)
  c += qe → 0x00065533
  State transition: nlps[14]=14, switch[14]=1 → newMPS = 1-0 = 1
  renorme() → A=0x9c02, C=0x000caa66, ct=6
AFTER: A=0x9c02 C=0x000caa66 ct=6 newState=14 newMPS=1
```

**Decoder:**
```
BEFORE: ctx=16 state=14 mps=0 A=0xa402 C=0x53ee0000 ct=3
  qe=0x5601
  a -= qe → 0x4e01
  (c >> 16) < qe? YES (0x53ee < 0x5601) → LPS path
  a < qe? YES (0x4e01 < 0x5601) → Conditional exchange
  a = qe = 0x5601
  d = mps = 0 (returns MPS, not LPS!)
  State transition: nmps[14]=15 (not nlps[14]=14!)
  renormd() → A=0xac02, C=0xa7dc0000, ct=2
AFTER: bit=0 A=0xac02 C=0xa7dc0000 ct=2 newState=15 newMPS=0
```

**Key Discrepancy:**
- Encoder: Encodes LPS → State 14→14, MPS 0→1
- Decoder: Detects LPS path but returns MPS → State 14→15, MPS 0→0
- **States diverge permanently after this operation**

---

## OpenJPEG Reference Implementation

### Encoder LPS (`opj_mqc_codelps_macro`)
```c
a -= (*curctx)->qeval;
if (a < (*curctx)->qeval) {
    c += (*curctx)->qeval;
} else {
    a = (*curctx)->qeval;
}
*curctx = (*curctx)->nlps;  // ALWAYS uses nlps!
opj_mqc_renorme_macro(mqc, a, c, ct);
```

### Decoder LPS Exchange (`opj_mqc_lpsexchange_macro`)
```c
if (a < (*curctx)->qeval) {
    a = (*curctx)->qeval;
    d = (*curctx)->mps;        // Returns MPS!
    *curctx = (*curctx)->nmps;  // Uses nmps!
} else {
    a = (*curctx)->qeval;
    d = !((*curctx)->mps);     // Returns LPS
    *curctx = (*curctx)->nlps;  // Uses nlps
}
```

**The Paradox:**
- Our encoder matches `codelps_macro` (uses nlps)
- Our decoder matches `lpsexchange_macro` (uses nmps when `a < qe`, nlps otherwise)
- Yet they're incompatible with each other!

---

## Failed Fix Attempts

### Attempt 1: Modified Decoder to Always Use nlps in LPS Path
- **Rationale:** Match encoder's behavior (always use nlps when encoding LPS)
- **Result:** 100% failure rate (all tests except 2×2 failed)
- **Why it failed:** Desynchronized even earlier (diverged before operation #191)

### Attempt 2: Modified Encoder to Use nmps When `a < qe` in LPS Path
- **Rationale:** Match decoder's conditional exchange behavior
- **Result:** Only 2×2 passed, all others 100% failure
- **Why it failed:** This contradicts OpenJPEG's `codelps_macro`

---

## Theoretical Analysis

### Conditional Exchange Concept
When `a < qe` after subtraction, the probability intervals are "swapped":
- The smaller interval (qe) is assigned to one symbol
- The larger interval (a, which was < qe, now becomes qe) is assigned to the other

**Encoder's Perspective:**
- Wants to encode LPS
- Calculates `a -= qe`
- If `a < qe`: conditional exchange occurs
- Question: Does this mean the actual encoded symbol becomes MPS?

**Decoder's Perspective:**
- Detects `(c >> 16) < qe` → small interval → LPS path
- Checks `a < qe` → conditional exchange occurred
- If YES: small interval now represents MPS → return MPS, use nmps
- If NO: small interval represents LPS → return LPS, use nlps

**The Inconsistency:**
- Encoder: LPS encoding → always nlps (regardless of exchange)
- Decoder: LPS path detected → nmps if exchange, nlps if no exchange

This suggests encoder and decoder have different interpretations of what "exchange" means for state transitions.

---

## Hypotheses

### Hypothesis 1: Encoder Implementation is Wrong
- OpenJPEG's `codelps_macro` may be incorrect or incomplete
- Real implementation might handle conditional exchange differently
- Need to find actual OpenJPEG encoder usage, not just macros

### Hypothesis 2: Decoder Implementation is Wrong
- Our interpretation of `lpsexchange_macro` might be incorrect
- Perhaps it's only called in specific scenarios we're missing
- Need to see how OpenJPEG's `decode_macro` orchestrates the exchange macros

### Hypothesis 3: Context State Pre-condition Violation
- Perhaps operation #191 shouldn't have state=14 in the first place
- Earlier operations might have incorrect state transitions
- This would explain why both encoder and decoder appear "correct" in isolation

### Hypothesis 4: Missing Piece in ISO Standard
- The ISO/IEC 15444-1 spec might have subtle details we're missing
- Conditional exchange might have additional rules for state transitions
- Need to read the standard more carefully, especially Annex C

---

## Next Investigation Steps

1. **Trace Earlier Operations**
   - Enable debug for operations #1-190
   - Find first state mismatch before #191
   - Check if context states were already wrong

2. **Study OpenJPEG's Complete Flow**
   - Find how `decode_macro` calls `lpsexchange_macro`
   - Understand when each exchange macro is invoked
   - Check for any pre/post-conditions

3. **Read ISO Standard Section C.3.2**
   - Focus on conditional exchange description
   - Understand exact semantics of interval assignment
   - Clarify state transition rules during exchange

4. **Compare with Working Implementation**
   - Find another JPEG2000 T1 implementation (Kakadu, JasPer)
   - Compare their MQ codec logic
   - Identify discrepancies with OpenJPEG

5. **Binary Compatibility Test**
   - Encode with OpenJPEG, decode with our implementation
   - Encode with ours, decode with OpenJPEG
   - Isolate whether encoder or decoder is wrong

---

## State Tables Reference

```
state 14:
  qe = 0x5601
  nmps = 15
  nlps = 14  (self-loop!)
  switch = 1 (MPS bit flips)
```

When at state 14 with mps=0:
- Encoding MPS (bit=0) → nmps[14]=15, mps stays 0
- Encoding LPS (bit=1) → nlps[14]=14, switch → mps becomes 1

**Why This Matters:**
- nlps[14]=14 is a self-loop
- But switch[14]=1 flips MPS bit
- So encoding LPS at (state=14, mps=0) stays at state 14 but with mps=1
- But nmps[14]=15 transitions to a different state
- This creates two divergent paths from the same starting point

---

## Conclusion

The bug is definitively in the conditional exchange logic during LPS encoding/decoding. However, the correct fix requires deeper understanding of either:

1. How OpenJPEG actually uses these macros in practice, OR
2. What the ISO standard precisely specifies about conditional exchange state transitions

Simply matching encoder to OpenJPEG's `codelps_macro` and decoder to OpenJPEG's `lpsexchange_macro` is insufficient - they are **intentionally asymmetric** but we don't yet understand the compensating mechanism that makes them work together.

---

**Date:** 2025-01-21
**Investigator:** Claude Code
**Status:** Blocked - need additional reference material or deeper ISO standard analysis
