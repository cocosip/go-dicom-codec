# DWT修复进展记录

**日期**: 2026-01-14
**任务**: 修复JPEG2000多级DWT变换bug并对齐OpenJPEG

---

## 问题发现

### 原始问题
- **症状**: Go编码器输出文件大小213KB，OpenJPEG输出172KB，差距40KB (~24%)
- **根本原因**: DWT多级变换每级错误地将LL系数缩小一半

### 测试验证 (test_dwt_levels.go)
使用常量数组 (所有值=590) 测试多级DWT：

**修复前**:
```
Level 1: LL[0] = 590 (ratio: 1.0000) ✓
Level 2: LL[0] = 295 (ratio: 0.5000) ✗ 应该是590
Level 3: LL[0] = 148 (ratio: 0.2508) ✗ 应该是590
Level 4: LL[0] = 74  (ratio: 0.1254) ✗ 应该是590
Level 5: LL[0] = 37  (ratio: 0.0627) ✗ 应该是590
```

**根因分析**:
- 第1级后数据布局: LL | HL / LH | HH (左上, 右上, 左下, 右下)
- 第2级应只处理LL区域(左上角)，但stride应保持原始宽度
- 原实现错误地使用缩小后的width作为stride，导致列访问越界到HL区域

---

## 修复方案

### 参考OpenJPEG实现
OpenJPEG的关键设计 (`dwt.c`):
```c
// opj_dwt_encode_and_deinterleave_v函数接收stride参数
static void opj_dwt_encode_and_deinterleave_v(
    void *array,
    void *tmp,
    OPJ_UINT32 height,
    OPJ_BOOL even,
    OPJ_UINT32 stride_width,  // ← 原始完整宽度
    OPJ_UINT32 cols)
{
    // 列访问: array[k * stride_width + c]
    // 保持原始stride，正确访问LL区域的列数据
}
```

### Go实现修复

#### 1. dwt53.go修复
- **Forward53_2D**: 添加`stride`参数，行列访问改用`data[y*stride+x]`
- **Inverse53_2D**: 同样添加`stride`参数
- **ForwardMultilevel**: 保持`originalStride := width`，每级使用原始stride
- **InverseMultilevel**: 同样保持原始stride

#### 2. dwt97.go修复
- **Forward97_2D**: 添加`stride`参数
- **Inverse97_2D**: 添加`stride`参数
- **ForwardMultilevel97**: 保持原始stride
- **InverseMultilevel97**: 保持原始stride

#### 3. 单元测试更新
- `dwt53_test.go`: 所有`Forward53_2D/Inverse53_2D`调用添加stride参数
- `dwt97_test.go`: 所有`Forward97_2D/Inverse97_2D`调用添加stride参数

---

## 修复效果

### 1. DWT DC值保持验证 ✅

**修复后测试结果** (test_dwt_levels.go):
```
Level 1: LL[0] = 590 (ratio: 1.0000) ✓
Level 2: LL[0] = 590 (ratio: 1.0000) ✓
Level 3: LL[0] = 590 (ratio: 1.0000) ✓
Level 4: LL[0] = 590 (ratio: 1.0000) ✓
Level 5: LL[0] = 590 (ratio: 1.0000) ✓
```
**结论**: 所有级别完美保持DC值！

### 2. 单元测试全部通过 ✅

**dwt53测试**:
```
=== RUN   TestForwardInverseMultilevel
    dwt53_test.go:166: Perfect reconstruction for 64x64 with 1 levels
    dwt53_test.go:166: Perfect reconstruction for 64x64 with 2 levels
    dwt53_test.go:166: Perfect reconstruction for 64x64 with 3 levels
    dwt53_test.go:166: Perfect reconstruction for 128x128 with 5 levels
    dwt53_test.go:166: Perfect reconstruction for 256x256 with 6 levels
--- PASS: TestForwardInverseMultilevel (0.00s)
```

**dwt97测试**:
```
=== RUN   TestForwardInverseMultilevel97
    --- PASS: TestForwardInverseMultilevel97/16x16_1-level (0.00s)
    --- PASS: TestForwardInverseMultilevel97/32x32_2-level (0.00s)
    --- PASS: TestForwardInverseMultilevel97/64x64_3-level (0.00s)
    --- PASS: TestForwardInverseMultilevel97/128x128_4-level (0.00s)
--- PASS: TestForwardInverseMultilevel97 (0.00s)
```

### 3. 编码器输出改善 🎯

#### DWT系数数量级修复
**修复前**:
```
DWT输出: 36, 38, 39...
T1输入 (缩放后): 2304, 2432... (36 << 6)
```

**修复后**:
```
DWT输出: 593, 591, 592, 592...
T1输入 (缩放后): 37952, 37824, 37888, 37888...
```
与OpenJPEG数量级匹配 (OpenJPEG: 37888, 37824, 37888...)

#### 文件大小显著改善
| 版本 | 文件大小 | 与OpenJPEG差距 |
|------|----------|----------------|
| **修复前** | 213,803 bytes (208.8 KB) | +40.0 KB (+23%) |
| **修复后** | 187,652 bytes (183.3 KB) | +14.6 KB (+8.5%) |
| **OpenJPEG目标** | 172,751 bytes (168.7 KB) | - |

**改善**: 差距从40KB缩小到14.6KB，**减少了63.5%的差距**！

---

## 仍存在的问题

### 1. DWT系数精确值差异

**Go编码器第一行系数**:
```
37952 37824 37888 37888 37888 37888 37952 37952...
```

**OpenJPEG第一行系数**:
```
37888 37824 37888 37888 37888 37824 37824 37824...
```

**分析**:
- 第1个系数: 37952 vs 37888 (差64，约0.17%)
- 第6个系数: 37888 vs 37824 (差64)
- 可能原因: DWT边界处理、舍入方式的细微差异

### 2. MaxBitplane差异

| 实现 | MaxBitplane | 说明 |
|------|-------------|------|
| Go | 16 | 从缩放后系数计算，然后减去6 |
| OpenJPEG | 10 | 直接从DWT系数计算 |

**分析**:
- OpenJPEG: max_coeff ≈ 38080, log2(38080) ≈ 15.2, bitplane = 10 (15 - 6 + 1)
- Go: 可能计算逻辑略有不同

### 3. Codeblock数量不匹配

| 实现 | 总码块数 | 各分辨率分布 |
|------|----------|--------------|
| Go | 121 | Res0:1, Res1:3, Res2:3, Res3:6, Res4:24, Res5:84 |
| 预期 | 454 | Res0:1, Res1:3, Res2:6, Res3:24, Res4:84, Res5:336 |

**分析**:
- Res2: 3 vs 6 (缺少一半)
- Res5: 84 vs 336 (缺少3/4)
- 可能原因: Subband划分、precinct尺寸计算有误

---

## 关键代码变更

### dwt53.go
```go
// Forward53_2D添加stride参数
func Forward53_2D(data []int32, width, height, stride int) {
    // 行访问: data[y*stride+x] 而不是 data[y*width+x]
    // 列访问: data[y*stride+x] 而不是 data[y*width+x]
}

// ForwardMultilevel保持原始stride
func ForwardMultilevel(data []int32, width, height, levels int) {
    originalStride := width
    curWidth := width
    curHeight := height

    for level := 0; level < levels; level++ {
        Forward53_2D(data, curWidth, curHeight, originalStride)
        curWidth = (curWidth + 1) / 2
        curHeight = (curHeight + 1) / 2
    }
}
```

### dwt97.go
```go
// Forward97_2D添加stride参数 (浮点版本)
func Forward97_2D(data []float64, width, height, stride int) {
    // 使用stride访问，与dwt53相同模式
}

// ForwardMultilevel97保持原始stride
func ForwardMultilevel97(data []float64, width, height, levels int) {
    originalStride := width
    curWidth := width
    curHeight := height

    for level := 0; level < levels; level++ {
        Forward97_2D(data, curWidth, curHeight, originalStride)
        curWidth = (curWidth + 1) / 2
        curHeight = (curHeight + 1) / 2
    }
}
```

---

## 下一步工作

### 优先级1: 调查剩余差异
1. **DWT系数精确值差异**
   - 对比OpenJPEG的5/3 lifting实现细节
   - 检查边界处理逻辑
   - 验证舍入方式

2. **Codeblock数量问题**
   - 检查subband尺寸计算
   - 验证precinct划分逻辑
   - 对比OpenJPEG的codeblock遍历顺序

3. **MaxBitplane计算**
   - 对齐OpenJPEG的numbps计算逻辑
   - 验证T1_NMSEDEC_FRACBITS的应用时机

### 优先级2: 端到端验证
- 多个测试图像验证
- 解码器往返测试
- 与OpenJPEG解码器交叉验证

### 优先级3: 性能优化
- SIMD优化 (参考OpenJPEG的SSE2/AVX2实现)
- 减少内存分配
- 并行处理

---

## 参考资料

### OpenJPEG关键文件
- `dwt.c`: DWT实现 (5/3和9/7)
  - `opj_dwt_encode_and_deinterleave_v`: 垂直变换，关键stride参数
  - `opj_dwt_encode_procedure`: 多级变换主循环
- `t1.c`: T1 EBCOT编码器
  - T1_NMSEDEC_FRACBITS = 6
  - 左移6位的scaling应用

### ISO/IEC 15444-1:2019 标准
- Annex F: DWT实现指南
- Annex D: T1 EBCOT编码

---

## 结论

✅ **DWT stride修复成功**：DC值保持完美，文件大小差距大幅缩小
⚠️ **仍有细节差异**：系数精确值、codeblock数量、maxBitplane需进一步对齐
🎯 **主要目标达成**：从213KB改善到187KB，接近172KB目标

此次修复是一个重大里程碑，为后续优化打下了坚实基础。
