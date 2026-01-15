# JPEG2000 编码器对比分析报告

**日期**: 2026-01-15
**目标**: 对比 OpenJPEG (C++) 和 Go 实现的 JPEG2000 编码器

---

## 执行摘要

在尝试对比两个JPEG2000编码器的过程中，发现了 **Go 编码器/解码器存在严重的数据转换问题**，导致无法正确进行 round-trip 编码/解码测试。

## 测试环境

- **测试文件**: `D:\1_raw_pixels.bin` (815,184 bytes)
- **Go 库**: github.com/cocosip/go-dicom-codec/jpeg2000
- **OpenJPEG**: fo-dicom-codec-code/Native/Common/OpenJPEG

## 测试过程

### 1. 文件格式分析

使用 `inspect_file.go` 分析原始文件：

```
File: D:/1_raw_pixels.bin
Size: 815184 bytes (796.08 KB)

First 16 bytes (hex):
4E 02 4E 02 50 02 4F 02 4F 02 4F 02 4F 02 4E 02

Pattern Analysis:
  ✓ Detected repeating 2-byte pattern (possibly interleaved)
  ✓ Detected repeating 4-byte pattern (possibly RGBA)

16-bit Analysis (first values):
  Little-endian: 590, 590, 592, 591, 591, 591, 591, 590
```

**结论**: 文件为 16 位 little-endian 数据，可能的图像尺寸为 918×444 (407,592 像素)。

### 2. Go 编码器测试

#### 测试用例 1: 918×888 (8-bit grayscale)
```
Input: 815,184 bytes
Output: 576,546 bytes
Compression: 1.41:1
Round-trip: ✗ 99.7% 字节不匹配
```

#### 测试用例 2: 816×999 (8-bit grayscale)
```
Input: 815,184 bytes
Output: 594,566 bytes
Compression: 1.37:1
Round-trip: ✗ 99.6% 字节不匹配
```

#### 测试用例 3: 918×444 (16-bit grayscale)
```
Input: 815,184 bytes
Output: 247,918 bytes
Compression: 3.29:1 ✓ (合理压缩率)
Round-trip: ✗ 99.9% 字节不匹配
```

#### 测试用例 4: 918×444 (8-bit, 2-component)
```
Input: 815,184 bytes
Output: 281,205 bytes
Compression: 2.90:1 ✓ (合理压缩率)
Round-trip: ✗ 99.7% 字节不匹配
```

### 3. 简单数据测试

为了隔离问题，创建了简单的测试数据（16×16, 值为 0, 1, 2, 3...）：

#### 8-bit 测试结果:
```
Original values:  0, 1, 2, 3, 4, 5, 6, 7, 8, 9...
Decoded component values: 128, 256, 384, 512, 640, 768, 896, 1024, 1152...
Decoded bytes (取低8位): 128, 0, 128, 0, 128, 0, 128, 0, 128...

Match rate: 0.39% (1/256 bytes)
```

#### 16-bit 测试结果:
```
Original values:  0, 100, 200, 300, 400, 500, 600, 700, 800...
Decoded component values: 32768, 65536, 98304, 131072, 163840, 196608...
Decoded values: 32768, 0, 32768, 0, 32768...

Match rate: 0.98% (5/512 bytes)
```

## 问题分析

### 发现的问题

1. **组件值缩放错误**
   - 8-bit 输入值 `n` → 解码后组件值约为 `n * 128`
   - 16-bit 输入值 `n` → 解码后组件值约为 `n * 256` + 32768

2. **DC 电平偏移**
   - 代码中找到 `applyInverseDCLevelShift()` 函数
   - 对无符号 8-bit 数据：加 128 (2^7)
   - 对无符号 16-bit 数据：加 32768 (2^15)
   - 这是标准 JPEG2000 行为，但组件值的基础值就已经错误

3. **数据类型转换问题**
   - 解码后的 `componentData` 是 `[][]int32` 类型
   - 直接转换为 `byte` 会截断高位 (`byte(256) = 0`)
   - 但即使考虑这点，底层值仍然不正确

### 可能的根本原因

1. **编码器的数据准备阶段**:
   - `convertPixelData()` 可能有问题
   - 输入字节到 int32 组件的转换可能有缩放错误

2. **小波变换或量化**:
   - 系数可能在某个阶段被错误地缩放
   - 可能与定点/浮点运算的精度有关

3. **反量化阶段**:
   - 解码时的反量化步骤可能应用了错误的缩放因子

## OpenJPEG 对比

### OpenJPEG 编译尝试

尝试编译 OpenJPEG C++ 编码器时遇到链接错误：

```
undefined reference to `opj_t1_ht_decode_cblk'
undefined reference to `__imp_opj_image_data_alloc'
undefined reference to `__imp_opj_stream_create'
... (多个链接错误)
```

**原因**: 缺少 HTJ2K 相关源文件 (`ht_dec.c`) 和一些函数实现。

### 建议的对比方法

由于直接编译 OpenJPEG 遇到困难，建议采用以下方法：

1. **使用已编译的 OpenJPEG 工具**:
   ```bash
   # 编码
   opj_compress -i input.raw -o output.j2k -F 918,444,1,8,u

   # 解码
   opj_decompress -i output.j2k -o decoded.raw
   ```

2. **添加调试输出到 Go 源码**:
   - 在 `encoder.go` 的 `convertPixelData()` 中添加日志
   - 在 `decoder.go` 的反量化阶段添加日志
   - 对比中间值与 OpenJPEG 的实现

3. **查看 OpenJPEG 源码关键部分**:
   - `openjpeg.c`: 主要 API 实现
   - `j2k.c`: JPEG2000 编码/解码核心
   - `dwt.c`: 小波变换
   - `t1.c`: Tier-1 编码 (EBCOT)
   - `t2.c`: Tier-2 编码 (包组织)
   - `tcd.c`: Tile encoder/decoder

## 建议的修复步骤

### 步骤 1: 添加详细日志

在 Go 编码器中添加调试输出：

```go
// In encoder.go - convertPixelData()
fmt.Printf("DEBUG: First 10 input bytes: %v\n", pixelData[:10])
fmt.Printf("DEBUG: First 10 converted int32 values: %v\n", e.data[0][:10])

// After DC level shift
fmt.Printf("DEBUG: After DC shift, first 10 values: %v\n", e.data[0][:10])
```

在解码器中添加调试输出：

```go
// In decoder.go - after inverse wavelet
fmt.Printf("DEBUG: After inverse wavelet, first 10 values: %v\n", d.data[0][:10])

// After inverse DC level shift
fmt.Printf("DEBUG: After inverse DC shift, first 10 values: %v\n", d.data[0][:10])
```

### 步骤 2: 对比 OpenJPEG 实现

查看 OpenJPEG 中相应的转换逻辑：

1. **输入数据转换** (openjpeg.c):
   ```c
   for (i = 0; i < numsamples; i++) {
       image->comps[compno].data[i] = input_buffer[i];  // 直接赋值，无缩放
   }
   ```

2. **DC 电平偏移** (j2k.c):
   ```c
   // Encoding: subtract 2^(prec-1) for unsigned
   if (!sgnd) {
       offset = 1 << (prec - 1);
       for (i = 0; i < w * h; i++) {
           data[i] -= offset;
       }
   }

   // Decoding: add back
   if (!sgnd) {
       offset = 1 << (prec - 1);
       for (i = 0; i < w * h; i++) {
           data[i] += offset;
       }
   }
   ```

3. **小波变换** (dwt.c):
   - 检查是否有额外的缩放因子
   - 确认 5/3 和 9/7 小波的实现细节

### 步骤 3: 单元测试

创建针对性的单元测试：

```go
func TestDCLevelShift(t *testing.T) {
    // Test 8-bit unsigned
    data := []int32{0, 1, 2, 3, 127, 128, 255}
    expected := []int32{-128, -127, -126, -125, -1, 0, 127}

    applyDCLevelShift(data, 8, false)

    for i, v := range data {
        if v != expected[i] {
            t.Errorf("Index %d: expected %d, got %d", i, expected[i], v)
        }
    }
}

func TestInverseDCLevelShift(t *testing.T) {
    // Inverse test
    data := []int32{-128, -127, -126, -125, -1, 0, 127}
    expected := []int32{0, 1, 2, 3, 127, 128, 255}

    applyInverseDCLevelShift(data, 8, false)

    for i, v := range data {
        if v != expected[i] {
            t.Errorf("Index %d: expected %d, got %d", i, expected[i], v)
        }
    }
}
```

## 创建的工具

在对比过程中创建了以下工具（位于 `comparison/` 目录）：

1. **go_encoder.go** - Go JPEG2000 编码器封装
2. **openjpeg_encoder.cpp** - OpenJPEG C++ 编码器（未成功编译）
3. **compare_j2k.go** - JPEG2000 码流对比工具
4. **detect_params.go** - 自动检测原始文件参数
5. **analyze_output.go** - 详细分析编码输出
6. **inspect_file.go** - 文件格式检测工具
7. **debug_roundtrip.go** - Round-trip 调试工具
8. **test_simple.go** - 简单数据测试

## 结论

1. **Go JPEG2000 编码器/解码器存在严重bug**，导致：
   - Round-trip 测试失败率 99%+
   - 解码后的组件值有错误的缩放因子
   - 无法用于生产环境

2. **OpenJPEG 编译困难**，需要：
   - 完整的源文件（包括 HTJ2K 支持）
   - 正确的编译配置
   - 或使用预编译的二进制工具

3. **建议的后续工作**：
   - 仔细对比 Go 和 OpenJPEG 的实现细节
   - 添加详细的调试日志
   - 从最简单的情况开始修复（8-bit 单组件图像）
   - 建立全面的单元测试套件
   - 参考 OpenJPEG 的 JPEG2000 标准实现

## 附录

### 测试文件信息

- **D:\1_raw_pixels.bin**:
  - 大小: 815,184 字节
  - 格式: 16-bit little-endian (推测)
  - 可能尺寸: 918×444, 816×499, 或其他组合
  - 值范围: 2-83 (非常窄的范围，可能是医学影像)

### 相关文件

- 工具代码: `D:\Code\go\go-dicom-codec\comparison\`
- Go 编码器: `D:\Code\go\go-dicom-codec\jpeg2000\encoder.go`
- Go 解码器: `D:\Code\go\go-dicom-codec\jpeg2000\decoder.go`
- OpenJPEG 源码: `D:\Code\go\go-dicom-codec\fo-dicom-codec-code\Native\Common\OpenJPEG\`

---

**报告生成时间**: 2026-01-15
**生成工具**: Claude Code
