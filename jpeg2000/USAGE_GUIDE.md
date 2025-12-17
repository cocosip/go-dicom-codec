# JPEG 2000 Complete Usage Guide

Complete guide for using the JPEG 2000 codec in go-dicom-codec.

## Table of Contents

- [Quick Start](#quick-start)
- [Lossless vs Lossy: Choosing the Right Mode](#lossless-vs-lossy-choosing-the-right-mode)
- [Basic Usage](#basic-usage)
- [Parameter Reference](#parameter-reference)
- [Advanced Features](#advanced-features)
  - [Multi-tile Encoding](#multi-tile-encoding)
  - [Region of Interest (ROI)](#region-of-interest-roi)
  - [Progressive/Multi-layer Encoding](#progressivemulti-layer-encoding)
  - [Part 2 Multi-component Transform](#part-2-multi-component-transform)
  - [HTJ2K High-Throughput](#htj2k-high-throughput)
- [Performance Tuning](#performance-tuning)
- [Code Examples](#code-examples)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

### Installation

```go
import (
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)
```

### Minimal Example: Lossless Compression

```go
// Create source pixel data
src := &codec.PixelData{
    Data:            rawPixelData,      // []byte
    Width:           512,
    Height:          512,
    SamplesPerPixel: 1,                 // Grayscale
    BitsStored:      8,
}

// Create codec
losslessCodec := lossless.NewCodec()

// Encode
dst := &codec.PixelData{}
err := losslessCodec.Encode(src, dst, nil)
if err != nil {
    panic(err)
}

// dst.Data now contains JPEG 2000 compressed data
fmt.Printf("Compressed: %d bytes -> %d bytes (%.1fx compression)\n",
    len(src.Data), len(dst.Data),
    float64(len(src.Data))/float64(len(dst.Data)))
```

### Minimal Example: Lossy Compression

```go
// Create lossy codec with quality 85
lossyCodec := lossy.NewCodec(85)

// Encode
dst := &codec.PixelData{}
err := lossyCodec.Encode(src, dst, nil)
if err != nil {
    panic(err)
}

// Typical compression: 3-5x with quality 85
```

---

## Lossless vs Lossy: Choosing the Right Mode

### Lossless Compression (5/3 Wavelet)

**Use when:**
- Perfect reconstruction is required (medical diagnosis, archival)
- No information loss is acceptable
- Image quality is paramount

**Characteristics:**
- ✅ **Perfect reconstruction** - 0 pixel errors
- ✅ **Reversible** - Decode returns exact original
- 📊 **Compression ratio**: 4-7x for medical images
- 🎚️ **Parameters**: Only `NumLevels` (0-6)
- 🔬 **Transfer Syntax UID**: `1.2.840.10008.1.2.4.90`

**Typical Compression Ratios:**
- Medical CT/MRI: 5-6x
- X-Ray: 4-5x
- Ultrasound: 6-7x

```go
codec := lossless.NewCodec()
params := lossless.NewLosslessParameters().WithNumLevels(5)
err := codec.Encode(src, dst, params)
```

---

### Lossy Compression (9/7 Wavelet)

**Use when:**
- Some information loss is acceptable
- Smaller file sizes are needed
- Storage/bandwidth is limited

**Characteristics:**
- 📉 **Controlled quality loss** - Configurable quality 1-100
- 📊 **Compression ratio**: 3-30x depending on quality
- 🎚️ **Parameters**: `Quality`, `NumLevels`, `TargetRatio`, etc.
- 🔬 **Transfer Syntax UID**: `1.2.840.10008.1.2.4.91`

**Quality Settings Guide:**
- **Quality 95-100**: Near-lossless (~2-3x compression, max error ≤1-2 pixels)
- **Quality 80-90**: High quality (~3-5x compression, visually lossless)
- **Quality 60-79**: Medium quality (~5-8x compression, minor artifacts)
- **Quality 40-59**: Low quality (~8-15x compression, visible artifacts)
- **Quality 1-39**: High compression (~15-30x, significant quality loss)

```go
codec := lossy.NewCodec(85)
params := lossy.NewLossyParameters().
    WithQuality(85).
    WithNumLevels(5)
err := codec.Encode(src, dst, params)
```

---

## Basic Usage

### 1. Lossless Encoding with Custom Parameters

```go
import (
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Create parameters with custom settings
params := lossless.NewLosslessParameters().
    WithNumLevels(5)  // 5 wavelet decomposition levels

// Create codec
codec := lossless.NewCodec()

// Prepare pixel data
src := &codec.PixelData{
    Data:                     pixelBytes,
    Width:                    512,
    Height:                   512,
    SamplesPerPixel:          1,        // 1=Grayscale, 3=RGB
    BitsStored:               8,
    PixelRepresentation:      0,        // 0=unsigned, 1=signed
    PhotometricInterpretation: "MONOCHROME2",
}

// Encode
dst := &codec.PixelData{}
err := codec.Encode(src, dst, params)
if err != nil {
    return err
}

// Compressed data in dst.Data
```

### 2. Lossy Encoding with Quality Control

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"

// Method 1: Using Quality parameter
params := lossy.NewLossyParameters().
    WithQuality(90).      // 1-100: higher = better quality
    WithNumLevels(5)      // More levels = better compression

codec := lossy.NewCodec(90)
err := codec.Encode(src, dst, params)

// Method 2: Using Target Compression Ratio
params := lossy.NewLossyParameters().
    WithTargetRatio(5.0). // Target 5:1 compression
    WithNumLevels(5)

err := codec.Encode(src, dst, params)
```

### 3. Decoding

```go
// Decoding is the same for both lossless and lossy
decoder := lossless.NewCodec()  // or lossy.NewCodec(quality)

src := &codec.PixelData{
    Data: compressedData,  // JPEG 2000 compressed bytes
}

dst := &codec.PixelData{}
err := decoder.Decode(src, dst, nil)
if err != nil {
    return err
}

// Decompressed pixel data in dst.Data
// Image dimensions in dst.Width, dst.Height
```

---

## Parameter Reference

### Lossless Parameters

#### `NumLevels` (int, default: 5)

Number of wavelet decomposition levels (0-6).

**Guidelines:**
- **0 levels**: No wavelet decomposition (minimal compression, fastest)
  - Use for: Debugging, very small images (<32x32)
  - Compression: ~2x
- **1-2 levels**: Light decomposition
  - Use for: Small images (32x128)
  - Compression: ~3-4x
- **3-4 levels**: Medium decomposition
  - Use for: Medium images (128-256)
  - Compression: ~4-5x
- **5 levels**: Default, recommended
  - Use for: Most images (256-1024)
  - Compression: ~5-6x
- **6 levels**: Maximum decomposition
  - Use for: Large images (>1024x1024)
  - Compression: ~6-7x

**Important**: The encoder automatically clamps NumLevels to ensure the LL band is at least 2x2 pixels.

```go
// Example: Choosing NumLevels based on image size
func chooseNumLevels(width, height int) int {
    minDim := width
    if height < minDim {
        minDim = height
    }

    switch {
    case minDim < 64:
        return 1
    case minDim < 128:
        return 3
    case minDim < 512:
        return 5
    default:
        return 6
    }
}

params := lossless.NewLosslessParameters().
    WithNumLevels(chooseNumLevels(width, height))
```

---

### Lossy Parameters

#### `Quality` (int, default: 80)

Compression quality (1-100), where 100 is near-lossless.

**Quality Scale:**
```
100 ─┬─ Near-lossless (2-3x compression, max error ≤1-2 pixels)
  95 ─┤
  90 ─┼─ High quality (3-5x compression, visually lossless)
  85 ─┤
  80 ─┤ ← Default
  75 ─┼─ Good quality (5-7x compression, minor artifacts)
  70 ─┤
  60 ─┼─ Medium quality (7-10x compression, visible artifacts)
  50 ─┤
  40 ─┼─ Low quality (10-15x compression)
  30 ─┤
  20 ─┼─ High compression (15-30x, significant loss)
  10 ─┤
   1 ─┴─ Maximum compression (30x+, poor quality)
```

**Examples:**
```go
// Archival quality (near-lossless)
params := lossy.NewLossyParameters().WithQuality(95)

// Diagnostic quality (visually lossless)
params := lossy.NewLossyParameters().WithQuality(85)

// Web preview quality
params := lossy.NewLossyParameters().WithQuality(60)

// Thumbnail quality
params := lossy.NewLossyParameters().WithQuality(40)
```

#### `NumLevels` (int, default: 5)

Same as lossless. More levels = better compression but slower.

#### `NumLayers` (int, default: 1)

Number of quality layers for progressive refinement.

**Use cases:**
- **1 layer**: Single quality level (default, fastest)
- **2-3 layers**: Progressive download (web, streaming)
- **4+ layers**: Fine-grained progressive refinement

```go
// Progressive encoding with 3 quality layers
params := lossy.NewLossyParameters().
    WithQuality(85).
    WithNumLayers(3)
```

See [Progressive/Multi-layer Encoding](#progressivemulti-layer-encoding) for details.

#### `TargetRatio` (float64, default: 0)

Target compression ratio (original_size / compressed_size).

**When to use:**
- You need predictable file sizes
- Storage quota management
- Bandwidth constraints

**How it works:**
- If `TargetRatio > 0`, the encoder uses PCRD (Post-Compression Rate-Distortion) optimization
- Quality parameter is estimated from target ratio
- Actual ratio may vary ±5-20% depending on image complexity

**Examples:**
```go
// Target 5:1 compression (e.g., 512KB -> ~102KB)
params := lossy.NewLossyParameters().WithTargetRatio(5.0)

// Target 10:1 compression (higher compression)
params := lossy.NewLossyParameters().WithTargetRatio(10.0)

// Combine with NumLayers for progressive + rate control
params := lossy.NewLossyParameters().
    WithTargetRatio(8.0).
    WithNumLayers(3)
```

**Achievable Ratios:**
- Medical images: 2-15x typical
- Natural images: 5-30x typical
- Very high ratios (>20x) will show visible artifacts

#### `QuantStepScale` (float64, default: 1.0)

Global quantization step scale multiplier.

**Advanced parameter** - Use when you need fine control over quantization.

- `QuantStepScale = 1.0`: Normal quantization (default)
- `QuantStepScale > 1.0`: Coarser quantization (more compression, lower quality)
- `QuantStepScale < 1.0`: Finer quantization (less compression, higher quality)

```go
// Increase compression by 20%
params := lossy.NewLossyParameters().
    WithQuality(80).
    WithQuantStepScale(1.2)
```

#### `SubbandSteps` ([]float64, default: nil)

Custom per-subband quantization steps.

**Advanced parameter** - For expert users who need precise control.

**Requirements:**
- Length must be exactly `3*NumLevels + 1`
- Order: LL, HL1, LH1, HH1, HL2, LH2, HH2, ..., HLN, LHN, HHN

```go
// Example for NumLevels=2 (3*2+1 = 7 steps)
params := lossy.NewLossyParameters().
    WithNumLevels(2).
    WithSubbandSteps([]float64{
        0.05,  // LL (lowest frequency)
        0.10,  // HL1
        0.10,  // LH1
        0.15,  // HH1 (higher quantization = more compression)
        0.20,  // HL2
        0.20,  // LH2
        0.25,  // HH2 (highest frequency, most compression)
    })
```

---

## Advanced Features

### Multi-tile Encoding

JPEG 2000 supports dividing large images into independent tiles for parallel processing and random access.

**Benefits:**
- 🚀 Parallel encoding/decoding
- 💾 Lower memory usage (process one tile at a time)
- 🎯 Random access to image regions
- 📦 Suitable for very large images (>4K)

**Use cases:**
- Large medical images (whole slide imaging, CT volumes)
- Gigapixel images
- Distributed processing

#### Basic Multi-tile Example

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000"

// Create encoding parameters with tiling
params := jpeg2000.DefaultEncodeParams(
    2048,  // width
    2048,  // height
    1,     // components (grayscale)
    8,     // bit depth
    false, // unsigned
)

// Configure tiles (512x512 tiles = 4x4 grid = 16 tiles)
params.TileWidth = 512
params.TileHeight = 512
params.NumLevels = 5
params.Lossless = true

// Encode
encoder := jpeg2000.NewEncoder(params)
compressed, err := encoder.Encode(pixelData)
if err != nil {
    return err
}

// Decoder automatically handles multi-tile codestreams
decoder := jpeg2000.NewDecoder()
err = decoder.Decode(compressed)
```

#### Tile Size Guidelines

**Recommended tile sizes:**
- **256x256**: Small tiles, high overhead, good for random access
- **512x512**: Balanced (recommended for most uses)
- **1024x1024**: Large tiles, less overhead, better compression
- **Image size**: Single tile (no tiling overhead)

**Rules:**
- Tile dimensions must divide image dimensions evenly (or use partial tiles)
- Smaller tiles = more overhead but better random access
- Larger tiles = less overhead but more memory

```go
// Example: Choosing tile size based on image size
func chooseTileSize(width, height int) (tileW, tileH int) {
    if width <= 512 && height <= 512 {
        // Small image: no tiling
        return 0, 0
    } else if width <= 2048 && height <= 2048 {
        // Medium image: 512x512 tiles
        return 512, 512
    } else {
        // Large image: 1024x1024 tiles
        return 1024, 1024
    }
}
```

#### Using Lossy Codec with Tiling

```go
// Tiling works with both lossless and lossy codecs
// But parameters must be passed differently

// For lossy codec, use lower-level API:
encParams := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
encParams.Lossless = false
encParams.Quality = 85
encParams.TileWidth = 512
encParams.TileHeight = 512

encoder := jpeg2000.NewEncoder(encParams)
compressed, err := encoder.Encode(pixelData)
```

**Note:** Direct tile access (encoding/decoding individual tiles) is supported through the lower-level encoder/decoder APIs. See `tile_assembler.go` for implementation details.

---

### Region of Interest (ROI)

Encode important image regions with higher quality while compressing background at lower quality.

**Use cases:**
- Medical imaging: Encode lesion/tumor with high quality, background with lower quality
- Surveillance: High-quality face region, low-quality background
- Satellite imagery: High-quality target area

**Supported features:**
- ✅ Multiple ROI regions
- ✅ Rectangle, Polygon, and Bitmap mask shapes
- ✅ Two styles: MaxShift (background shift) and General Scaling
- ✅ Per-component ROI support
- ✅ Main header and Tile-part header RGN markers

#### Simple Rectangle ROI

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000"

// Define ROI rectangle (in pixels)
roiParams := &jpeg2000.ROIParams{
    X0:     100,   // Top-left X
    Y0:     100,   // Top-left Y
    Width:  200,   // ROI width
    Height: 200,   // ROI height
    Shift:  3,     // Background shift (0-7, higher = more compression)
}

// Create encoding parameters
encParams := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
encParams.Lossless = false
encParams.Quality = 70          // Background quality
encParams.ROI = roiParams       // ROI will be encoded at higher quality

encoder := jpeg2000.NewEncoder(encParams)
compressed, err := encoder.Encode(pixelData)
```

**Shift parameter:**
- `Shift = 0`: No ROI (same quality everywhere)
- `Shift = 1-3`: Slight background degradation
- `Shift = 4-5`: Moderate background degradation (recommended)
- `Shift = 6-7`: Heavy background degradation

#### Multiple ROI Regions with ROIConfig

```go
// Create ROI configuration with multiple regions
roiConfig := &jpeg2000.ROIConfig{
    DefaultShift: 5,
    DefaultStyle: jpeg2000.ROIStyleMaxShift,
    ROIs: []jpeg2000.ROIRegion{
        {
            ID:    "lesion1",
            Rect:  &jpeg2000.ROIParams{X0: 100, Y0: 100, Width: 50, Height: 50},
            Shift: 6,  // Higher shift = more background compression
        },
        {
            ID:    "lesion2",
            Rect:  &jpeg2000.ROIParams{X0: 300, Y0: 200, Width: 80, Height: 80},
            Shift: 5,
        },
    },
}

encParams := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
encParams.Lossless = false
encParams.Quality = 60          // Low background quality
encParams.ROIConfig = roiConfig

encoder := jpeg2000.NewEncoder(encParams)
compressed, err := encoder.Encode(pixelData)
```

#### Polygon ROI

```go
// Define ROI as polygon
roiConfig := &jpeg2000.ROIConfig{
    ROIs: []jpeg2000.ROIRegion{
        {
            ID:    "irregular_region",
            Shape: jpeg2000.ROIShapePolygon,
            Polygon: []jpeg2000.Point{
                {X: 100, Y: 100},
                {X: 200, Y: 120},
                {X: 180, Y: 200},
                {X: 90, Y: 190},
            },
            Shift: 5,
        },
    },
}
```

#### Bitmap Mask ROI

```go
// Create binary mask (true = ROI, false = background)
maskWidth := 512
maskHeight := 512
mask := make([]bool, maskWidth*maskHeight)

// Set ROI pixels (example: circular region)
centerX, centerY, radius := 256, 256, 100
for y := 0; y < maskHeight; y++ {
    for x := 0; x < maskWidth; x++ {
        dx, dy := x-centerX, y-centerY
        if dx*dx+dy*dy <= radius*radius {
            mask[y*maskWidth+x] = true
        }
    }
}

roiConfig := &jpeg2000.ROIConfig{
    ROIs: []jpeg2000.ROIRegion{
        {
            ID:         "circular_roi",
            Shape:      jpeg2000.ROIShapeMask,
            MaskWidth:  maskWidth,
            MaskHeight: maskHeight,
            MaskData:   mask,
            Shift:      5,
        },
    },
}
```

#### General Scaling Style

```go
// General Scaling: uses scaling factor instead of bit-plane shift
// More flexible but more complex
roiConfig := &jpeg2000.ROIConfig{
    DefaultStyle: jpeg2000.ROIStyleGeneralScaling,
    ROIs: []jpeg2000.ROIRegion{
        {
            ID:    "roi1",
            Rect:  &jpeg2000.ROIParams{X0: 100, Y0: 100, Width: 200, Height: 200},
            Scale: 5,  // Scaling factor (similar semantics to Shift)
        },
    },
}
```

**ROI Decoding:**
- Decoder automatically detects and applies ROI from RGN markers
- No special decoding parameters needed
- ROI information is embedded in the codestream

---

### Progressive/Multi-layer Encoding

Encode image with multiple quality layers for progressive refinement.

**Use cases:**
- 🌐 Progressive image loading on web
- 📡 Streaming over low-bandwidth networks
- 🎞️ Quick preview + high-quality detail
- 💾 Storage with multiple quality versions

**Benefits:**
- Single file contains multiple quality levels
- Decoder can stop at any layer (lower quality, faster decode)
- Efficient bandwidth usage (stream only needed layers)

#### Basic Multi-layer Encoding

```go
params := lossy.NewLossyParameters().
    WithQuality(85).
    WithNumLayers(5)  // 5 quality layers (low to high)

codec := lossy.NewCodec(85)
err := codec.Encode(src, dst, params)
```

**Layer distribution:**
- Layer 0: Lowest quality (fast preview)
- Layer 1-3: Progressive refinement
- Layer 4: Highest quality (full detail)

#### Layer Allocation with Target Ratio

```go
// Combine layers with target compression ratio
params := lossy.NewLossyParameters().
    WithTargetRatio(10.0).  // Target 10:1 overall compression
    WithNumLayers(4)         // 4 quality layers

// PCRD optimization distributes bits optimally across layers
err := codec.Encode(src, dst, params)
```

**How PCRD works:**
- Post-Compression Rate-Distortion optimization
- Automatically allocates bits to each layer for optimal quality
- Ensures each layer adds meaningful quality improvement
- Respects target compression ratio constraint

#### Progression Orders

JPEG 2000 supports 5 progression orders for organizing layers:

```go
encParams := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
encParams.NumLayers = 3
encParams.ProgressionOrder = 0  // LRCP (default)

// Progression orders:
// 0 = LRCP: Layer-Resolution-Component-Position (recommended)
// 1 = RLCP: Resolution-Layer-Component-Position
// 2 = RPCL: Resolution-Position-Component-Layer
// 3 = PCRL: Position-Component-Resolution-Layer
// 4 = CPRL: Component-Position-Resolution-Layer
```

**Guidelines:**
- **LRCP** (0): Best for progressive quality (default, recommended)
- **RLCP** (1): Good for progressive resolution
- **RPCL** (2): Good for spatial random access
- **PCRL/CPRL**: Specialized uses

#### Decoding Specific Layers

```go
// Decoder automatically handles multi-layer codestreams
decoder := jpeg2000.NewDecoder()

// Decode all layers (full quality)
err := decoder.Decode(compressed)
fullQualityData := decoder.GetPixelData()

// Note: Partial layer decoding requires lower-level API
// See t2/tile_decoder.go for layer-by-layer decoding
```

---

### Part 2 Multi-component Transform

JPEG 2000 Part 2 supports custom multi-component transforms (MCT) beyond standard RGB↔YCbCr.

**Use cases:**
- Custom color space transformations
- Multi-spectral imaging (hyperspectral, satellite)
- Medical imaging (PET/CT fusion, multi-modal)
- Scientific imaging with >3 channels

**Features:**
- ✅ Custom matrix transforms (float32 or int32)
- ✅ Component offsets (shifts)
- ✅ Reversible integer transforms (lossless)
- ✅ Multiple transform stages (matrix + offset combinations)
- ✅ MCT, MCC, MCO marker support

#### Simple Custom Matrix Transform

```go
import (
    "github.com/cocosip/go-dicom-codec/jpeg2000"
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
)

// Define custom 3x3 transform matrix (e.g., custom RGB to decorrelated space)
matrix := [][]float64{
    {0.299, 0.587, 0.114},      // Y component
    {-0.168736, -0.331264, 0.5}, // Cb component
    {0.5, -0.418688, -0.081312}, // Cr component
}

// Define inverse matrix (for decoding)
inverseMatrix := [][]float64{
    {1.0, 0.0, 1.402},
    {1.0, -0.344136, -0.714136},
    {1.0, 1.772, 0.0},
}

// Create parameters with MCT
params := lossy.NewLossyParameters().
    WithQuality(85).
    SetParameter("mctMatrix", matrix).
    SetParameter("inverseMctMatrix", inverseMatrix).
    SetParameter("mctMatrixElementType", uint8(1)) // 1=float32

// Use Part 2 Multi-component codec
codec := lossy.NewPart2MultiComponentCodec(85)
err := codec.Encode(src, dst, params)
```

#### Reversible Integer Transform (Lossless)

```go
// For lossless compression, use integer matrix with NormScale
matrix := [][]float64{
    {1, 0, 1},
    {1, -1, 0},
    {0, 1, -1},
}

params := lossless.NewLosslessParameters().
    SetParameter("mctMatrix", matrix).
    SetParameter("mctMatrixElementType", uint8(0)). // 0=int32
    SetParameter("mctNormScale", float64(1.0)).     // NormScale=1 for reversible
    SetParameter("mcoPrecision", uint8(0x01))       // Reversible flag

codec := lossless.NewPart2MultiComponentLosslessCodec()
err := codec.Encode(src, dst, params)
```

#### Builder API for Complex Transforms

```go
// Use builder for complex multi-stage transforms
binding := jpeg2000.NewMCTBinding().
    Assoc(2).                              // AssocType: 2=matrix then offset
    Components([]uint16{0, 1, 2}).        // Apply to RGB components
    Matrix(matrix).
    Inverse(inverseMatrix).
    Offsets([]int32{128, 0, 0}).         // Optional offsets
    ElementType(1).                        // 1=float32
    MCOPrecision(0x00).                    // Rounding: nearest
    Build()

params := lossy.NewLossyParameters().
    WithMCTBindings([]jpeg2000.MCTBindingParams{binding})

codec := lossy.NewPart2MultiComponentCodec(85)
err := codec.Encode(src, dst, params)
```

**MCOPrecision Bits:**
- `0x01`: Reversible flag (for lossless integer transforms)
- `0x00`: RoundNearest (default)
- `0x04`: Floor
- `0x08`: Ceil
- `0x0C`: Truncate

**For more details, see the Part 2 Multi-component section in the main README.**

---

### HTJ2K High-Throughput

HTJ2K (ISO/IEC 15444-15:2019) is a high-throughput variant of JPEG 2000.

**Benefits:**
- ⚡ **4-10x faster** encoding/decoding than traditional JPEG 2000
- 🎯 Designed for real-time applications
- 📦 Similar compression efficiency
- 🔬 DICOM UIDs: `.201` (Lossless), `.202` (Lossless RPCL), `.203` (Lossy)

**Status:** ✅ Complete implementation (2025-12)

#### Using HTJ2K Codec

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000/htj2k"

// HTJ2K Lossless
codec := htj2k.NewHTJ2KLosslessCodec()
params := htj2k.NewHTJ2KParameters() // Default lossless settings

err := codec.Encode(src, dst, params)

// HTJ2K Lossy
codec := htj2k.NewHTJ2KCodec(85) // Quality 85
params := htj2k.NewHTJ2KParameters().WithQuality(85)

err := codec.Encode(src, dst, params)
```

**HTJ2K Differences:**
- Uses HT (High-Throughput) block coding instead of EBCOT
- MEL (Adaptive Run-Length) + MagSgn + VLC encoding
- Simpler arithmetic coding (faster hardware implementation)
- Same wavelet transforms (5/3 lossless, 9/7 lossy)

**When to use HTJ2K:**
- Real-time medical imaging (fluoroscopy, ultrasound)
- Video streaming
- Low-latency applications
- Hardware-accelerated environments

**See `jpeg2000/htj2k/README.md` for detailed HTJ2K documentation.**

---

## Performance Tuning

### Choosing Optimal Parameters

#### Image Size vs NumLevels

```
Image Size       | Recommended NumLevels | Why
-----------------|-----------------------|----------------------------
< 64x64          | 0-1                   | Too small for wavelet benefit
64x64 - 128x128  | 1-3                   | Light decomposition
128x128 - 512x512| 3-5                   | Balanced (5 is default)
512x512 - 2048   | 5-6                   | Better compression
> 2048x2048      | 6 + tiling            | Use tiles for large images
```

#### Quality vs Compression Ratio (Lossy)

```
Quality | Compression | Use Case
--------|-------------|----------------------------------
95-100  | 2-3x        | Archival, near-lossless
80-90   | 3-5x        | Diagnostic medical imaging
60-79   | 5-8x        | Web preview, telemedicine
40-59   | 8-15x       | Thumbnails, low-bandwidth
1-39    | 15-30x      | Extreme compression (poor quality)
```

### Memory Usage

**Lossless encoding memory:**
```
Memory ≈ ImageSize × (2 + NumLevels × 0.5)
```

Example for 512x512x1 image (256KB):
- NumLevels=3: ~640KB
- NumLevels=5: ~896KB

**Tips for reducing memory:**
- Use tiling for large images (process tiles independently)
- Lower NumLevels (less wavelet coefficients)
- Process frames sequentially (don't load all at once)

### Encoding Speed

**Fastest settings (lossless):**
```go
params := lossless.NewLosslessParameters().
    WithNumLevels(1)  // Minimal wavelet decomposition
```

**Fastest settings (lossy):**
```go
params := lossy.NewLossyParameters().
    WithQuality(80).
    WithNumLevels(3).    // Fewer levels = faster
    WithNumLayers(1)     // Single layer = faster
```

**Slowest but best compression:**
```go
params := lossy.NewLossyParameters().
    WithQuality(85).
    WithNumLevels(6).
    WithNumLayers(5).
    WithTargetRatio(10.0)  // PCRD optimization adds overhead
```

### Compression vs Speed Tradeoff

| Setting           | Encode Time | Compression | Use Case              |
|-------------------|-------------|-------------|-----------------------|
| NumLevels=1       | Fast        | ~3-4x       | Quick preview         |
| NumLevels=3       | Medium      | ~4-5x       | Balanced              |
| NumLevels=5       | Slow        | ~5-6x       | Default (recommended) |
| NumLevels=6       | Slower      | ~6-7x       | Best compression      |
| + Multi-layer     | Slower      | Same        | Progressive           |
| + TargetRatio     | Slowest     | Controlled  | Rate control          |
| + Tiling          | Parallel    | Slight loss | Large images          |
| HTJ2K             | **Fastest** | Same        | Real-time             |

---

## Code Examples

### Example 1: Complete Lossless Workflow

```go
package main

import (
    "fmt"
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
    // Load raw pixel data (example: 512x512 8-bit grayscale)
    width, height := 512, 512
    pixelData := make([]byte, width*height)
    // ... fill pixelData with actual image data ...

    // Create source
    src := &codec.PixelData{
        Data:                     pixelData,
        Width:                    uint16(width),
        Height:                   uint16(height),
        SamplesPerPixel:          1,
        BitsStored:               8,
        PixelRepresentation:      0,
        PhotometricInterpretation: "MONOCHROME2",
    }

    // Create parameters
    params := lossless.NewLosslessParameters().
        WithNumLevels(5)

    // Create codec and encode
    encoder := lossless.NewCodec()
    dst := &codec.PixelData{}
    err := encoder.Encode(src, dst, params)
    if err != nil {
        panic(fmt.Sprintf("Encode failed: %v", err))
    }

    // Report compression
    ratio := float64(len(src.Data)) / float64(len(dst.Data))
    fmt.Printf("Compressed: %d bytes -> %d bytes (%.1fx)\n",
        len(src.Data), len(dst.Data), ratio)

    // Decode back
    decoded := &codec.PixelData{}
    err = encoder.Decode(dst, decoded, nil)
    if err != nil {
        panic(fmt.Sprintf("Decode failed: %v", err))
    }

    // Verify perfect reconstruction
    errors := 0
    for i := range src.Data {
        if src.Data[i] != decoded.Data[i] {
            errors++
        }
    }
    fmt.Printf("Pixel errors: %d (should be 0 for lossless)\n", errors)
}
```

### Example 2: Lossy Encoding with Quality Control

```go
package main

import (
    "fmt"
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
    // ... load pixelData ...

    src := &codec.PixelData{
        Data:            pixelData,
        Width:           512,
        Height:          512,
        SamplesPerPixel: 1,
        BitsStored:      8,
    }

    // Try different quality levels
    qualities := []int{95, 85, 70, 50}

    for _, quality := range qualities {
        params := lossy.NewLossyParameters().
            WithQuality(quality).
            WithNumLevels(5)

        codec := lossy.NewCodec(quality)
        dst := &codec.PixelData{}
        err := codec.Encode(src, dst, params)
        if err != nil {
            fmt.Printf("Quality %d: encode failed: %v\n", quality, err)
            continue
        }

        ratio := float64(len(src.Data)) / float64(len(dst.Data))
        fmt.Printf("Quality %d: %d bytes -> %d bytes (%.1fx compression)\n",
            quality, len(src.Data), len(dst.Data), ratio)

        // Decode and measure error
        decoded := &codec.PixelData{}
        err = codec.Decode(dst, decoded, nil)
        if err != nil {
            fmt.Printf("Quality %d: decode failed: %v\n", quality, err)
            continue
        }

        // Calculate PSNR or max error
        maxError := 0
        for i := range src.Data {
            diff := int(src.Data[i]) - int(decoded.Data[i])
            if diff < 0 {
                diff = -diff
            }
            if diff > maxError {
                maxError = diff
            }
        }
        fmt.Printf("Quality %d: Max pixel error: %d\n", quality, maxError)
    }
}
```

### Example 3: Progressive Encoding and Streaming

```go
package main

import (
    "fmt"
    "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
    // ... load pixelData ...

    src := &codec.PixelData{
        Data:            pixelData,
        Width:           1024,
        Height:          1024,
        SamplesPerPixel: 1,
        BitsStored:      8,
    }

    // Create progressive encoding with 5 layers
    params := lossy.NewLossyParameters().
        WithQuality(85).
        WithNumLayers(5).
        WithTargetRatio(8.0)  // Target 8:1 compression

    codec := lossy.NewCodec(85)
    dst := &codec.PixelData{}
    err := codec.Encode(src, dst, params)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Encoded with 5 quality layers\n")
    fmt.Printf("Total size: %d bytes\n", len(dst.Data))
    fmt.Printf("Compression ratio: %.1fx\n",
        float64(len(src.Data))/float64(len(dst.Data)))

    // In a real application, you would:
    // 1. Stream layers progressively over network
    // 2. Decoder shows preview after layer 1
    // 3. Decoder refines image as more layers arrive
    // 4. Full quality after all layers decoded

    // Decode (all layers)
    decoded := &codec.PixelData{}
    err = codec.Decode(dst, decoded, nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Decoded: %dx%d\n", decoded.Width, decoded.Height)
}
```

### Example 4: Multi-tile Encoding for Large Images

```go
package main

import (
    "fmt"
    "github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
    // Large image: 4096x4096
    width, height := 4096, 4096
    pixelData := make([]byte, width*height)
    // ... load pixel data ...

    // Configure tiling (512x512 tiles = 8x8 grid = 64 tiles)
    params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
    params.TileWidth = 512
    params.TileHeight = 512
    params.NumLevels = 5
    params.Lossless = true

    // Encode
    encoder := jpeg2000.NewEncoder(params)
    compressed, err := encoder.Encode(pixelData)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Encoded %dx%d image with %dx%d tiles\n",
        width, height,
        params.TileWidth, params.TileHeight)
    fmt.Printf("Number of tiles: %d\n",
        (width/params.TileWidth)*(height/params.TileHeight))
    fmt.Printf("Compressed size: %d bytes (%.1fx)\n",
        len(compressed),
        float64(len(pixelData))/float64(len(compressed)))

    // Decode (automatically handles tiles)
    decoder := jpeg2000.NewDecoder()
    err = decoder.Decode(compressed)
    if err != nil {
        panic(err)
    }

    decodedData := decoder.GetPixelData()
    fmt.Printf("Decoded: %dx%d\n", decoder.Width(), decoder.Height())

    // Verify
    errors := 0
    for i := range pixelData {
        if pixelData[i] != decodedData[i] {
            errors++
        }
    }
    fmt.Printf("Pixel errors: %d\n", errors)
}
```

### Example 5: ROI Encoding

```go
package main

import (
    "fmt"
    "github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
    width, height := 512, 512
    pixelData := make([]byte, width*height)
    // ... load pixel data ...

    // Define ROI: center 200x200 region
    roi := &jpeg2000.ROIParams{
        X0:     156,  // (512-200)/2
        Y0:     156,
        Width:  200,
        Height: 200,
        Shift:  5,    // Background will be heavily compressed
    }

    // Configure encoding
    params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
    params.Lossless = false
    params.Quality = 60     // Low quality for background
    params.NumLevels = 5
    params.ROI = roi        // ROI gets higher quality

    encoder := jpeg2000.NewEncoder(params)
    compressed, err := encoder.Encode(pixelData)
    if err != nil {
        panic(err)
    }

    fmt.Printf("ROI encoding:\n")
    fmt.Printf("  Background quality: %d\n", params.Quality)
    fmt.Printf("  ROI region: %dx%d at (%d,%d)\n",
        roi.Width, roi.Height, roi.X0, roi.Y0)
    fmt.Printf("  ROI shift: %d (background compressed %dx more)\n",
        roi.Shift, 1<<roi.Shift)
    fmt.Printf("  Compressed size: %d bytes\n", len(compressed))

    // Decode
    decoder := jpeg2000.NewDecoder()
    err = decoder.Decode(compressed)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Decoded successfully with ROI preserved\n")
}
```

---

## Troubleshooting

### Common Errors and Solutions

#### Error: "image too small for NumLevels"

**Cause:** Image dimensions are too small for the requested number of wavelet levels.

**Solution:**
```go
// Reduce NumLevels or let encoder auto-adjust
params := lossless.NewLosslessParameters().
    WithNumLevels(3)  // Use fewer levels for small images

// Or use auto-clamping (encoder does this automatically)
```

**Rule:** After N levels, smallest dimension should be ≥ 2 pixels
- 64x64 image: max 5 levels (64 → 32 → 16 → 8 → 4 → 2)
- 32x32 image: max 4 levels
- 16x16 image: max 3 levels

#### Error: "SubbandSteps length mismatch"

**Cause:** `SubbandSteps` array length doesn't match `3*NumLevels + 1`.

**Solution:**
```go
// For NumLevels=5, need 3*5+1 = 16 steps
steps := make([]float64, 16)
// ... fill steps ...
params := lossy.NewLossyParameters().
    WithNumLevels(5).
    WithSubbandSteps(steps)

// Or don't set SubbandSteps (let encoder compute automatically)
```

#### Low compression ratio (worse than expected)

**Possible causes and solutions:**

1. **Image is very noisy or random**
   - Solution: Use lossy compression or lower quality
   - Random data doesn't compress well

2. **NumLevels too low**
   - Solution: Increase NumLevels (try 5 or 6)
   ```go
   params.WithNumLevels(6)
   ```

3. **Image is too small**
   - Solution: Small images (<128x128) don't compress as well
   - Use NumLevels=1-3 for small images

4. **Using lossless on noisy data**
   - Solution: Consider lossy compression
   - Lossless can't remove noise, limits compression

#### High memory usage

**Solutions:**

1. **Use tiling for large images**
   ```go
   params.TileWidth = 512
   params.TileHeight = 512
   ```

2. **Reduce NumLevels**
   ```go
   params.WithNumLevels(3)  // Instead of 5-6
   ```

3. **Process frames sequentially** (for multi-frame images)
   - Don't load all frames at once
   - Encode/decode one frame at a time

#### Decode error: "invalid marker" or "unexpected EOC"

**Possible causes:**

1. **Corrupted data**
   - Verify data integrity
   - Check file I/O errors

2. **Wrong codec**
   - Ensure you're using JPEG 2000 decoder for JPEG 2000 data
   - Check Transfer Syntax UID

3. **Incomplete data**
   - Ensure complete codestream (SOC to EOC)
   - Check for truncated files

#### ROI not applied correctly

**Checks:**

1. **Verify ROI coordinates are within image bounds**
   ```go
   if !roi.IsValid(width, height) {
       fmt.Println("ROI coordinates invalid!")
   }
   ```

2. **Ensure Shift > 0**
   - Shift=0 means no ROI effect

3. **Quality must be lossy for visible ROI effect**
   - Lossless + ROI doesn't make sense
   - Use Quality < 100

---

## Best Practices

### Choosing Lossless vs Lossy

✅ **Use Lossless when:**
- Medical diagnosis (radiological images)
- Legal/archival requirements
- Perfect reconstruction needed
- Source is already compressed
- Small file size increase is acceptable

✅ **Use Lossy when:**
- Storage/bandwidth limited
- Visual quality sufficient (not diagnostic)
- Source is uncompressed raw data
- Compression ratio >5x needed

### Parameter Selection Guidelines

**For medical imaging:**
```go
// Diagnostic images (must be lossless)
params := lossless.NewLosslessParameters().WithNumLevels(5)

// Non-diagnostic images (lossy acceptable)
params := lossy.NewLossyParameters().WithQuality(85).WithNumLevels(5)

// Telemedicine/preview (aggressive lossy)
params := lossy.NewLossyParameters().WithQuality(70).WithNumLevels(5)
```

**For general images:**
```go
// Archival quality
params := lossy.NewLossyParameters().WithQuality(95).WithNumLevels(6)

// Web images
params := lossy.NewLossyParameters().WithQuality(75).WithNumLevels(5)

// Thumbnails
params := lossy.NewLossyParameters().WithQuality(50).WithNumLevels(3)
```

### Performance Tips

1. **Benchmark before optimizing**
   ```go
   import "testing"

   func BenchmarkEncode(b *testing.B) {
       codec := lossless.NewCodec()
       params := lossless.NewLosslessParameters()

       b.ResetTimer()
       for i := 0; i < b.N; i++ {
           codec.Encode(src, dst, params)
       }
   }
   ```

2. **Profile memory usage**
   ```bash
   go test -memprofile=mem.prof
   go tool pprof mem.prof
   ```

3. **Use HTJ2K for real-time applications**
   - 4-10x faster than traditional JPEG 2000
   - Same quality and compression

4. **Parallel processing for batch jobs**
   ```go
   // Process multiple images in parallel
   var wg sync.WaitGroup
   for _, img := range images {
       wg.Add(1)
       go func(img Image) {
           defer wg.Done()
           // Encode image
       }(img)
   }
   wg.Wait()
   ```

---

## Additional Resources

- **Main README**: `jpeg2000/README.md` - Technical details and status
- **Part 2 Guide**: Section in main README - Multi-component transforms
- **HTJ2K Guide**: `jpeg2000/htj2k/README.md` - High-Throughput JPEG 2000
- **Parameter Guide**: `PARAMETERS.md` - Type-safe parameters
- **Examples**: `examples/` directory - Complete working examples

### Standards References

- **ISO/IEC 15444-1:2019** - JPEG 2000 Part 1 (Core)
- **ISO/IEC 15444-2** - JPEG 2000 Part 2 (Extensions)
- **ISO/IEC 15444-15:2019** - JPEG 2000 Part 15 (HTJ2K)
- **DICOM PS3.5** - Transfer Syntax Specifications

### Support

For issues, questions, or contributions:
- GitHub Issues: [go-dicom-codec/issues](https://github.com/cocosip/go-dicom-codec/issues)
- Main project: [go-dicom-codec](https://github.com/cocosip/go-dicom-codec)

---

**Last Updated:** 2025-12-17
**Document Version:** 1.0
**Library Version:** Current main branch
