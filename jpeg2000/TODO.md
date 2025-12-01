# JPEG 2000 ROI TODO

面向完整 ROI 能力（多 ROI、General Scaling、掩码/多边形、tile 级 RGN）的设计与实施清单。

## 现状
- 已有：单矩形 ROI 的 MaxShift，编码/解码双方需共享同一矩形；无 tile 级 RGN。
- 未有：多 ROI、掩码/多边形 ROI、General Scaling、标准 RGN 标记写入/解析、自动 ROI 元数据传递。

## 目标
- 覆盖 JPEG 2000 ROI 的 MaxShift 与 General Scaling，两种方式都支持。
- 支持多 ROI、多组件、多 tile，ROI 形状支持矩形/多边形/位图掩码。
- 标准化码流：正确写入/解析 RGN（tile-part header），必要时带兼容的私有元数据传递 ROI 几何。
- 向下兼容：默认行为与当前单矩形 MaxShift 保持一致。

## 进度更新（2025-12-01）
- [x] 完成 ROIConfig/ROIRegion MVP 设计与校验（多矩形 MaxShift，兼容 legacy ROI）。
- [x] 编解码管线接入多矩形 MaxShift：统一 RGN shift，ROIInfo 支持多矩形判定。
- [x] General Scaling（Srgn=1）基础支持：ROIConfig/编码端可写 Srgn=1，按组件的 shift 与矩形管理（当前仍使用统一 shift/矩形语义，后续需补齐真正的 General Scaling 权重与掩码管线）。
- [x] 新增 ROIConfig 校验与多矩形/General Scaling 编码/解码单元测试。
- [x] General Scaling 的真实缩放应用：Srgn=1 时对 ROI 系数执行 2^k 上移/反缩放（矩形/掩码路径）。
- [x] 掩码/多边形基础：支持 Polygon/MaskData 输入，生成全分辨率掩码并用于 ROI 判定，编码/解码均可透传掩码。
- [ ] 掩码多分辨率/tile 映射优化（缓存/按 block 步长下采样）、tile 级 RGN/私有元数据仍待实现。

## 下一步（短期）
- General Scaling：在编码端实现权重/bitplane 处理（Srgn=1），在解码端解析 Srgn=1 并正确反缩放。
- ROI 样式透传：解码端读取 RGN 的 Srgn（0/1），以便后续样式特定处理。
- 掩码/多边形：定义输入接口与 rasterization，支撑多分辨率/tile 映射。
- tile 级 RGN：支持按 tile/tile-part 写入/解析 RGN，保持向后兼容。

## 范围与不做
- 范围：编码器、解码器、ROI 参数与数据结构、码流标记、掩码生成/映射、测试验证。
- 不做：HTJ2K ROI 相关特性；非标准元数据方案仅作为可选私有扩展。

## 设计要点
- **ROI 定义模型**：统一结构（ID、组件集、tile 选择、形状=矩形/多边形/掩码、风格=MaxShift|GeneralScaling、shift/scale 参数）。支持提供图像坐标系的几何，内部生成多分辨率/tile 对齐的掩码。
- **参数与 API**：
  - 编码参数：新增 `ROIConfig`（[]ROI + 默认 shift/scale），支持矩形/多边形/外部掩码输入；允许选择 MaxShift 或 General Scaling。
  - 解码参数：允许传入 ROI 几何（兼容旧模式），或从码流/私有元数据读取；提供禁用 ROI 反缩放的开关以便对比测试。
  - 兼容：保持现有单矩形 MaxShift 的 API 可用。
- **掩码生成与映射**：
  - 矩形/多边形 rasterization -> 全分辨率掩码；支持直接提供位图掩码。
  - 针对每个 tile/resolution/coding-block 下采样并裁剪掩码；提供缓存避免重复计算。
  - 支持组件感知（每组件独立掩码）。
- **编码器（T1 前处理 + 码流标记）**：
  - MaxShift：在量化前对 ROI 系数做全量上移（2^k），按掩码应用；非 ROI 保持不变。
  - General Scaling：实现可配置的 k 值和权重（分层 bit-plane 上移/部分上移），符合 ISO/IEC 15444-1 B.10。
  - 在 tile-part header 中按组件写 RGN（支持多 ROI，多段 RGN；遵循 spec 顺序与作用域）。
  - 支持可选的私有 COM marker/box 携带 ROI 几何（便于解码端自动恢复掩码，需文档化为可选扩展）。
- **解码器**：
  - 解析 RGN，区分 MaxShift/General Scaling；应用与编码一致的掩码反缩放。
  - 当存在私有 ROI 元数据时自动重建掩码；否则使用外部传入的 ROI 几何。
  - 多 ROI 合并策略：按 ROI ID 或顺序覆盖/最大 shift。
- **错误处理与回退**：缺失 ROI 元数据时，保持非 ROI 解码结果；提供告警而非崩溃。

## 实施清单（建议顺序）
- [ ] 需求冻结：确定目标形状集（矩形/多边形/掩码）、支持的组件/ tile 粒度、默认 shift/scale。
- [ ] 数据结构与参数：
  - [ ] 设计 `ROI`/`ROIConfig` 结构与编码/解码参数签名，补充向后兼容路径。
  - [ ] 定义 ROI 组合/覆盖规则及默认 shift/scale。
- [ ] 掩码管线：
  - [ ] 矩形/多边形 rasterization，外部掩码导入。
  - [ ] 多分辨率/tile/block 映射与缓存。
  - [ ] 组件感知与大图内存占用控制。
- [ ] 编码器侧：
  - [ ] MaxShift 多 ROI 支持；掩码驱动的系数上移。
  - [ ] General Scaling 编码路径（含可配置 shift/scale）。
  - [ ] RGN 写入（tile-part header，支持多段、多组件）。
  - [ ] 可选 COM marker/box 写入 ROI 几何（私有扩展）。
- [ ] 解码器侧：
  - [ ] RGN 解析与 ROI 样式区分；多 ROI 合并。
  - [ ] ROI 反缩放（MaxShift & General Scaling）掩码应用。
  - [ ] 私有 ROI 元数据解析与掩码重建。
- [ ] 测试与验证：
  - [ ] 单元测试：掩码生成/裁剪、MaxShift/General Scaling 系数还原。
  - [ ] 集成测试：多 ROI、多组件、多 tile；边界 ROI；无 ROI 回退。
  - [ ] 互操作性：与 OpenJPEG/其它实现的 ROI 解码对比（尽可能基于标准 RGN）。
  - [ ] 性能与内存评估（大图+多 ROI）。
- [ ] 文档：README/参数文档更新，说明私有元数据的兼容性与开关。

## 里程碑与验收
- M1：数据结构/API 定稿，掩码生成完成。
- M2：MaxShift 多 ROI 全通；标准 RGN 写入/解析；基本测试通过。
- M3：General Scaling 完成；互操作测试通过（能解码标准 ROI 码流）。
- M4：私有 ROI 元数据扩展（可选）；文档与示例更新。
