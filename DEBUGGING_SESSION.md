# JPEG 2000 T1 Encoder/Decoder Debugging Session

## 会话日期
2025-11-13

## 发现和修复的关键Bug

### Bug #1: Cleanup Pass 中已显著系数的位平面值未解码

**问题描述：**
解码器在 Cleanup Pass 普通路径中跳过了已显著的系数（`if (flags & T1_SIG) != 0 { continue }`），但编码器仍然为这些系数编码位平面值。这导致 MQ 流失去同步。

**根本原因：**
编码器和解码器对已显著系数的处理不一致：
- 编码器：总是编码位平面值（`isSig = (absVal >> bitplane) & 1`），无论是否已显著
- 解码器：跳过已显著的系数

**修复方案：**
解码器必须匹配编码器行为：
1. 总是解码位平面值，即使系数已显著
2. 如果 bit=1 且 alreadySig：更新系数值但不解码符号位
3. 如果 bit=1 且 !alreadySig：解码符号位，首次变得显著

**代码位置：**
- `jpeg2000/t1/decoder.go` CP normal path (around line 531-583)

**测试结果：**
- ✅ TestMinimalSingleCoeff: 全部通过
- ✅ 单系数编码/解码同步正确

### Bug #2: VISIT 标志过早清除

**问题描述：**
SPP 和 MRP 在通道结束时清除 VISIT 标志，导致在 SPP 中变得显著的系数在同一位平面的 MRP 中被错误地处理，造成值加倍。

**根本原因：**
VISIT 标志的生命周期理解错误：
- 错误：每个通道结束后清除
- 正确：整个位平面的三个通道中保持，只在位平面结束后清除

**行为分析：**
```
BP 0 SPP: 系数变得显著，值=1，设置 T1_VISIT
          然后立即清除 T1_VISIT ✗
BP 0 MRP: 检查 (SIG && !VISIT)，条件满足，处理系数
          读取 refBit=1，值变成 2 ✗
BP 0 CP:  alreadySig=true，解码 bit=1，值变成 3 ✗
最终：1 → 3（错误）
```

**正确行为：**
```
BP 0 SPP: 系数变得显著，值=1，设置 T1_VISIT
          保持 T1_VISIT ✓
BP 0 MRP: 检查 (SIG && !VISIT)，VISIT=true，跳过 ✓
BP 0 CP:  已访问，跳过 ✓
最终：1 → 1（正确）
```

**修复方案：**
注释掉 SPP 和 MRP 中的 VISIT 清除代码：
- SPP: line 300
- MRP: line 364

**代码位置：**
- `jpeg2000/t1/decoder.go` lines 300, 364

**测试结果：**
- ✅ 单系数在 SPP 中变得显著：1 → 1（正确）
- ⚠️ 多系数模式仍有问题

## 当前状态

### 通过的测试
- ✅ TestMinimalSingleCoeff (所有单系数测试)
- ✅ TestT1EncodeDecodeRoundTrip/Single_non-zero
- ✅ TestT1EncodeDecodeRoundTrip/All_zeros

### 失败的测试
- ❌ TestT1EncodeDecodeRoundTrip/Simple_pattern
- ❌ TestT1EncodeDecodeRoundTrip/Negative_values
- ❌ JPEG2000 完整测试（梯度数据等）

### 剩余问题

#### 问题 A: 多系数模式错误
**测试用例：** `[1, 0, 0, 2]` (2x2)
**期望输出：** `[1, 0, 0, 2]`
**实际输出：** `[1, -1, 0, 3]`

**分析：**
- 位置 0（值 1）：正确 ✓
- 位置 1（值 0）：变成 -1 ✗
- 位置 3（值 2）：变成 3 ✗

可能原因：
1. 邻居标志更新错误
2. 上下文计算错误
3. RL 路径问题
4. 其他系数影响第一个系数的处理

## 调试工具和方法

### 创建的测试文件
1. `minimal_test.go` - 单系数测试（1x1）
2. `debug_double_test.go` - 值加倍测试（2x2，单个值）
3. `debug_diagonal_test.go` - 对角线模式测试（2x2，两个值）

### 调试输出
- 编码器：MRP 和 CP 的详细日志
- 解码器：SPP、MRP、CP 的详细日志
- 每个位平面开始时的系数值
- VISIT 标志状态

### 关键调试技巧
1. **追踪单个系数的完整生命周期** - 从编码到解码
2. **检查 MQ 编码/解码对称性** - 确保每个 Encode 对应一个 Decode
3. **验证标志状态** - T1_SIG, T1_VISIT, T1_SIGN
4. **使用最小测试用例** - 1x1, 2x2 而不是复杂的模式

## 下一步行动计划

### 立即行动（高优先级）
1. **调查多系数交互问题**
   - 为什么位置 1 变成 -1
   - 为什么位置 3 从 2 变成 3
   - 检查邻居标志和上下文计算

2. **清理调试输出**
   - 移除或条件化 fmt.Printf 语句
   - 保留关键日志用于未来调试

### 中期行动
3. **完善 RL 路径**
   - 确保 RL 路径的 alreadySig 逻辑正确
   - 测试 4x4 及以上的块

4. **验证编码器对称性**
   - 编码器是否也需要修复 VISIT 标志清除？
   - 检查编码器的 SPP 和 MRP

### 长期行动
5. **完整测试套件**
   - 修复所有 T1 测试
   - 修复所有 JPEG2000 集成测试
   - 处理梯度数据测试

6. **性能优化**
   - 移除调试代码
   - 优化热点路径

## 技术笔记

### JPEG 2000 EBCOT Tier-1 编码架构

**三个通道（每个位平面）：**
1. **SPP (Significance Propagation Pass)**
   - 处理有显著邻居但自己未显著的系数
   - 系数首次变得显著时：编码符号位（带预测）
   - 设置 T1_SIG | T1_VISIT

2. **MRP (Magnitude Refinement Pass)**
   - 处理已显著的系数（从之前的位平面）
   - 跳过条件：`(!T1_SIG || T1_VISIT)`
   - 编码精细化位

3. **CP (Cleanup Pass)**
   - 处理 SPP 未处理的所有系数
   - 有 RL (Run-Length) 和普通两种路径
   - 系数首次变得显著时：编码符号位（均匀上下文）
   - 已显著系数：只编码位平面值，不编码符号位

### VISIT 标志语义

**用途：** 标记系数在当前位平面已被处理过

**生命周期：**
- 设置：在 SPP 或 CP 中首次变得显著时
- 保持：整个位平面的三个通道中
- 清除：位平面结束时（CP 后）

**为什么需要：**
防止在同一位平面的不同通道中重复处理系数。

### MQ 编解码对称性

关键规则：**编码器的每个 Encode() 调用必须对应解码器的一个 Decode() 调用**

如果不对称会导致：
- 解码错误的位值
- 上下文状态不同步
- 后续所有解码都错误

## 参考资料

- ISO/IEC 15444-1:2019 - JPEG 2000 标准
- OpenJPEG 实现（参考）
- `KNOWN_ISSUES.md` - 已知问题文档
- `CURRENT_STATUS.md` - 当前状态文档
