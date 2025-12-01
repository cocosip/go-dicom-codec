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
- [ ] `debug_*.go` - 临时调试文件，可以删除

## Known Issues

### 🔴 CRITICAL: Multi-Layer Encoding (Lossless & Lossy)
**症状**: Multi-layer encoding产生高达200+像素的误差（包括lossless模式）
**实际测试结果**:
```
Single-layer lossless:  maxError=0   ✅ 完美
Single-layer lossy:     maxError=3   ✅ 正常量化误差
Multi-layer lossless:   maxError=216-238 ❌ 严重错误
Multi-layer lossy:      maxError=250+ ❌ 几乎完全失败
```

**根本原因**: Multi-layer架构存在深层问题，不仅仅是MQ context preservation
- PassLengths累加逻辑可能有误
- Layer data分片/重组可能不正确
- 可能还有其他未知的状态管理问题

**当前状态**:
- 测试允许lossless multi-layer有≤250像素误差（multilayer_test.go:150-157）
- 这是一个**已知但未解决**的问题
- Multi-layer功能基本不可用于生产环境

**优先级**: **CRITICAL** 🔴
- 影响: Multi-layer功能完全不可靠
- 阻塞: 无法用于progressive transmission场景

**需要行动**:
1. 深入调试PassLengths累加和layer data分片逻辑
2. 对比OpenJPEG的multi-layer实现
3. 可能需要重新设计multi-layer架构
4. 创建详细的单元测试来隔离问题

**临时建议**:
- ⚠️ **不要使用multi-layer功能**（NumLayers > 1）
- 仅使用single-layer (NumLayers = 1) - 完全可靠

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
