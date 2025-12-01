# Cleanup TODO

## Debug Code to Remove

以下文件包含调试输出，需要在后续commit中清理：

### Core Files
- [x] `t1/decoder.go` - ✅ 已清理 (设置 debugLH = false)
- [x] `t2/tile_decoder_fixed.go` - ✅ 已清理
- [x] `t2/packet_decoder.go` - ✅ 已清理
- [x] `t2/packet_encoder.go` - ✅ 已清理
- [x] `encoder.go` - ✅ 已清理
- [x] `decoder.go` - ✅ 已清理
- [x] `t2/tile_decoder.go` - ✅ 已清理

### Test Files
- [x] `debug_*.go` - ✅ 已删除临时调试文件

## Known Issues

### ✅ FIXED: Multi-Layer Lossless Encoding
**原症状**: Multi-layer lossless encoding产生200+ pixel errors
**根本原因**: Shared backing array导致packet data被覆盖

**问题详情**:
在`tile_decoder_fixed.go`中，从packet.Body提取code-block数据时使用了slice操作：
```go
cbData = packet.Body[dataOffset : dataOffset+cbIncl.DataLength]
```

这导致`cbData`与`packet.Body`共享同一个底层数组。当多个packets被处理时：
1. Packet 1的cbData被append到existing.data（共享backing array）
2. Packet 3的packet.Body重用了相同的内存区域
3. Packet 3的数据覆盖了Packet 1的数据
4. 最终decoder收到的是错误的数据

**修复方法**:
创建独立的copy而不是slice：
```go
// CRITICAL FIX: Create a copy instead of slicing to avoid shared backing array
cbData = make([]byte, cbIncl.DataLength)
copy(cbData, packet.Body[dataOffset:dataOffset+cbIncl.DataLength])
```

**修复后测试结果**:
```
Single-layer lossless:  maxError=0   ✅ 完美
Single-layer lossy:     maxError=3   ✅ 正常量化误差
Multi-layer lossless:   maxError=0   ✅ 完美！（已修复）
Multi-layer lossy:      maxError=TBD ⚠️ 需要进一步测试
```

**状态**: ✅ Lossless multi-layer已完全修复
**文件**: `t2/tile_decoder_fixed.go` line 48-49

### 🔴 CRITICAL: Multi-Layer Lossy Encoding
**状态**: 仍然存在严重问题
**症状**:
```
Test: multi_layer_lossy_test.go
Expected: maxError <= 100 (lossy mode合理误差)
Actual:   maxError = 250, errorCount = 4081/4096 (99.6%像素错误)

Original: [0 1 2 3 4 5 6 7 8 9]
Decoded:  [0 171 0 0 32 62 11 0 0 144]
```

**根本原因推测**:
虽然shared backing array问题已修复（解决了lossless问题），但lossy模式有额外的问题：
1. **MQ Context Preservation**: Lossy模式下，相邻pass之间应该保留context状态
2. **当前实现**: decoder.go line 136-140使用了WORKAROUND - 每个pass都创建新decoder
   ```go
   // Lossy: WORKAROUND - use fresh decoder until context preservation is fixed
   // TODO: Should use NewMQDecoderWithContexts to preserve contexts
   t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
   ```
3. **正确做法**: 应该使用`SetData()`更新data而保持contexts，或实现`NewMQDecoderWithContexts`

**修复优先级**: CRITICAL 🔴
- Lossless multi-layer: ✅ 已修复（maxError=0）
- Lossy multi-layer: ❌ 几乎完全失败

**需要行动**:
1. 实现MQ decoder的context preservation机制
2. 修改`DecodeLayeredWithMode`的lossy路径使用context preservation
3. 验证与OpenJPEG的行为一致性

**临时建议**:
- ✅ Lossless multi-layer可以使用（已修复）
- ❌ Lossy multi-layer仍然不可用

## Completed Fixes

✅ Debug代码清理完成
✅ Bug #1: PassLengths BaseOffset计算错误
✅ Bug #2: Upfront byte-unstuffing破坏packet边界
✅ Bug #3: BitReader未处理stuffed bytes
✅ Bug #4: Header reading错误safety check
✅ Bug #5: ZeroBitplanes未跨layer保存
✅ Bug #6: Lossless未使用PassLengths
✅ Bug #7: Code-block spatial position计算错误
✅ Bug #8: Large image encoder bug (512x512+) - 已通过byte-stuffing修复
✅ Bug #9: **Multi-layer lossless shared backing array** - CRITICAL修复
  - 文件: `t2/tile_decoder_fixed.go`
  - 问题: slice共享backing array导致packet data被后续packet覆盖
  - 修复: 使用`make+copy`创建独立copy而不是直接slice
  - 影响: Multi-layer lossless从完全失败(maxError=250)到完美工作(maxError=0)
  - 提交: 2025-12-01
