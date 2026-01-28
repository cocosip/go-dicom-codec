# HTJ2K 编码器完整实现计划

## 📋 项目概述

**目标**: 实现符合 ITU-T.814 / ISO/IEC 15444-15:2019 标准的完整 HTJ2K 编码器

**当前状态**:
- ✅ 解码器：完整实现（VLC 表、MEL 解码、MagSgn 解码、指数预测器）
- ✅ 段格式：12位 Scup 格式对齐 OpenJPH
- ⚠️ 编码器：仅 raw mode（临时方案）

**问题**:
- 编码器使用 raw mode，不进行真正的 HTJ2K 压缩
- 小图像（≤64x64）测试通过
- 大图像（≥128x128）测试失败（raw mode 不适合多 codeblock 场景）

---

## 🎯 实施策略

### 分阶段实现

**Phase 1**: VLC 编码核心（关键路径）
**Phase 2**: Quad Pair 处理和上下文计算
**Phase 3**: 指数预测器和 UVLC
**Phase 4**: MEL/VLC 段融合和优化
**Phase 5**: 测试和验证

---

## 📝 详细实现计划

### Phase 1: VLC 编码核心 [优先级: 最高]

**目标**: 实现基本的 VLC 表查找和编码

#### Step 1.1: VLC 表数据结构 [~50 行]
**文件**: `vlc_tables.go` (新建)

**任务**:
- [ ] 定义 VLC 表条目结构
  ```go
  type VLCEntry struct {
      Codeword uint16 // 编码字
      Length   uint8  // 编码长度
      EkBits   uint8  // 每个样本的有效位指示
      UqOff    uint8  // Uq 偏移量
  }
  ```
- [ ] 从 OpenJPH 移植 table0.h（初始行）
- [ ] 从 OpenJPH 移植 table1.h（后续行）
- [ ] 实现表查找函数
  ```go
  func lookupVLC0(context, rho, eps uint8) VLCEntry
  func lookupVLC1(context, rho, eps uint8) VLCEntry
  ```

**参考**: OpenJPH `table0.h`, `table1.h`

**测试**:
- 验证表条目数量（2048 条/表）
- 测试典型查找（context=0, rho=15, eps=15）

---

#### Step 1.2: 上下文计算 [~30 行]
**文件**: `context.go` (新建)

**任务**:
- [ ] 实现 sigma 计算（邻居显著性）
  ```go
  func computeSigma(neighbors []bool) uint8
  ```
- [ ] 实现 context 计算
  ```go
  func computeContext(qx, qy int, sigMap [][]bool) uint8
  ```
- [ ] 实现 eps 计算（指数掩码）
  ```go
  func computeEps(eQ []int, maxE int) uint8
  ```

**参考**: OpenJPH `ojph_block_encoder.cpp` 行 600-650

**测试**:
- 测试边界条件（第一行、最后一行）
- 验证 context 范围（0-15）

---

#### Step 1.3: VLC 编码器增强 [~80 行]
**文件**: 修改 `vlc_encoder.go`

**任务**:
- [ ] 添加反向位写入支持（从缓冲区末尾写入）
  ```go
  func (v *VLCEncoder) WriteReverse(cwd uint32, len int) error
  ```
- [ ] 实现字节填充（> 0x8F 后只用 7 位）
- [ ] 实现 VLC 终止
  ```go
  func (v *VLCEncoder) Terminate() error
  ```
- [ ] 添加融合准备
  ```go
  func (v *VLCEncoder) GetLastByte() (byte, int) // 返回最后字节和已用位数
  ```

**参考**: OpenJPH `ojph_block_encoder.cpp` 行 358-446

**测试**:
- 测试反向写入
- 测试字节填充规则
- 验证与解码器的往返

---

### Phase 2: Quad Pair 处理 [优先级: 高]

#### Step 2.1: 样本预处理 [~60 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 实现样本值提取和归一化
  ```go
  func (h *HTEncoder) preprocessSample(val int32) (mag uint32, sign int, eQ int)
  ```
- [ ] 计算每个 quad 的 rho（显著性掩码）
- [ ] 计算每个样本的指数 e_q
- [ ] 找出 quad 的最大指数 max_e

**参考**: OpenJPH `ojph_block_encoder.cpp` 行 600-650

**公式**:
```
val_adjusted = val << (p - missing_msbs)  // p = 30 for 32-bit
mag = abs(val_adjusted) >> 1
e_q = 31 - leading_zeros(mag)
```

**测试**:
- 测试各种样本值（0, ±1, ±127, ±255）
- 验证指数计算

---

#### Step 2.2: Quad Pair 结构 [~40 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 扩展 QuadInfo 结构
  ```go
  type QuadInfo struct {
      Rho      uint8    // 显著性模式
      EQ       [4]int   // 每个样本的指数
      MaxE     int      // 最大指数
      Eps      uint8    // 指数掩码
      Samples  [4]int32 // 样本值
      SigCount int      // 显著样本数
  }
  ```
- [ ] 实现 quad pair 提取
  ```go
  func (h *HTEncoder) getQuadPair(qx, qy int) (QuadInfo, QuadInfo, bool)
  ```
- [ ] 处理边界条件（最后一列可能只有一个 quad）

**测试**:
- 测试 quad pair 提取
- 验证边界处理

---

#### Step 2.3: 初始行 vs 后续行 [~50 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 区分初始行（qy=0）和后续行
  ```go
  func (h *HTEncoder) encodeInitialRow() error
  func (h *HTEncoder) encodeSubsequentRows() error
  ```
- [ ] 初始行：使用 table0，context=0
- [ ] 后续行：使用 table1，计算 context

**参考**: OpenJPH 行 597-805（初始行），行 809-1014（后续行）

**测试**:
- 测试单行 codeblock
- 测试多行 codeblock

---

### Phase 3: 指数预测器和 UVLC [优先级: 中]

#### Step 3.1: Kappa 计算 [~40 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 实现 Kappa 计算（后续行）
  ```go
  func computeKappa(rho uint8, maxEPrev int) int
  ```
- [ ] 公式：
  ```
  if popcount(rho) > 1:
      kappa = max(1, maxEPrev)
  else:
      kappa = 1
  ```

**参考**: OpenJPH 行 875-876

**测试**:
- 测试单样本 quad（kappa=1）
- 测试多样本 quad

---

#### Step 3.2: Uq 和 u_q 计算 [~30 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 计算 Uq（基准指数）
  ```go
  Uq := max(maxE, kappa)
  ```
- [ ] 计算 u_q（需要编码的值）
  ```go
  uq := Uq - kappa
  ```
- [ ] 从 VLC 表提取 uqOff

**测试**:
- 验证 Uq 范围
- 测试 u_q 计算

---

#### Step 3.3: UVLC 编码集成 [~40 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 检查是否需要 UVLC 编码
  ```go
  if uq > 2 {
      // 使用现有的 UVLCEncoder
      h.uvlc.Encode(uq - 2)
  }
  ```
- [ ] 集成现有的 `uvlc_encoder.go`
- [ ] 确保 UVLC 输出到正确的流

**注意**: `uvlc_encoder.go` 已存在，只需集成

**测试**:
- 测试 u_q ≤ 2（不需要 UVLC）
- 测试 u_q > 2（需要 UVLC）

---

### Phase 4: MEL/VLC 段融合 [优先级: 中]

#### Step 4.1: 段终止 [~50 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 实现 MEL 终止
  ```go
  func (m *MELEncoder) Terminate() (lastByte byte, usedBits int)
  ```
- [ ] 实现 VLC 终止（已在 Step 1.3）
- [ ] 对齐到字节边界

**参考**: OpenJPH 行 420-446

**测试**:
- 测试各种终止场景
- 验证字节对齐

---

#### Step 4.2: 最后字节融合 [~60 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 实现融合逻辑
  ```go
  func fuseMELVLC(melLast, vlcLast byte, melBits, vlcBits int) (fused byte, canFuse bool)
  ```
- [ ] 检查融合条件：
  - MEL 和 VLC 的已用位不重叠
  - 融合结果 ≠ 0xFF
  - VLC 还有空间
- [ ] 融合成功：写入单个字节
- [ ] 融合失败：分别写入两个字节

**参考**: OpenJPH 行 420-446

**公式**:
```go
fused := melLast | vlcLast
melMask := (0xFF << melRemaining) & 0xFF
vlcMask := 0xFF >> (8 - vlcUsed)
safe := ((fused ^ melLast) & melMask) | ((fused ^ vlcLast) & vlcMask) == 0
canFuse := safe && fused != 0xFF && vlcSpace > 0
```

**测试**:
- 测试可融合场景
- 测试不可融合场景
- 验证 0xFF 回避

---

#### Step 4.3: Scup 编码 [~20 行]
**文件**: 修改 `encoder.go`

**任务**:
- [ ] 计算最终的 MEL/VLC 长度
- [ ] 编码 12 位 Scup
  ```go
  scup := melLen + vlcLen
  footer[0] = byte(scup & 0x0F)
  footer[1] = byte((scup >> 4) & 0xFF)
  ```
- [ ] 已实现，验证正确性

**测试**:
- 验证 Scup 解码
- 测试边界值（0, 2, 4079）

---

### Phase 5: 测试和验证 [优先级: 最高]

#### Step 5.1: 单元测试 [持续]

**文件**: `encoder_test.go` (新建/扩展)

**任务**:
- [ ] 测试每个组件独立功能
  - VLC 表查找
  - 上下文计算
  - Kappa 计算
  - 融合逻辑
- [ ] 测试边界条件
- [ ] 测试错误处理

**覆盖率目标**: >90%

---

#### Step 5.2: 往返测试 [关键]

**文件**: 修改 `htj2k_roundtrip_test.go`

**任务**:
- [ ] 小 codeblock（4x4, 8x8, 16x16）
- [ ] 中等 codeblock（32x32, 64x64）
- [ ] 大 codeblock（128x128+）
- [ ] 各种数据模式：
  - 全零
  - 单个非零
  - 稀疏数据
  - 密集数据
  - 梯度模式

**目标**:
- 无损编码：0 误差
- 有损编码：PSNR > 40dB

---

#### Step 5.3: 与 OpenJPH 对比 [可选]

**任务**:
- [ ] 对比编码输出
- [ ] 对比压缩率
- [ ] 对比编码时间

**工具**:
- 使用 OpenJPH 编码同样的数据
- 比较 codeblock 字节

---

## 📊 工作量估算

| Phase | 步骤 | 代码量 | 时间估算 | 依赖 |
|-------|------|--------|----------|------|
| 1.1 | VLC 表结构 | ~50 行 | 0.5小时 | 无 |
| 1.2 | 上下文计算 | ~30 行 | 0.5小时 | 无 |
| 1.3 | VLC 编码器 | ~80 行 | 1小时 | 1.1, 1.2 |
| 2.1 | 样本预处理 | ~60 行 | 1小时 | 无 |
| 2.2 | Quad Pair | ~40 行 | 0.5小时 | 2.1 |
| 2.3 | 初始/后续行 | ~50 行 | 1小时 | 1.1-2.2 |
| 3.1 | Kappa 计算 | ~40 行 | 0.5小时 | 2.2 |
| 3.2 | Uq/u_q | ~30 行 | 0.5小时 | 3.1 |
| 3.3 | UVLC 集成 | ~40 行 | 0.5小时 | 3.2 |
| 4.1 | 段终止 | ~50 行 | 1小时 | 1.3 |
| 4.2 | 字节融合 | ~60 行 | 1.5小时 | 4.1 |
| 4.3 | Scup 编码 | ~20 行 | 0.5小时 | 4.2 |
| 5.1 | 单元测试 | ~200 行 | 2小时 | 各步骤 |
| 5.2 | 往返测试 | ~100 行 | 1小时 | 完整编码器 |
| **总计** | | **~850 行** | **~12小时** | |

---

## 🎬 里程碑

### Milestone 1: 基本编码能力 ✨
**目标**: 完成 Phase 1-2，能编码简单的 codeblock

**成功标准**:
- 4x4 单 codeblock 往返测试通过
- VLC 表查找正确
- 基本的 quad 处理工作

**验收测试**: `TestBasicEncoding4x4`

---

### Milestone 2: 标准兼容 🎯
**目标**: 完成 Phase 3，符合 ITU-T.814

**成功标准**:
- 指数预测器工作
- UVLC 编码正确
- 多行 codeblock 工作

**验收测试**: `TestStandardCompliance`

---

### Milestone 3: 完整实现 🏆
**目标**: 完成 Phase 4-5，生产就绪

**成功标准**:
- 所有大小的 codeblock 测试通过
- MEL/VLC 融合正确
- 无损模式 0 误差
- 代码覆盖率 >90%

**验收测试**: `TestHTJ2KLosslessRoundTrip` (所有测试)

---

## 📌 注意事项

### 关键难点

1. **VLC 表移植**
   - OpenJPH 表是 C++ 宏生成的
   - 需要手动转换为 Go 数组
   - 注意字节序和打包

2. **反向位写入**
   - VLC 从缓冲区末尾向前写
   - 需要仔细处理边界
   - 字节填充规则复杂

3. **MEL/VLC 融合**
   - 融合条件微妙
   - 需要精确匹配 OpenJPH 行为
   - 测试覆盖所有场景

### 调试策略

1. **逐步验证**
   - 每个步骤添加单元测试
   - 对比 OpenJPH 中间结果
   - 使用十六进制 dump 对比输出

2. **参考实现**
   - 保持 OpenJPH 源码在手边
   - 对照行号确认逻辑
   - 复制关键注释

3. **增量测试**
   - 从最简单的 case 开始
   - 逐步增加复杂度
   - 每个里程碑独立验证

---

## 🔗 参考资料

- **标准**: ITU-T T.814 / ISO/IEC 15444-15:2019
- **源码**: OpenJPH `ojph_block_encoder.cpp`
- **表定义**: OpenJPH `table0.h`, `table1.h`
- **测试**: OpenJPH test suite

---

## 📝 更新日志

| 日期 | 里程碑 | 备注 |
|------|--------|------|
| 2026-01-28 | 计划创建 | 初始实现计划 |
| | | |

---

## ✅ 下一步行动

1. **立即开始**: Phase 1.1 - VLC 表数据结构
2. **准备工作**: 提取 OpenJPH table0.h 和 table1.h
3. **环境设置**: 设置测试框架

**预计完成时间**: 3-4 个工作日（分多个会话）
