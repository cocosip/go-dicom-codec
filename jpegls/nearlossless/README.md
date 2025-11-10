# JPEG-LS Near-Lossless

**Transfer Syntax UID**: 1.2.840.10008.1.2.4.81

## 状态

⏳ **开发中** - JPEG-LS 近无损压缩实现

## 技术规范

JPEG-LS Near-Lossless (ISO/IEC 14495-1 / ITU-T T.87) 扩展了 LOCO-I 算法：
- **压缩类型**: 近无损 (可控误差)
- **位深度**: 2-16 bit
- **NEAR 参数**: 最大误差界限 (0=无损, >0=近无损)
- **颜色空间**: Grayscale, RGB, Multi-component

## NEAR 参数说明

NEAR 参数控制压缩质量和压缩比之间的权衡：

| NEAR | 含义 | 压缩比 | 应用场景 |
|------|------|--------|----------|
| 0 | 完全无损 | 基准 | 需要完美重建 |
| 1-3 | 极小误差 | +10-20% | 高质量医学影像 |
| 4-7 | 小误差 | +30-50% | 一般诊断 |
| 8-15 | 中等误差 | +60-100% | 预览/传输 |

**最大误差**: 每个像素的误差保证 ≤ NEAR

## JPEG-LS Near-Lossless vs Lossless

| 特性 | Near-Lossless (NEAR=3) | Lossless (NEAR=0) |
|-----|----------------------|-------------------|
| 压缩比 | ~1.3x 提升 | 基准 |
| 质量 | 最大误差 ±3 | 完美 (误差=0) |
| 速度 | 更快 (更少上下文) | 快 |
| 应用 | 高质量有损 | 无损存档 |

## 参考

- ITU-T T.87 (ISO/IEC 14495-1) - Section on NEAR parameter
- DICOM PS3.5 - Section 8.2.5
