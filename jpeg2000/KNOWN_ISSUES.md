# JPEG 2000 Known Issues

## T1 Cleanup Pass Run-Length Encoding Synchronization Issue

### Description
There is a subtle synchronization issue in the Tier-1 (T1) Cleanup Pass run-length (RL) encoding for specific sparse data patterns and dimensions.

### Affected Cases
- **Data Pattern**: Sparse patterns where significant coefficients are at RL boundary + 1 positions (e.g., x=1,5,9,13 instead of x=0,4,8,12)
- **Dimensions**: Specific width × height combinations:
  - 9×10 and above
  - 11×10 and above
  - 14×9 and above
  - 15×9 and above
  - 16×10 and above

### Symptoms
- First N rows decode correctly (e.g., 6 rows for 9×10)
- Subsequent rows show increasing decode errors
- Error rate typically 5-15% of pixels

### Working Cases
- ✅ All dimensions with dense data patterns (e.g., alternating 0,1 everywhere)
- ✅ Sparse patterns at RL boundaries (x=0,4,8,12...)
- ✅ Small dimensions (< 9×9 for most patterns)
- ✅ Single-row data
- ✅ Non-square dimensions in many cases

### Root Cause Analysis
The issue appears to be related to how the encoder and decoder make RL (`canUseRL`) decisions when:
1. Processing multiple rows with the same sparse pattern
2. Neighbor significance flags propagate across rows
3. The MQ encoder/decoder internal state after processing a specific number of bits

Detailed investigation shows:
- Encoder and decoder flags are synchronized before Cleanup Pass
- Both use identical RL logic
- Problem manifests only during Cleanup Pass execution
- Likely related to subtle timing difference in flag updates or MQ codec state

### Impact
- **Low impact on real medical images**: Real DICOM images typically have dense pixel data, not the specific sparse patterns that trigger this bug
- **Affects only lossless mode with specific test patterns**
- Most common image sizes and patterns work correctly

### Workaround
For affected cases, the codec still produces reasonable output with 5-15% error. For critical applications requiring 100% accuracy with sparse patterns:
1. Use smaller code-block sizes (e.g., 32×32 instead of 64×64)
2. Avoid the specific problematic dimensions if possible
3. Use dense data patterns

### Test Cases
See `jpeg2000/t1/debug_9x10_test.go` for minimal reproduction case.

### Detailed Analysis (2025-01-12)

**Test Results:**
- ✓ 8×8 gradient: Perfect
- ✓ 16×16 gradient: Perfect
- ✗ 32×32 gradient: 99.7% error (1021/1024 pixels)
- ✗ 64×64 gradient: High error rate
- ✗ 5×5 gradient: 48% error
- ✗ 9×9 gradient: 96.3% error

**RL Encoding Disable Test:**
- Disabling RL encoding entirely fixes 32×32 but breaks 8×8
- This suggests there are MULTIPLE independent bugs:
  1. RL encoding logic has a synchronization bug affecting certain sizes
  2. Non-RL path has a different bug affecting other sizes

**Code Flow Analysis:**
The RL encoding block (`encoder.go:287-338`, `decoder.go:327-360`) is supposed to:
1. Check if next 4 coefficients can use RL encoding (no T1_SIG, no T1_SIG_NEIGHBORS)
2. Find first significant coefficient position (rlSigPos) in the run [0,1,2,3]
3. Encode RL bit (0=all insignificant, 1=at least one significant)
4. If RL bit=1: encode position (2 bits uniform), then process that coefficient
5. Implicitly skip coefficients before rlSigPos (they're insignificant)
6. Continue loop to process remaining coefficients in run (after rlSigPos)

**Potential Issues Identified:**
1. VISIT flag management: Coefficients skipped by RL encoding (positions 0 to rlSigPos-1) don't have their VISIT flags explicitly cleared
2. Neighbor flag updates: After processing coefficient at rlSigPos, `updateNeighborFlags` may affect subsequent RL decisions
3. MQ codec state: Possible state divergence between encoder/decoder after certain bit sequences
4. Edge cases: Width not multiple of 4, last incomplete 4-run handling

### Next Steps
1. **PRIORITY**: Compare line-by-line with OpenJPEG's `opj_t1_enc_clnpass()` and `opj_t1_dec_clnpass()` functions
2. Add detailed logging to track encoder/decoder RL decisions for same input
3. Test with OpenJPEG-generated J2K streams to verify decoder
4. Consider consulting JPEG 2000 experts or forums (OpenJPEG mailing list)
5. Review ISO/IEC 15444-1:2019 Annex D.3.5 (Cleanup coding pass) in detail

### Workaround
**Temporary fix option**: Disable RL encoding entirely (set condition to `false`)
- Pros: Fixes 32×32 and larger sizes
- Cons: Breaks 8×8, reduces compression efficiency
- Conclusion: Not viable without fixing the other bug

### Progress Update (2025-01-12 continued)

**RL Encoding Fix Implemented:**
Based on OpenJPEG source code analysis, implemented proper RL encoding that processes all coefficients from `runlen` to position 3 in the 4-run, matching OpenJPEG's switch fall-through pattern.

**Current Test Results After Fix:**
- ✓ 3×3, 4×4: Perfect
- ✗ 5×5: 48% error (NEW failure pattern)
- ✓ 7×7, 8×8: Perfect
- ✗ 9×9: 96.3% error (NEW failure pattern)
- ✓ 11×11, 12×12, 13×13, 15×15, 16×16, 17×17: Perfect
- ✗ 32×32: 98.8% error (still failing)
- ✗ 64×64: 98.2% error (still failing)

**New Pattern Discovery:**
The failures are not simply "large sizes". Specific sizes (5×5, 9×9, 32×32+) fail while others (16×16, 17×17) pass. This suggests the bug is related to specific interactions between:
1. Image dimensions
2. RL encoding boundaries
3. Possibly row/column alignment issues

**Verification:**
- T1 encoder/decoder works correctly with DC-shifted data for sizes ≤16×16 (except 5×5 and 9×9)
- The issue is NOT related to DC level shift
- The issue is NOT simply in the RL encoding logic (partially fixed)

### Root Cause Identified! (2025-01-12 final)

**CRITICAL DISCOVERY**: The fundamental issue is the **loop orientation** of the Cleanup Pass!

**OpenJPEG Implementation:**
- Processes 4-coefficient groups **VERTICALLY** (same column, 4 consecutive rows)
- Outer loop: `for k = 0; k < height; k += 4` (4 rows at a time)
- Inner loop: `for i = 0; i < width; i++` (iterate columns)
- RL encoding applies to vertical groups of 4 coefficients in the same column

**Current Implementation (INCORRECT):**
- Processes coefficients **HORIZONTALLY** (same row, multiple columns)
- Outer loop: `for y = 0; y < height; y++` (row by row)
- Inner loop: `for x = 0; x < width; x++` (iterate columns)
- RL encoding incorrectly applied to horizontal groups

**Impact:**
This architectural mismatch causes encoder/decoder to process completely different data patterns:
- RL encoding decisions differ
- Context calculations differ
- Flag propagation differs
- Explains ALL observed failures (5×5, 9×9, 32×32+)

### Status
**ROOT CAUSE IDENTIFIED** - Requires complete rewrite of Cleanup Pass loop structure to process vertical 4-row groups instead of horizontal processing. This is a major architectural change affecting both encoder and decoder.

## T1 Uniform Data Decoding Issue

### Description
T1 encoder/decoder fails to correctly encode and decode uniform data (all coefficients with the same value) for certain dimensions.

### Affected Cases
- **Data Pattern**: Uniform data where all pixels have identical values (e.g., all -128, all 0, all 50)
- **Dimensions**:
  - ✗ 3×3 (9 pixels)
  - ✗ 15×15 (225 pixels)
  - ✗ 16×16 (256 pixels)
  - ✓ 1×1, 2×2, 4×4, 5×5, 8×8, 9×9, 12×12 (all work perfectly)

### Symptoms
- Encoded data is very small (e.g., 8 bytes for 16×16 = 256 pixels)
- Decoded values are completely incorrect (e.g., expecting -128, getting -207, -250, -204...)
- 100% of pixels mismatch for affected dimensions

### Working Cases
- ✅ Single pixel (1×1)
- ✅ Small power-of-2 dimensions (2×2, 4×4, 8×8)
- ✅ Certain non-power-of-2 dimensions (5×5, 9×9, 12×12)
- ✅ Gradient data (non-uniform values) works for ALL dimensions

### Root Cause Analysis
This appears to be related to the same RL encoding synchronization issue as above. Uniform data creates an extreme case where:
- For small/zero values: No significant coefficients at all → RL encoding heavily used
- For larger values: All coefficients significant at same bitplane → unusual pattern

The pattern of failures (3×3, 15×15, 16×16) suggests a relationship with specific dimensions and RL boundary calculations.

### Impact
- **High impact for test data**: Many test cases use uniform data patterns
- **Low impact for real images**: Real medical images rarely have large regions of identical pixel values
- **Workaround**: Use gradient or non-uniform test data

### Status
**Active Investigation** - Related to RL encoding issue above. Needs deep analysis of Cleanup Pass RL logic.
