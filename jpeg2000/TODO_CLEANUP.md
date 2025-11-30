# Cleanup TODO

## Debug Code to Remove

以下文件包含调试输出，需要在后续commit中清理：

### Core Files
- [ ] `t1/decoder.go` - 移除 debugLH 和所有 fmt.Printf("[T1 DECODE...")
- [ ] `t2/tile_decoder_fixed.go` - 移除所有 fmt.Printf("[DEBUG...")、"[CREATE CB..."、"[DECODE CB..."、"[LOOP...]"、"[TOTAL...]"
- [ ] `t2/packet_decoder.go` - 移除 fmt.Printf("[PKT DEC...")
- [ ] `t2/packet_encoder.go` - 移除 fmt.Printf("[PKT ENC...")、"[ADDCODEBLOCK..."、"[ENCODE_HEADER..."
- [ ] `encoder.go` - 移除 fmt.Printf("[ENCODER LH...")

### Test Files (Lower Priority)
- [ ] `t2/tile_decoder.go` - 移除 fmt.Printf("[ASSEMBLE CB...")
- [ ] 临时测试文件 - 考虑是否保留或删除

## Known Issues

### Large Image Encoder Bug (512x512+)
**症状**: 512x512及更大图像的lossless编码/解码失败
**根因**: Encoder只为部分code blocks (5/48) 编码数据，其余未编码
**证据**:
```
256x256: 16 CBs, all encoded → PASS ✅
512x512: 64 CBs, only 5 encoded → FAIL ❌
```
**优先级**: HIGH - 影响large image support
**文件**: `encoder.go`, `t2/packet_encoder.go`
**下一步**: 调试encoder为何跳过某些code blocks

### Multi-Layer Context Preservation (Lossy Mode)
**症状**: Multi-layer lossy精度下降 (maxError: 1 → 45)
**当前方案**: Workaround - lossy模式使用fresh decoder (不保留contexts)
**影响**: Multi-layer lossy可用但精度略低
**优先级**: MEDIUM - 功能可用，性能待优化
**文件**: `t1/decoder.go:141-148`
**下一步**: 深入研究MQ encoder/decoder在TERMALL模式下的状态转换

## Completed Fixes

✅ Bug #1: PassLengths BaseOffset计算错误
✅ Bug #2: Upfront byte-unstuffing破坏packet边界
✅ Bug #3: BitReader未处理stuffed bytes
✅ Bug #4: Header reading错误safety check
✅ Bug #5: ZeroBitplanes未跨layer保存
✅ Bug #6: Lossless未使用PassLengths
✅ Bug #7: Code-block spatial position计算错误
✅ Decoder修复: 为not-included code blocks创建全0条目
