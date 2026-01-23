# AGENTS.md

本文件为 Codex 在本仓库工作的说明。

## 项目概述

go-dicom-codec 是一个 Go 语言实现的 DICOM 图像编解码库，专注于实现编码/解码算法本身。
DICOM 封装、分片、元数据、传输语法管理等由外部 DICOM 库负责。

支持/规划的编解码家族：
- JPEG（Baseline/Extended/Lossless）
- JPEG-LS（规划中）
- JPEG 2000（规划中）

## 常用命令

### 构建
```bash
go build ./...
```

### 测试
```bash
# 全量测试
go test ./...

# 覆盖率
go test -cover ./...

# 详细输出
go test -v ./...

# 指定包
go test ./jpeg/baseline
go test ./jpeg/lossless14sv1

# 基准
go test -bench=. ./...
```

### 开发
```bash
# 格式化
go fmt ./...

# 运行 linter（需要 golangci-lint）
golangci-lint run

# 依赖整理
go mod tidy

# 依赖校验
go mod verify
```

## 结构与核心概念

### 目录结构
```
codec/              # 核心接口与注册表
jpeg/               # JPEG 家族
  common/           # 共享工具（Huffman/DCT/markers 等）
  baseline/         # JPEG Baseline（Process 1）
  lossless14sv1/    # JPEG Lossless SV1（Process 14, Predictor 1）
jpegls/             # JPEG-LS（规划中）
jpeg2000/           # JPEG 2000（规划中）
```

### 编解码接口
- 所有 codec 实现 `codec.Codec` 接口
- 每个 codec 有唯一 UID（通常为 DICOM Transfer Syntax UID）
- 通过全局注册表按名称或 UID 获取

### Encode/Decode
- Encode: `codec.EncodeParams` 包含像素数据、尺寸、分量、位深、选项
- Decode: `codec.DecodeResult` 返回像素与元信息

## JPEG 关键细节

- Byte Stuffing：扫描数据中 0xFF 后必须补 0x00
- Baseline 颜色空间：RGB -> YCbCr，包含 4:2:0 采样
- Huffman：使用 `jpeg/common/tables.go` 标准表
- DCT/IDCT：整数实现，8x8 block，注意舍入与裁剪

## 测试策略

- 单元测试：编码/解码分别测试，覆盖多尺寸/位深/边界条件
- Round-trip：
  - Lossless 必须 0 误差
  - Lossy 在质量范围内误差可接受
- 兼容性：与标准库 image/jpeg 或其他实现互操作
- 基准：不同尺寸评估性能

## 新增 Codec（示例：JPEG-LS）

1. 建结构
```
jpegls/
  lossless/
    encoder.go
    decoder.go
    codec.go
    options.go
    lossless_test.go
```

2. 实现接口并注册
```go
type Codec struct {}

func (c *Codec) Encode(params codec.EncodeParams) ([]byte, error) { ... }
func (c *Codec) Decode(data []byte) (*codec.DecodeResult, error) { ... }
func (c *Codec) UID() string { return "1.2.840.10008.1.2.4.80" }
func (c *Codec) Name() string { return "jpeg-ls-lossless" }

func init() { codec.Register(&Codec{}) }
```

3. 补齐测试与基准

## 开发规范

- 纯 Go 实现，禁止 CGO
- 错误信息清晰
- 热路径关注性能
- 公共 API 需要文档
- 变更前运行 `golangci-lint run`

## 规划

详见 `TODO.md`



## 源码目录

- OpenJPEG的源码在当前根目录下的 fo-dicom-codec-code/Native/Common/OpenJPEG

- OpenJPH的源码在当前根目录下的 fo-dicom-codec-code/Native/Common/OpenJPH