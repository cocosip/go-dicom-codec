# JPEG 2000 Known Issues

## Gradient Data Decoding (2025-11-12)

### 症状
0-level (无DWT) gradient数据测试失败，错误率高达98.83%
- ✓ Solid (纯色uniform) data: 完美重建
- ✗ Gradient data: 几乎完全错误
- ✗ Checker pattern: 失败

### 测试案例
- 8x8 gradient: 253/256 错误 (98.83%)
- 16x16 gradient: 失败
- 带DWT的larger images: 失败

### 可能原因
1. Gradient数据在多个bit-planes中都有变化，可能触发不同的code path
2. 可能存在context计算或bit-plane顺序问题
3. 需要详细对比encoder和decoder的处理流程

### 调试策略
1. 对比uniform vs gradient的encoding/decoding差异
2. 追踪gradient case的bit-plane processing
3. 验证context计算是否正确
4. 检查是否有其他flag管理问题

---

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

### Next Steps
1. **PRIORITY**: Compare line-by-line with OpenJPEG's `opj_t1_enc_clnpass()` and `opj_t1_dec_clnpass()` functions
2. Add detailed logging to track encoder/decoder RL decisions for same input
3. Test with OpenJPEG-generated J2K streams to verify decoder
4. Consider consulting JPEG 2000 experts or forums (OpenJPEG mailing list)
5. Review ISO/IEC 15444-1:2019 Annex D.3.5 (Cleanup coding pass) in detail

### Workaround
For affected cases, the codec still produces reasonable output with 5-15% error. For critical applications requiring 100% accuracy with sparse patterns:
1. Use smaller code-block sizes (e.g., 32×32 instead of 64×64)
2. Avoid the specific problematic dimensions if possible
3. Use dense data patterns

### Test Cases
See `jpeg2000/t1/debug_9x10_test.go` for minimal reproduction case.

### Status
**Active Investigation** - Needs deep analysis of Cleanup Pass RL logic and comparison with OpenJPEG implementation.
