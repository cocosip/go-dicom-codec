# JPEG2000 编码器对比工具 - 项目总结

## 项目概述

本项目提供了一套完整的工具，用于对比 OpenJPEG (C++) 和 Go 实现的 JPEG2000 编码器的编码结果。

## 已创建的文件

### 1. 编码器实现

#### `cpp/openjpeg_encoder.cpp`
- **功能**: 使用 OpenJPEG 库对原始像素数据进行 JPEG2000 编码
- **语言**: C++
- **依赖**: OpenJPEG 源码 (位于 `../fo-dicom-codec-code/Native/Common/OpenJPEG`)
- **编译**:
  ```bash
  g++ -o openjpeg_encoder.exe openjpeg_encoder.cpp \
    -I../../fo-dicom-codec-code/Native/Common/OpenJPEG \
    ../../fo-dicom-codec-code/Native/Common/OpenJPEG/*.c \
    -lm -lpthread -lstdc++
  ```
- **使用**:
  ```bash
  openjpeg_encoder.exe <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]
  ```

#### `go_encoder.go`
- **功能**: 使用 go-dicom-codec 库对原始像素数据进行 JPEG2000 编码
- **语言**: Go
- **依赖**: github.com/cocosip/go-dicom-codec/jpeg2000
- **使用**:
  ```bash
  go run go_encoder.go <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]
  ```

### 2. 对比分析工具

#### `compare_j2k.go`
- **功能**: 详细对比两个 JPEG2000 编码器的输出结果
- **分析内容**:
  - 文件大小对比
  - 码流标记(Marker)序列对比
  - 标记内容字节级对比
  - 解码后像素数据对比
- **使用**:
  ```bash
  # 基本对比
  go run compare_j2k.go output_openjpeg.j2k output_go.j2k

  # 详细对比
  go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed
  ```

#### `detect_params.go`
- **功能**: 根据文件大小自动检测可能的图像参数
- **输出**:
  - 标准图像尺寸匹配
  - 质因数分解
  - 可能的自定义尺寸组合
- **使用**:
  ```bash
  go run detect_params.go <input.bin>
  ```

### 3. 自动化脚本

#### `run_comparison.bat` (Windows)
- **功能**: 一键完成编译、编码、对比全流程
- **使用**:
  ```batch
  run_comparison.bat D:\1_raw_pixels.bin 918 888 1 8 0
  ```

#### `run_comparison.sh` (Linux/Mac)
- **功能**: 一键完成编译、编码、对比全流程 (Unix版本)
- **使用**:
  ```bash
  chmod +x run_comparison.sh
  ./run_comparison.sh /path/to/1_raw_pixels.bin 918 888 1 8 0
  ```

### 4. 文档

#### `README.md`
- 完整的项目说明文档
- 安装、编译、使用说明
- 故障排查指南
- 高级使用技巧

#### `USAGE_EXAMPLE.md`
- 针对 `D:\1_raw_pixels.bin` 的详细使用示例
- 步骤化的操作指南
- 预期输出示例
- 结果分析方法

#### `SUMMARY.md` (本文件)
- 项目总结
- 文件清单
- 快速开始指南

## 快速开始

### 对 D:\1_raw_pixels.bin 进行编码对比

根据文件大小分析 (815,184 字节)，最可能的图像参数是 **918×888 灰度图像**。

#### 方法 1: 使用自动化脚本 (推荐)

```batch
cd D:\Code\go\go-dicom-codec\comparison
run_comparison.bat D:\1_raw_pixels.bin 918 888 1 8 0
```

#### 方法 2: 手动执行步骤

```batch
# 1. 检测参数
go run detect_params.go D:\1_raw_pixels.bin

# 2. 编译 OpenJPEG 编码器 (首次运行)
cd cpp
g++ -o openjpeg_encoder.exe openjpeg_encoder.cpp ^
  -I..\..\fo-dicom-codec-code\Native\Common\OpenJPEG ^
  ..\..\fo-dicom-codec-code\Native\Common\OpenJPEG\*.c ^
  -lm -lpthread -lstdc++
cd ..

# 3. OpenJPEG 编码
cpp\openjpeg_encoder.exe D:\1_raw_pixels.bin output_openjpeg.j2k 918 888 1 8 0

# 4. Go 编码
go run go_encoder.go D:\1_raw_pixels.bin output_go.j2k 918 888 1 8 0

# 5. 对比结果
go run compare_j2k.go output_openjpeg.j2k output_go.j2k

# 6. 详细对比 (可选)
go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed > comparison_detailed.txt
```

## 预期结果分析

### 1. 文件大小对比
- OpenJPEG 和 Go 的输出文件大小应该相近
- 差异通常在 5% 以内
- 较大差异可能表示编码策略不同

### 2. 标记序列对比
- 两者应该有相同的标记类型和顺序
- 主要标记: SOC → SIZ → COD → QCD → SOT → SOD → EOC
- 标记大小可能略有差异（取决于参数编码方式）

### 3. 解码验证
- **关键**: 对于无损编码，解码后的像素数据必须完全相同
- 如果有像素差异，说明至少一个编码器有 bug
- 有损编码才允许像素差异

### 4. 压缩率
- 医学影像的无损压缩率通常在 2:1 到 4:1 之间
- 自然图像可能更高
- 两个编码器的压缩率应该相似

## 对比指标说明

### ✓ 正常情况
```
File Sizes:
  OpenJPEG: 245,678 bytes
  Go:       247,123 bytes
  Difference: 1,445 bytes (0.59%)    ← 小于 5% ✓

Marker sequence: 8/8 matched (100%)   ← 完全匹配 ✓

Decoding Verification:
  ✓ Decoded pixels are identical!     ← 无损编码必须一致 ✓
```

### ✗ 异常情况
```
File Sizes:
  Difference: 15,445 bytes (6.29%)    ← 差异较大，需调查

Marker sequence: 6/8 matched (75%)    ← 标记序列不同，需调查

Decoding Verification:
  ✗ Pixel differences found: 1234     ← 无损编码有差异 = BUG!
```

## 对比差异的可能原因

### 文件大小差异
1. **编码顺序不同**: LRCP vs RLCP 等
2. **预分区大小**: 不同的预分区设置
3. **码块组织**: 码块边界处理方式
4. **量化表**: 量化步长表示方式
5. **可选标记**: COM、RGN 等可选标记的使用

### 像素差异 (无损编码)
1. **小波变换**: 5/3 可逆小波的实现差异
2. **DC 电平偏移**: DC level shift 计算错误
3. **码块编解码**: EBCOT T1 编码器 bug
4. **颜色空间**: RCT (可逆颜色变换) 实现错误
5. **边界处理**: 图像/Tile 边界的像素处理

## 进一步测试建议

### 1. 多样化测试
```bash
# 测试不同图像尺寸
run_comparison.bat test_64x64.bin 64 64 1 8 0
run_comparison.bat test_512x512.bin 512 512 1 8 0
run_comparison.bat test_1024x1024.bin 1024 1024 1 8 0

# 测试 RGB 图像
run_comparison.bat test_rgb.bin 512 512 3 8 0

# 测试 16 位图像
run_comparison.bat test_16bit.bin 512 512 1 16 0

# 测试有符号数据
run_comparison.bat test_signed.bin 512 512 1 12 1
```

### 2. 性能对比
```bash
# 测试编码速度
time cpp\openjpeg_encoder.exe input.bin out1.j2k 1024 1024 1 8 0
time go run go_encoder.go input.bin out2.j2k 1024 1024 1 8 0
```

### 3. 压缩率测试
- 测试不同类型的图像内容
- 医学影像 vs 自然图像
- 平坦区域 vs 高细节区域

### 4. 互操作性测试
```bash
# 使用 OpenJPEG 解码器解码 Go 的输出
opj_decompress -i output_go.j2k -o decoded_go.raw

# 使用 Go 解码器解码 OpenJPEG 的输出 (已在 compare_j2k.go 中实现)
```

## 故障排查

### OpenJPEG 编译失败
- 检查 g++ 是否已安装
- 确认 OpenJPEG 源码路径正确
- 尝试单独编译测试

### Go 编码失败
- 检查 Go 版本 (需要 1.18+)
- 运行 `go mod tidy`
- 查看具体错误信息

### 参数不匹配
- 使用 `detect_params.go` 检测正确参数
- 验证文件大小计算: width × height × components × bytes_per_sample
- 检查文件是否包含头部/元数据

### 解码失败
- 确认编码成功完成
- 检查 J2K 文件是否有效
- 使用外部工具验证 (opj_decompress 等)

## 技术细节

### 编码参数设置

两个编码器使用相同的参数：
- **小波**: 5/3 可逆小波 (无损)
- **分解层数**: 5 层
- **码块大小**: 64×64
- **预分区**: 默认 (32768)
- **层数**: 1 层
- **进展顺序**: LRCP
- **Tile**: 单 Tile (整幅图像)

### JPEG2000 码流结构

```
SOC  (Start of Codestream)
SIZ  (Image and tile size)
COD  (Coding style default)
QCD  (Quantization default)
[COM] (Comment, optional)
SOT  (Start of tile)
SOD  (Start of data)
  [Compressed bitstream]
EOC  (End of codestream)
```

### 关键测试点

1. **SIZ 标记**: 图像尺寸、位深度、分量数
2. **COD 标记**: 编码样式、分解层数、码块大小、进展顺序
3. **QCD 标记**: 量化参数
4. **压缩数据**: MQ 编码的码块数据

## 贡献与反馈

如果发现问题或有改进建议：
1. 记录详细的复现步骤
2. 保存输入文件和参数
3. 附上对比结果和错误信息
4. 提交 issue 或 pull request

## 参考资料

- [JPEG2000 标准 (ISO/IEC 15444-1)](https://www.iso.org/standard/78321.html)
- [OpenJPEG 官方文档](https://www.openjpeg.org/)
- [go-dicom-codec 项目](https://github.com/cocosip/go-dicom-codec)

## 许可证

与父项目相同。

---

**创建日期**: 2026-01-15
**版本**: 1.0
**维护者**: Claude Code
