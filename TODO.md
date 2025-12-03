# TODO.md

开发进度跟踪和任务列表

## 项目状态

**go-dicom-codec** 是一个为 DICOM 医学影像提供图像编解码功能的 Go 库，支持 JPEG、JPEG-LS 和 JPEG 2000 系列编解码器。

本库专注于编解码器的实现，不涉及 DICOM 封装、分片、元数据等处理（这些由外部 DICOM 库负责）。

### 当前状态
- ✅ 项目架构重构完成 (2025-11-10)
- ✅ JPEG Baseline 编解码器 (1.2.840.10008.1.2.4.50)
- ✅ JPEG Lossless 编解码器 (1.2.840.10008.1.2.4.57) - 所有 7 个预测器
- ✅ JPEG Lossless SV1 编解码器 (1.2.840.10008.1.2.4.70)
- ✅ JPEG Extended (12-bit) 编解码器 (1.2.840.10008.1.2.4.51)
- ✅ JPEG-LS Lossless 编解码器 (1.2.840.10008.1.2.4.80)
- ✅ JPEG-LS Near-Lossless 编解码器 (1.2.840.10008.1.2.4.81) - **完成!** 🎉
  - ✅ 核心功能完成 (所有 NEAR 值 0-255 完全工作)
  - ✅ Codec 接口实现和注册
  - ✅ NEAR≥7 大图像问题已修复 (偏差校正在 NEAR>0 时禁用)
- ✅ JPEG 2000 Lossless 编解码器 (1.2.840.10008.1.2.4.90) - **完成!** 🎉
- ✅ JPEG 2000 Lossy 编解码器 (1.2.840.10008.1.2.4.91) - **完成!** 🎉

---

## 第一阶段: 核心架构 ✅ (已完成)

### 1.1 项目重构 ✅
- [x] 重命名项目: go-jpeg → go-dicom-codec
- [x] 更新模块路径: github.com/cocosip/go-dicom-codec
- [x] 创建核心接口包 `codec/`
  - [x] `codec/codec.go` - 编解码器接口定义
  - [x] `codec/registry.go` - 编解码器注册和管理
  - [x] `codec/errors.go` - 通用错误定义
- [x] 重组目录结构
  - [x] `jpeg/` - JPEG 系列编解码器
  - [x] `jpeg/common/` - JPEG 公共工具
  - [x] `jpeg/baseline/` - Baseline 编解码器
  - [x] `jpeg/lossless14sv1/` - Lossless SV1 编解码器
- [x] 更新所有导入路径
- [x] 更新文档 (README.md, CLAUDE.md, TODO.md)

### 1.2 核心接口设计 ✅
- [x] `Codec` 接口: Encode(), Decode(), UID(), Name()
- [x] `EncodeParams` 结构: 统一的编码参数
- [x] `DecodeResult` 结构: 统一的解码结果
- [x] `Options` 接口: 编解码器特定选项
- [x] 注册表模式: 支持按名称或 UID 查找编解码器

---

## 第二阶段: JPEG 系列

### 2.1 JPEG Baseline (1.2.840.10008.1.2.4.50) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] 编码器: 支持 8-bit 灰度和 RGB 图像
- [x] 解码器: 完整的 JPEG Baseline 解析
- [x] 测试覆盖:
  - [x] 单元测试 (编码/解码)
  - [x] 往返测试
  - [x] 与 Go 标准库互操作性验证
  - [x] 性能基准测试
- [x] 性能: 编码 ~1.17ms, 解码 ~2.97ms (512x512 灰度)

### 2.2 JPEG Lossless SV1 (1.2.840.10008.1.2.4.70) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] 编码器: 预测器 1 (左像素) 实现
- [x] 解码器: SOF3 解析和差分解码
- [x] 支持 2-16 bit 精度
- [x] 测试覆盖:
  - [x] 8-bit 灰度和 RGB 测试
  - [x] 完美重建验证 (0 误差)
  - [x] 性能基准测试
- [x] 性能: 编码 ~3.65ms, 解码 ~40.2ms (512x512 灰度)

### 2.3 JPEG Extended (1.2.840.10008.1.2.4.51) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] **UID**: 1.2.840.10008.1.2.4.51
- [x] **功能**: 8/12-bit 有损压缩, SOF1 标记
- [x] 编码器: 基于 Go 标准库的简化实现
  - [x] 8-bit 直接编码
  - [x] 12-bit 通过缩放实现
  - [x] SOF0 → SOF1 标记转换
- [x] 解码器: 自动检测和处理
  - [x] 检测精度字段
  - [x] SOF1 → SOF0 临时转换
  - [x] 8-bit 结果缩放回 12-bit
- [x] 测试覆盖:
  - [x] 8-bit 编解码 (压缩比 2.37x, 平均误差 1.69)
  - [x] 12-bit 编解码 (压缩比 12.74x, 平均误差 11.88)
  - [x] RGB 图像 (压缩比 4.23x, 最大误差 8)
  - [x] 参数验证和质量级别
- [x] 创建 `Codec` 实现并注册
- [x] 测试状态: ✅ 100% 通过 (11/11)

### 2.4 JPEG Lossless (1.2.840.10008.1.2.4.57) ✅ (已完成)
- [x] **UID**: 1.2.840.10008.1.2.4.57
- [x] **功能**: 通用无损压缩, 支持 7 种预测器
- [x] 实现 `jpeg/lossless/encoder.go`
  - [x] 实现 7 种预测器 (1-7)
  - [x] 自动选择最优预测器
  - [x] 差分编码
- [x] 实现 `jpeg/lossless/decoder.go`
  - [x] 解析预测器选择值 (Ss)
  - [x] 支持所有预测器模式
  - [x] 差分解码
  - [x] 修复字节填充双重处理问题
- [x] 测试
  - [x] 所有预测器测试 (7/7 通过)
  - [x] 完美重建验证 (0 错误)
  - [x] 预测器性能对比
  - [x] RGB 图像测试
- [x] 创建 `Codec` 实现并注册
- [x] 外部接口完整集成
- [x] 性能: 编码 ~12.5ms, 解码 ~8.3ms (512x512 灰度, 预测器 1)

---

## 第三阶段: JPEG-LS 系列 ⏳

### 3.1 JPEG-LS Lossless (1.2.840.10008.1.2.4.80) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] **UID**: 1.2.840.10008.1.2.4.80
- [x] **功能**: JPEG-LS 无损压缩 (LOCO-I 算法)
- [x] **技术**: 上下文自适应, Golomb-Rice 编码, MED 预测
- [x] 创建目录 `jpegls/lossless/`
- [x] 实现编码器
  - [x] 上下文建模 (729 contexts)
  - [x] Golomb-Rice 编码
  - [x] MED 预测算法
  - [x] Run mode 优化
- [x] 实现解码器
  - [x] Golomb-Rice 解码
  - [x] 上下文重建
  - [x] 预测值计算
- [x] 测试
  - [x] 完美重建验证 (0 错误)
  - [x] 压缩率测试 (4.17x grayscale, 2.51x RGB)
  - [x] 12-bit 支持 (2.94x)
- [x] 创建 `Codec` 实现并注册
- [x] 测试状态: ✅ 100% 通过 (11/11)

### 3.2 JPEG-LS Near-Lossless (1.2.840.10008.1.2.4.81) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] **UID**: 1.2.840.10008.1.2.4.81
- [x] **功能**: JPEG-LS 近无损压缩 (可配置误差界限)
- [x] 创建目录 `jpegls/nearlossless/`
- [x] 实现 NEAR 参数支持
  - [x] 误差量化/去量化
  - [x] 量化预测值
  - [x] LSE/SOS marker 中的 NEAR 参数
- [x] 实现编码器
  - [x] 量化邻居和误差
  - [x] 重建样本存储
- [x] 实现解码器
  - [x] 解析 NEAR 参数
  - [x] 量化误差解码
- [x] 测试覆盖
  - [x] NEAR=0 (无损): ✅ 完美重建 (0 错误)
  - [x] NEAR=1-10: ✅ 所有误差界限满足
  - [x] NEAR=3: ✅ 最大误差≤3, 压缩率 5.79x
  - [x] NEAR=7: ✅ 最大误差≤7, 压缩率 4.56x (已修复!)
  - [x] RGB NEAR=3: ✅ 最大误差≤3
  - [x] 压缩率对比测试: ✅ 通过
  - [x] 参数验证: ✅ 通过
  - [x] Codec 接口测试: ✅ 全部通过
- [x] 创建 `Codec` 实现并注册
- [x] 测试状态: ✅ 100% 通过 (17/17)
- [x] **修复 NEAR≥7 大图像支持** (2025-11-10)
  - [x] 问题诊断: 偏差校正在 near-lossless 模式下导致编解码器状态发散
  - [x] 解决方案: 在 NEAR>0 时禁用偏差校正
  - [x] 额外优化: 上下文计算基于原始邻居值而非量化值
  - [x] 验证测试: 所有 NEAR 值 (0-255) 在所有图像尺寸下工作正常

---

## 第四阶段: JPEG 2000 系列 ✅ (基础功能已完成)

### 4.1 技术调研 ✅
- [x] 选择实现策略: 纯 Go 实现
- [x] 评估可行性: 确认纯 Go 实现方案

### 4.2 JPEG 2000 Lossless (1.2.840.10008.1.2.4.90) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] **UID**: 1.2.840.10008.1.2.4.90
- [x] **功能**: JPEG 2000 Part 1 无损压缩
- [x] **技术**: 5/3 可逆小波变换, EBCOT, MQ 算术编码
- [x] 创建目录 `jpeg2000/lossless/`
- [x] 实现编码器
  - [x] 5/3 可逆小波变换 (多级)
  - [x] MQ 算术编码器 (47状态)
  - [x] EBCOT Tier-1 编码器 (位平面编码)
  - [x] EBCOT Tier-2 数据包编码
  - [x] 码流生成器 (SOC, SIZ, COD, QCD, SOT, SOD, EOC)
- [x] 实现解码器
  - [x] 码流解析器
  - [x] MQ 算术解码器
  - [x] EBCOT Tier-1/2 解码
  - [x] 5/3 逆小波变换
- [x] 测试
  - [x] 完美重建验证 (0 像素误差)
  - [x] 多种图像尺寸 (64×64 到 1024×1024)
  - [x] 多级小波分解 (0-6 级)
  - [x] RGB 图像支持
- [x] 创建 `Codec` 实现并注册
- [x] 压缩比: 5.5:1 到 6.8:1
- [x] 测试状态: ✅ 100% 通过

### 4.3 JPEG 2000 Lossy (1.2.840.10008.1.2.4.91) ✅ (已完成)
- [x] **实现状态**: 完全实现并测试通过
- [x] **UID**: 1.2.840.10008.1.2.4.91
- [x] **功能**: JPEG 2000 有损压缩
- [x] **技术**: 9/7 不可逆小波变换
- [x] 创建目录 `jpeg2000/lossy/`
- [x] 实现 9/7 小波变换
  - [x] 前向变换 (编码)
  - [x] 逆向变换 (解码)
  - [x] CDF 9/7 滤波器系数
  - [x] 浮点运算支持
- [x] 修改编码器支持 lossy 模式
  - [x] 在 `encoder.go` 中添加 9/7 小波支持
  - [x] Float64 <-> Int32 转换
  - [x] 修复 DC level shift bug
- [x] 修改解码器支持 lossy 模式
  - [x] 在 `tile_decoder.go` 中添加 9/7 小波支持
  - [x] 自动检测变换类型
- [x] 测试
  - [x] 多种图像尺寸 (16×16, 64×64, RGB)
  - [x] 误差验证 (64×64+: 1-2 像素最大误差)
  - [x] 压缩比验证 (~3:1)
- [x] 创建 `Codec` 实现并注册
- [x] 测试状态: ✅ 100% 通过
- [x] 已知限制: 当前使用最小量化 (float→int 舍入)

### 4.3.1 JPEG 2000 Lossy 增强 ✅ (已完成，2025-12-03)
- [x] 质量参数/量化控制
  - [x] 质量级别 (1-100)
  - [x] 可配置分解层数（小图自动裁剪）
  - [x] 全局量化步长倍率 (QuantStepScale)
  - [x] 自定义子带量化步长 (SubbandSteps，需 3*numLevels+1)
- [x] 码率控制（启发式）
  - [x] TargetRatio 目标压缩比 -> 二分搜索质量逼近（非精确 R-D）
- [x] 多质量层支持 (NumLayers)
- [x] 速率失真优化 / 精确码率控制 ✅ (基础实现完成)
  - [x] **T1 精确失真计算** ✅ (2025-12-03)
    - [x] 实现 `calculateDistortion()` - 基于真实重建误差（SSE）
    - [x] 替换旧的估算公式 `rdDistortionDelta()`
    - [x] PassData 记录每个 coding pass 的 Rate/Distortion
    - [x] 考虑 bit-plane 编码特性（已编码/未编码比特）
  - [x] **T2 基于 slope 的 R-D 分配** ✅ (已实现)
    - [x] `AllocateLayersRateDistortionPasses()` - PCRD 式分配
    - [x] 按 ΔD/ΔR slope 排序 contributions
    - [x] 基于 TargetRatio 的 budget 约束
    - [x] 多层渐进式分配策略
  - [x] **TargetRatio 接入 R-D 分配** ✅ (已实现)
    - [x] `applyRateDistortion()` 在编码器中集成
    - [x] 自动触发条件: NumLayers>1 或 TargetRatio>0
    - [x] 基于 PassData 的精确分配
  - [x] **完整测试套件** ✅ (2025-12-03)
    - [x] `rate_distortion_test.go` - R-D 分配算法测试（8个测试）
    - [x] `progressive_decode_test.go` - 多层渐进解码测试（6个测试）
    - [x] `target_ratio_test.go` - TargetRatio 精度测试（7个测试）
    - [x] `distortion_accuracy_test.go` - 失真精度测试（6个测试）
  - [x] **高级 PCRD 优化** ✅ (任务5 已完成)
    - [x] Lambda 二分搜索算法（最优截断点）
    - [x] 基于 Lagrange 乘数的全局 R-D 优化（按 slope≥λ 截断）
    - [x] 改进的多层预算分配策略（EXPONENTIAL/EQUAL_RATE/EQUAL_QUALITY/ADAPTIVE）
    - [x] 详见下方 **4.3.2 节**

### 4.3.2 JPEG 2000 高级 PCRD 优化 ✅ (任务5 - 已完成，2025-12-03)

**目标**: 实现完整的 Post-Compression Rate-Distortion (PCRD) 优化算法，基于 ISO/IEC 15444-1 Annex J.

**当前状态**:
- ✅ 基础 R-D 分配已实现（基于 slope 排序）
- ✅ 精确失真计算已实现（SSE 重建误差）
- ✅ 完整 PCRD-opt λ 搜索与截断实现（跨 code-block 全局优化）
- ✅ 多层预算策略实现（EXPONENTIAL/EQUAL_RATE/EQUAL_QUALITY/ADAPTIVE）

**已实现功能**:

#### 5.1 Lambda 二分搜索算法
- [x] **目标**: 找到最优的 Lagrange 乘数 λ，使得码率约束得到满足
- [x] **理论基础**:
  - 最小化: `J = D + λ*R`
  - 对于给定的 λ，选择所有 slope ≥ λ 的 passes
  - 通过二分搜索 λ，找到满足目标码率的最优截断点
- [x] **实现要点**:
  - `FindOptimalLambda(passes, targetRate)`
  - Lambda 范围: [0, max_slope]
  - 收敛条件: |actual_rate - target_rate| < tolerance
  - 返回: 最优 λ 值和对应的 pass 选择
- [x] **参考**: ISO/IEC 15444-1:2019 Annex J.2

#### 5.2 基于 Lambda 的全局 R-D 优化
- [x] **Slope 计算改进**:
  - `slope = ΔD / ΔR`（每个 pass 的增量）
  - 考虑跨 code-block 的全局最优性
  - 处理 slope 相等的情况（稳定排序）
- [x] **截断策略**:
  - `TruncateAtLambda(passes, lambda)` - 基于 λ 截断
  - 每个 code-block 独立截断
  - 保证单调性（layer[i] ≥ layer[i-1]）
- [x] **质量层分配**:
  - `AllocateLayersWithLambda(passes, numLayers, targetRates)`
  - 为每一层计算独立的 λ 值
  - 层间 λ 单调递减（λ[0] > λ[1] > ... > λ[n-1]）

#### 5.3 改进的多层预算分配策略
- [x] **策略**:
  - **等质量增量**（EQUAL_QUALITY）
  - **等码率增量**（EQUAL_RATE）
  - **自适应分配**（ADAPTIVE）
  - **指数分布**（EXPONENTIAL）
- [x] **实现**:
  - `ComputeLayerBudgets(totalBudget, numLayers, strategy)`
  - 返回每层累计目标码率数组

#### 5.4 集成到编码器
- [x] **`applyRateDistortion()` 集成**:
  - 开关控制完整 PCRD 与简化分配的切换
  - 性能开关（Lambda 搜索可配置容差）
- [x] **EncodeParams 扩展**:
  - `UsePCRDOpt bool`
  - `LayerBudgetStrategy string`
  - `LambdaTolerance float64`
- [x] **性能考虑**:
  - 二分迭代控制与增量缓存
  - 避免重复失真计算

#### 5.5 测试和验证
- [x] **单元测试**:
  - `TestFindOptimalLambda`
  - `TestTruncateAtLambda`
  - `TestLayerBudgetStrategies`
- [x] **对比测试**:
  - Simple vs PCRD 压缩效率对比（基础验证）
  - 码率精度（±5% 目标）
- [x] **性能测试**:
  - 搜索迭代与编码时间开销（基础验证）

#### 5.6 参考实现
- [x] **OpenJPEG 参考**:
  - `openjpeg/src/lib/openjp2/t2.c` - PCRD-opt 实现
  - `opj_t2_encode_packets_pcrd()` 函数
  - Lambda 搜索算法
- [x] **Kakadu 论文**:
  - D. Taubman, "High Performance Scalable Image Compression"
  - PCRD-opt 算法描述

**预期收益**:
- 📈 **码率精度**: 低压缩比（3–5:1）误差≤5%，中等压缩比（≤8:1）误差≤10%，高压缩比需进一步优化
- 📈 **压缩效率**: 相同码率下 PSNR 提升 0.5-1.5 dB
- 📈 **层间质量**: 更平滑的渐进式质量提升
- ⚠️ **性能代价**: 编码时间增加 5-10%

**实现优先级**:
1. 高优先级: Lambda 二分搜索（核心算法）
2. 中优先级: 全局 R-D 优化（质量提升）
3. 低优先级: 高级预算策略（边际收益）

### 4.4 JPEG 2000 Multi-component (1.2.840.10008.1.2.4.92/93) ⏳
- [ ] **UIDs**: 1.2.840.10008.1.2.4.92 (Lossless), .93 (Lossy)
- [ ] **功能**: JPEG 2000 Part 2 多分量支持
- [ ] 支持多光谱图像
- [ ] 创建 `Codec` 实现并注册

### 4.5 JPEG 2000 High-Throughput (HTJ2K) (1.2.840.10008.1.2.4.201/202/203) 🔬
- [x] **UIDs**:
  - 1.2.840.10008.1.2.4.201 (HTJ2K Lossless)
  - 1.2.840.10008.1.2.4.202 (HTJ2K RPCL Lossless)
  - 1.2.840.10008.1.2.4.203 (HTJ2K Lossy/Lossless)
- [x] **技术调研**: 基于 ISO/IEC 15444-15:2019 规范
- [x] **实现状态**: 核心组件完成 (2025-12-03)
  - [x] MEL 编码器/解码器（完全符合规范）
  - [x] MagSgn 编码器/解码器
  - [x] VLC 表格（OpenJPEG 803条目）✅
  - [x] VLC 解码器（上下文感知）✅
  - [x] 上下文计算（邻域显著性）✅
  - [x] 集成 HTJ2K 块解码器 ✅
  - [x] 包重命名: t1ht → htj2k ✅
  - [x] **U-VLC 解码（Clause 7.3.6）✅** (2025-12-03)
    - [x] decodeUPrefix - 前缀解码 (u_pfx ∈ {1,2,3,5})
    - [x] decodeUSuffix - 后缀解码 (1-bit 或 5-bit)
    - [x] decodeUExtension - 扩展解码 (4-bit for u_sfx≥28)
    - [x] 公式 (3): u = u_pfx + u_sfx + 4*u_ext
    - [x] 公式 (4): u = 2 + u_pfx + u_sfx + 4*u_ext (初始行对)
    - [x] 第二quad简化解码 (uq1>2时)
    - [x] 全部15个U-VLC测试通过
  - [x] **指数预测器计算（Clause 7.3.7）✅** (2025-12-03)
    - [x] MagnitudeExponent(μn) - 幅度指数计算
    - [x] QuadMaxExponent - Quad最大指数
    - [x] ComputePredictor(Kq) - 公式 (5): Kq = max(E'qL, E'qT) - γq
    - [x] ComputeExponentBound(Uq) - Uq = Kq + uq
    - [x] 首行特殊处理: Kq = 1
    - [x] Gamma计算: γq = 1 if |significant samples| > 1
    - [x] 全部9个预测器测试通过
  - [x] **Quad-pair交错解码（Clause 7.3.4）✅** (2025-12-03)
    - [x] Quad-pair概念实现 (q1=2g, q2=2g+1)
    - [x] 初始行对（initial line-pair）特殊处理
    - [x] 三种U-VLC解码模式:
      - [x] 标准解码: 公式(3) - 非初始行对或单个ulf=1
      - [x] 初始对解码: 公式(4) - 初始行对且两个ulf都=1
      - [x] 简化解码: ubit+1 - 当uq1>2时第二quad
    - [x] CxtVLC和U-VLC正确交错
    - [x] 奇数宽度处理 (最后一对只有第一quad)
    - [x] DecodeQuadPair - 单对解码
    - [x] DecodeAllQuadPairs - 批量解码
    - [x] 全部13个测试通过 (覆盖所有特殊情况)
  - [x] **VLC编码器实现（Annex F.3/F.4）✅** (2025-12-03)
    - [x] **U-VLC编码器** (`uvlc_encoder.go`)
      - [x] EncodeUVLC - Table 3完整实现
      - [x] 前缀编码: u_pfx ∈ {1,2,3,5}
      - [x] 后缀编码: 0/1/5-bit (根据u_pfx)
      - [x] 扩展编码: 4-bit (当u_sfx≥28)
      - [x] EncodeUVLCInitialPair - 公式(4)支持
      - [x] EncodePrefixBits - 前缀比特模式
      - [x] 全部14个测试通过 (包括往返测试)
    - [x] **上下文感知VLC编码器** (`vlc_encoder_new.go`)
      - [x] 基于OpenJPEG VLC表的反向查找
      - [x] EncodeCxtVLC - 上下文、rho、ulf、EMB编码
      - [x] emitVLCBits - 比特打包与bit-stuffing
      - [x] EncodeQuadPair - Quad-pair交错编码
      - [x] 初始/非初始行对编码表分离
      - [x] 字节流反转（符合规范要求）
    - [x] BitStreamWriter接口 - U-VLC和VLC集成
- [x] 创建 `Codec` 实现并注册
- [x] 文档说明实验性状态
- [x] **当前进度**: 核心编解码组件实现完成，72+测试通过
- [ ] **未来改进**:
  - [ ] 与参考实现对比测试（OpenJPEG/OpenJPH）
  - [ ] 性能优化（SIMD、查找表优化）
  - [ ] EMB模式计算优化

---

## 第五阶段: 性能优化

### 5.1 算法优化
- [ ] DCT/IDCT SIMD 优化
- [ ] 并行处理多个块
- [ ] 内存池减少分配
- [ ] Huffman 查找表优化

### 5.2 性能基准
- [ ] 建立性能基准数据库
- [ ] 跨平台性能测试
- [ ] 内存使用分析
- [ ] 大批量图像处理测试

---

## 第六阶段: 测试和验证

### 6.1 测试覆盖
- [ ] 单元测试覆盖率 > 85%
- [ ] 集成测试: 所有编解码器互操作
- [ ] 边界条件测试
  - [ ] 最小图像 (1x1, 8x8)
  - [ ] 大图像 (4096x4096+)
  - [ ] 非标准尺寸

### 6.2 互操作性测试
- [ ] 与 dcmtk 互操作
- [ ] 与 pydicom 互操作
- [ ] 与其他 DICOM 查看器兼容性

### 6.3 符合性测试
- [ ] DICOM 标准符合性验证
- [ ] 使用标准测试图像集
- [ ] 参考实现对比

---

## 第七阶段: 文档和示例

### 7.1 API 文档
- [ ] 完整 GoDoc 注释
- [ ] 每个公共函数的示例
- [ ] 参数和返回值详细说明

### 7.2 使用指南
- [x] README.md 基础版本
- [ ] 详细使用文档
- [ ] 各编解码器的最佳实践
- [ ] 性能调优指南

### 7.3 示例代码
- [ ] 创建 `examples/` 目录
  - [ ] JPEG Baseline 示例
  - [ ] JPEG Lossless 示例
  - [ ] JPEG-LS 示例
  - [ ] JPEG 2000 示例
  - [ ] 与 DICOM 库集成示例
  - [ ] 批量处理示例

---

## 第八阶段: 发布准备

### 8.1 版本管理
- [ ] 语义化版本 (v0.1.0 → v1.0.0)
- [ ] CHANGELOG.md
- [ ] 版本标签和发布

### 8.2 CI/CD
- [ ] GitHub Actions 配置
  - [ ] 自动化测试
  - [ ] 多平台构建
  - [ ] 性能基准追踪
  - [ ] 代码覆盖率报告

### 8.3 质量保证
- [ ] 代码审查
- [ ] 静态分析 (golangci-lint)
- [ ] 安全扫描
- [ ] LICENSE 文件

---

## 当前优先级

**已完成任务:**

1. ✅ 项目架构重构和核心接口设计
2. ✅ JPEG Baseline 编解码器适配到新架构
3. ✅ JPEG Lossless SV1 编解码器适配到新架构
4. ✅ JPEG Lossless (7 种预测器) 实现
5. ✅ JPEG Extended (12-bit) 实现
6. ✅ JPEG-LS Lossless 实现
7. ✅ JPEG-LS Near-Lossless 实现
8. ✅ JPEG 2000 Lossless 实现 - **完成!** 🎉
   - ✅ 纯 Go 实现 (5/3 可逆小波, MQ 编码, EBCOT)
   - ✅ 完美重建 (0 像素误差)
   - ✅ 压缩比 5.5:1 到 6.8:1
   - ✅ 所有测试通过
9. ✅ JPEG 2000 Lossy 实现 - **完成!** 🎉
   - ✅ 9/7 不可逆小波变换
   - ✅ 高质量压缩 (64×64+: 1-2 像素误差)
   - ✅ 压缩比 ~3:1
   - ✅ 所有测试通过

**下一步开发方向 (按优先级):**

10. ⏳ **推荐**: 开发方向选择
    - **选项 A**: JPEG 2000 增强功能
      - 质量参数支持 (可配置量化)
      - 多质量层
      - 多瓦片支持
      - ROI (感兴趣区域) 编码
    - **选项 B**: 优化现有编解码器性能
      - DCT/IDCT SIMD 优化
      - 小波变换 SIMD 优化
      - 并行处理多个块
      - 内存池减少分配
    - **选项 C**: 编写示例代码和使用文档
      - 创建 examples/ 目录
      - 详细使用指南
      - 最佳实践文档
      - 与 DICOM 库集成示例
    - **选项 D**: 完善测试覆盖率和互操作性测试
      - 提高测试覆盖率 >85%
      - 与 dcmtk/pydicom 互操作性测试
      - 边界条件测试
      - 大规模图像测试

---

## 技术说明

### JPEG 系列
- **标准**: ITU-T T.81 (JPEG), ITU-T T.87 (JPEG Lossless)
- **关键技术**: DCT, 霍夫曼编码, 量化, 预测编码
- **当前状态**: Baseline 和 Lossless SV1 已完成

### JPEG-LS 系列
- **标准**: ITU-T T.87 (ISO/IEC 14495)
- **关键技术**: 上下文自适应, Golomb-Rice 编码, 预测
- **优势**: 更好的无损压缩率, 低复杂度
- **当前状态**: 待实现

### JPEG 2000 系列
- **标准**: ISO/IEC 15444
- **关键技术**: 小波变换, EBCOT, 算术编码
- **优势**: 优秀的压缩率, 渐进式传输, ROI 编码
- **挑战**: 实现复杂度高, 考虑使用第三方库
- **当前状态**: 调研阶段

---

**最后更新**: 2025-12-03

**最新进展**:
- ✅ HTJ2K 实验性实现完成 (基于 ISO/IEC 15444-15:2019)
  - ✅ MEL 编码器/解码器（完全符合规范，13状态机）
  - ✅ MagSgn 编码器/解码器（完整实现）
  - ✅ HT block 框架和基础结构
  - ✅ 3个 HTJ2K codec 已注册 (UID: .201, .202, .203)
  - ⚠️ VLC 表使用简化实现（需要完整 Annex C 表格）
- ✅ 项目文档已更新，标注 HTJ2K 为实验性状态
- 🎯 下一步: 完善 HTJ2K VLC 表 或 开发其他功能

**备注**:
- 本库专注于编解码器实现，不处理 DICOM 封装、分片、元数据等
- Transfer Syntax UID 的定义和管理由外部 DICOM 库负责
- 编解码器可通过 UID 或名称访问
