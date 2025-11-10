# JPEG-LS Lossless

**Transfer Syntax UID**: 1.2.840.10008.1.2.4.80

## 状态

✅ **已完成** - JPEG-LS 无损压缩，所有测试通过

## 技术规范

JPEG-LS (ISO/IEC 14495-1 / ITU-T T.87) 基于 LOCO-I 算法：
- **压缩类型**: 无损
- **位深度**: 2-16 bit
- **颜色空间**: Grayscale, RGB, Multi-component
- **核心技术**:
  - MED (Median Edge Detection) 预测
  - 上下文自适应建模
  - Golomb-Rice 编码
  - Run mode 优化

## JPEG-LS vs JPEG Lossless

| 特性 | JPEG-LS | JPEG Lossless |
|-----|---------|---------------|
| 算法 | LOCO-I (上下文自适应) | 预测编码 + Huffman |
| 压缩率 | 更好 (~20-30% 改进) | 良好 |
| 复杂度 | 低 | 非常低 |
| 标准 | ISO/IEC 14495 | ITU-T T.81 |

## 测试结果

所有测试通过，验证了完美的无损压缩：

### 8-bit 性能
- ✅ 压缩比: 4.17x (grayscale), 2.51x (RGB)
- ✅ 完美重建: 0 错误

### 12-bit 性能
- ✅ 压缩比: 2.94x
- ✅ 完美重建: 0 错误

### 其他功能
- ✅ 参数验证
- ✅ 平坦区域优化 (2.89x)
- ✅ Codec 注册
- ✅ DICOM 集成

## 使用示例

```go
import "github.com/cocosip/go-dicom-codec/jpegls/lossless"

// 编码
jpegLSData, err := lossless.Encode(pixelData, width, height, components, bitDepth)

// 解码
pixels, w, h, comp, bd, err := lossless.Decode(jpegLSData)

// 使用 Codec 接口
codec := lossless.NewLosslessCodec(8)
params := codec.EncodeParams{
    PixelData:  pixelData,
    Width:      width,
    Height:     height,
    Components: components,
    BitDepth:   8,
}
encoded, err := codec.Encode(params)
```

## 参考

- ITU-T T.87 (ISO/IEC 14495-1)
- "The LOCO-I Lossless Image Compression Algorithm"
- DICOM PS3.5 - Section 8.2.4

---

**测试状态**: ✅ 100% 通过 (11/11 tests)
**最后更新**: 2025-11-10
