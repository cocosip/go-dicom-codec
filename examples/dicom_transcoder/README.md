# DICOM Transfer Syntax Transcoder

一个专业的 DICOM 文件格式转换工具，可以将 DICOM 文件在不同的压缩格式（Transfer Syntax）之间进行转换。

## 功能特点

✨ **支持多种压缩格式**
- JPEG Baseline (有损, 8-bit)
- JPEG Lossless SV1 (无损)
- JPEG-LS Lossless (无损)
- JPEG 2000 Lossless (无损)
- JPEG 2000 Lossy (有损)

✨ **批量转换**
- 一次性生成所有格式的转换文件
- 自动创建独立的输出目录
- 每种格式生成独立的 DICOM 文件

✨ **友好的用户界面**
- 支持命令行参数
- 支持交互式输入
- 支持拖拽文件
- 程序执行完不会自动退出，需手动按 Enter 退出

## 快速开始

### 构建程序

```bash
cd examples/dicom_transcoder
go build -o dicom_transcoder.exe .
```

### 使用方法

#### 方法 1: 命令行参数

```bash
# Windows
dicom_transcoder.exe "C:\path\to\your\file.dcm"

# Linux/macOS
./dicom_transcoder /path/to/your/file.dcm
```

#### 方法 2: 拖放文件

将 DICOM 文件直接拖放到 `dicom_transcoder.exe` 上

#### 方法 3: 交互式输入

```bash
# 直接运行程序
dicom_transcoder.exe

# 然后按提示输入文件路径或拖放文件到控制台窗口
Enter DICOM file path (or drag and drop file here):
```

## 使用示例

### 示例输出

```
╔════════════════════════════════════════════════════════════════╗
║         DICOM Transfer Syntax Transcoder                       ║
║         Converts DICOM files between compression formats       ║
╚════════════════════════════════════════════════════════════════╝

📂 Input file: C:\DICOM\chest_xray.dcm

⏳ Reading DICOM file...
✓ Successfully read DICOM file
  Source Transfer Syntax: 1.2.840.10008.1.2.1

📋 Image Information:
  Rows: 512
  Columns: 512
  Bits Stored: 16
  Samples Per Pixel: 1
  Photometric Interpretation: MONOCHROME2
  Modality: CR

📁 Output directory: C:\DICOM\chest_xray_transcoded

══════════════════════════════════════════════════════════════════
Starting transcoding process...
══════════════════════════════════════════════════════════════════

[1/5] Transcoding to JPEG Baseline (Lossy 8-bit)
      Transfer Syntax: 1.2.840.10008.1.2.4.50
      ✓ Success!
      📊 Size: 524.3 KB → 45.2 KB (11.60x compression)
      💾 Output: chest_xray_jpeg_baseline.dcm

[2/5] Transcoding to JPEG Lossless SV1
      Transfer Syntax: 1.2.840.10008.1.2.4.70
      ✓ Success!
      📊 Size: 524.3 KB → 312.5 KB (1.68x compression)
      💾 Output: chest_xray_jpeg_lossless_sv1.dcm

[3/5] Transcoding to JPEG-LS Lossless
      Transfer Syntax: 1.2.840.10008.1.2.4.80
      ✓ Success!
      📊 Size: 524.3 KB → 178.9 KB (2.93x compression)
      💾 Output: chest_xray_jpegls_lossless.dcm

[4/5] Transcoding to JPEG 2000 Lossless
      Transfer Syntax: 1.2.840.10008.1.2.4.90
      ✓ Success!
      📊 Size: 524.3 KB → 298.7 KB (1.76x compression)
      💾 Output: chest_xray_j2k_lossless.dcm

[5/5] Transcoding to JPEG 2000 Lossy
      Transfer Syntax: 1.2.840.10008.1.2.4.91
      ✓ Success!
      📊 Size: 524.3 KB → 156.3 KB (3.35x compression)
      💾 Output: chest_xray_j2k_lossy.dcm

══════════════════════════════════════════════════════════════════
Transcoding Summary
══════════════════════════════════════════════════════════════════
✓ Successful: 5
📁 Output directory: C:\DICOM\chest_xray_transcoded
══════════════════════════════════════════════════════════════════

──────────────────────────────────────────────────────────────────
Press Enter to exit...
```

### 输出文件结构

```
原始文件目录/
├── chest_xray.dcm                          # 原始文件
└── chest_xray_transcoded/                  # 自动创建的输出目录
    ├── chest_xray_jpeg_baseline.dcm        # JPEG Baseline 格式
    ├── chest_xray_jpeg_lossless_sv1.dcm    # JPEG Lossless SV1 格式
    ├── chest_xray_jpegls_lossless.dcm      # JPEG-LS Lossless 格式
    ├── chest_xray_j2k_lossless.dcm         # JPEG 2000 Lossless 格式
    └── chest_xray_j2k_lossy.dcm            # JPEG 2000 Lossy 格式
```

## 支持的传输语法

| 格式名称 | Transfer Syntax UID | 类型 | 典型压缩比 | 位深度支持 | 适用场景 |
|---------|---------------------|------|-----------|-----------|---------|
| **JPEG Baseline** | 1.2.840.10008.1.2.4.50 | 有损 | ~12-30x | **仅8-bit** | Web预览、缩略图 |
| **JPEG Lossless SV1** | 1.2.840.10008.1.2.4.70 | 无损 | ~1.7x | **2-16 bit** ✅ | 传统系统兼容 |
| **JPEG-LS Lossless** | 1.2.840.10008.1.2.4.80 | 无损 | ~3-4x | **2-16 bit** ✅ | 最佳无损压缩 ⭐ |
| **JPEG 2000 Lossless** | 1.2.840.10008.1.2.4.90 | 无损 | ~1.8x | **2-16 bit** ✅ | 现代标准 |
| **JPEG 2000 Lossy** | 1.2.840.10008.1.2.4.91 | 有损 | ~3-7x | **2-16 bit** ✅ | 高质量有损压缩 |

> **✅ 16-bit 完全支持**: 从版本 December 2025 起，所有无损和有损编解码器（除 JPEG Baseline）都完全支持 12-16 位医学图像，实现完美的无损重建。

## 编码格式选择指南

### 医学影像应用场景

#### 🏥 诊断用途（必须无损）
推荐使用：
1. **JPEG-LS Lossless** - 最佳压缩比，快速编解码
2. **JPEG 2000 Lossless** - 现代标准，良好兼容性
3. **JPEG Lossless SV1** - 传统系统支持最好

#### 📁 归档存储（空间优先）
- **首选**: JPEG-LS Lossless（最高无损压缩率）
- **备选**: JPEG 2000 Lossless

#### 🌐 Web浏览/快速预览
- **首选**: JPEG 2000 Lossy（高质量，适中文件大小）
- **备选**: JPEG Baseline（最高压缩率，最小文件）

#### 🔄 跨系统兼容
- **首选**: JPEG Lossless SV1（最广泛支持）
- **备选**: 未压缩格式

### 不同影像类型推荐

| 影像类型 | 推荐格式 | 原因 |
|---------|---------|------|
| **CT/MRI** | JPEG-LS Lossless | 高压缩比，完美重建 |
| **X光** | JPEG-LS Lossless | 适合灰度图像 |
| **超声** | JPEG 2000 Lossy (高质量) | 可接受轻微质量损失 |
| **病理切片** | JPEG 2000 Lossless | 色彩丰富，需要无损 |
| **数字X光** | JPEG Lossless SV1 | 兼容性最佳 |

## 常见问题

### Q: 为什么程序执行完不自动退出？
A: 这是特意设计的，方便用户查看转换结果和可能的错误信息。按 Enter 键即可退出。

### Q: 输出文件保存在哪里？
A: 在输入文件的同一目录下，会自动创建一个名为 `原文件名_transcoded` 的文件夹。

### Q: 支持哪些输入格式？
A: 支持任何有效的 DICOM 文件，包括未压缩和已压缩的各种格式。

### Q: 无损压缩是否真的"无损"？
A: 是的，无损格式保证像素级别的完美重建，适用于诊断用途。

### Q: 有损压缩会影响诊断吗？
A: 有损压缩（如 JPEG Baseline）会损失部分图像细节，**不推荐用于诊断目的**，仅用于预览和归档参考。

### Q: 转换后的文件可以用其他DICOM浏览器打开吗？
A: 可以，转换后的文件是标准的 DICOM 文件，可以被任何支持相应传输语法的 DICOM 软件打开。

### Q: 遇到 "unexpected pixel data element type" 错误怎么办？
A: 这个错误已经在最新版本中修复。程序使用 `parser.ReadAll` 选项来确保像素数据被完全读取。如果仍然遇到问题：
1. 确保你使用的是最新版本的程序
2. 检查输入的 DICOM 文件是否有效（可以用其他 DICOM 查看器打开）
3. 尝试使用其他 DICOM 文件测试

### Q: 某些格式转换失败怎么办？
A: 不同的编码格式有不同的要求：
- **JPEG Baseline 只支持 8-bit 图像** - 如果你的图像是 16-bit，程序会自动跳过此格式并建议使用 JPEG 2000 Lossy 代替
- 某些压缩格式可能不支持特定的光度解释
- 如果源文件已经是压缩格式，会先解压再重新压缩
- 查看错误信息了解具体原因

### Q: 我的 16-bit 图像无法转换为 JPEG Baseline 怎么办？
A: 这是正常的。JPEG Baseline (1.2.840.10008.1.2.4.50) 标准只支持 8-bit 图像。对于 16-bit 医学图像，推荐：
- **有损压缩**: 使用 JPEG 2000 Lossy（压缩比 3-7x，质量很好）
- **无损压缩**: 使用 JPEG-LS Lossless（压缩比 3-4x，完美重建）

## 技术实现

本工具基于以下核心组件：
- **go-dicom**: DICOM 文件解析和写入
- **go-dicom-codec**: 各种图像编解码器实现
- **Transcoder**: 自动处理不同传输语法间的转换

### 转换流程

```
读取DICOM文件
    ↓
解析元数据和像素数据
    ↓
创建转码器 (Transcoder)
    ↓
如果需要：解压缩 → 重新压缩
    ↓
更新传输语法元数据
    ↓
写入新的DICOM文件
```

## 性能参考

在典型的 512×512 16-bit 医学图像上：

| 操作 | 时间 |
|------|------|
| 读取DICOM | ~50ms |
| JPEG-LS 编码 | ~100ms |
| JPEG 2000 编码 | ~200ms |
| JPEG Baseline 编码 | ~80ms |
| 完整转换（5种格式） | ~800ms |

*测试环境: Intel Core i7, 16GB RAM, SSD*

## 相关链接

- [项目主页](../../README.md)
- [DICOM 标准](https://www.dicomstandard.org/)
- [Transfer Syntax 规范](https://dicom.nema.org/medical/dicom/current/output/chtml/part05/chapter_10.html)
- [go-dicom 库](https://github.com/cocosip/go-dicom)

## 许可证

本示例是 go-dicom-codec 项目的一部分。
