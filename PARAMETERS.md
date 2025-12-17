# Type-Safe Parameters Guide

This document explains the type-safe parameter system for codec configurations.

## Overview

Starting from this version, all codecs provide type-safe parameter structures that implement the `codec.Parameters` interface. This provides better type safety, IDE autocomplete, and compile-time checking.

## Benefits

### Before (String-Based Parameters)

```go
// ❌ Problems:
// - Easy to mistype: "quality" vs "qualty"
// - No IDE autocomplete
// - Requires type assertion
// - No compile-time checking
// - Have to read documentation to know available parameters

params := someGenericParameters()
params.SetParameter("quality", 95)           // What parameters exist?
q := params.GetParameter("quality").(int)    // Type assertion needed
```

### After (Type-Safe Parameters)

```go
// ✅ Benefits:
// - Compile-time type checking
// - IDE autocomplete shows all available parameters
// - No type assertion needed
// - Self-documenting code
// - Impossible to mistype parameter names

params := lossy.NewLossyParameters()
params.Quality = 95    // IDE autocomplete!
                       // Compile-time checking!
                       // No type assertion!
```

## Performance

Type-safe parameter access is **~6x faster** than string-based access:

```
BenchmarkTypeSafeVsStringBased/TypeSafe-22      1000000000    0.1266 ns/op
BenchmarkTypeSafeVsStringBased/StringBased-22   1000000000    0.7667 ns/op
```

## Available Parameter Structures

### JPEG 2000 Lossy: `JPEG2000LossyParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"

// Create with defaults
params := lossy.NewLossyParameters()
params.Quality = 95      // 1-100: compression quality
params.NumLevels = 5     // 0-6: wavelet decomposition levels
params.NumLayers = 1     // 1-N: quality layers (progressive)
params.TargetRatio = 0   // 0 or >1: target compression ratio

// Or use method chaining
params := lossy.NewLossyParameters().
    WithQuality(95).
    WithNumLevels(5).
    WithNumLayers(3).
    WithTargetRatio(8.0)

// Use with codec
codec := lossy.NewCodec(80)
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `Quality` (int, default: 80): Compression quality (1-100)
  - 100: Near-lossless (~2-3:1 compression, max error ≤1-2 pixels)
  - 95: Archival quality (~3:1 compression)
  - 85: High quality (~3-5:1 compression, visually lossless)
  - 75: Good quality (~5-7:1 compression)
  - 60: Medium quality (~7-10:1 compression, minor artifacts)
  - 50: Low quality (~10-15:1 compression, visible artifacts)
  - 20: High compression (~15-30:1, significant quality loss)
- `NumLevels` (int, default: 5): Wavelet decomposition levels (0-6)
  - More levels = better compression but slower encoding
  - Recommended: 1-3 for small images (<128x128), 5-6 for large images (>=512x512)
- `NumLayers` (int, default: 1): Number of quality layers for progressive refinement
  - 1: Single quality level (default, fastest)
  - 2-3: Progressive download/streaming
  - 4+: Fine-grained progressive refinement
  - See [Progressive Encoding Guide](jpeg2000/USAGE_GUIDE.md#progressivemulti-layer-encoding)
- `TargetRatio` (float64, default: 0): Target compression ratio (original_size / compressed_size)
  - 0: Use Quality parameter (default)
  - >1: Target specific compression ratio (e.g., 5.0 for 5:1 compression)
  - Actual ratio may vary ±5-20% depending on image complexity
  - Uses PCRD (Post-Compression Rate-Distortion) optimization
- `QuantStepScale` (float64, default: 1.0): Global quantization step multiplier (advanced)
  - 1.0: Normal quantization
  - >1.0: Coarser quantization (more compression, lower quality)
  - <1.0: Finer quantization (less compression, higher quality)
- `SubbandSteps` ([]float64, default: nil): Custom per-subband quantization steps (advanced)
  - Length must be exactly `3*NumLevels + 1`
  - For expert users who need precise control over quantization

### JPEG 2000 Lossless: `JPEG2000LosslessParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"

// Create with defaults
params := lossless.NewLosslessParameters()
params.NumLevels = 5     // 0-6: wavelet decomposition levels

// Or use method chaining
params := lossless.NewLosslessParameters().
    WithNumLevels(5)

// Use with codec
codec := lossless.NewCodec()
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `NumLevels` (int, default: 5): Wavelet decomposition levels (0-6)
  - More levels = better compression but more computation
  - Recommended: 1-3 for small images (<128x128), 5-6 for large images (>=512x512)

### JPEG Baseline: `JPEGBaselineParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpeg/baseline"

// Create with defaults
params := baseline.NewBaselineParameters()
params.Quality = 85      // 1-100: JPEG quality

// Or use method chaining
params := baseline.NewBaselineParameters().
    WithQuality(90)

// Use with codec
codec := baseline.NewBaselineCodec(85)
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `Quality` (int, default: 85): JPEG compression quality (1-100)
  - 100: Best quality
  - 85: High quality (default)
  - 75: Medium quality
  - 50: Lower quality

### JPEG Extended: `JPEGExtendedParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpeg/extended"

// Create with defaults
params := extended.NewExtendedParameters()
params.Quality = 85      // 1-100: JPEG quality
params.BitDepth = 12     // 8 or 12: bits per sample

// Or use method chaining
params := extended.NewExtendedParameters().
    WithQuality(90).
    WithBitDepth(12)

// Use with codec
codec := extended.NewExtendedCodec(12, 85)
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `Quality` (int, default: 85): JPEG compression quality (1-100)
- `BitDepth` (int, default: 12): Bits per sample (8 or 12)
  - 8: Standard 8-bit encoding
  - 12: Extended 12-bit encoding (main feature)

### JPEG Lossless: `JPEGLosslessParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpeg/lossless"

// Create with defaults
params := lossless.NewLosslessParameters()
params.Predictor = 1     // 0-7: predictor algorithm

// Or use method chaining
params := lossless.NewLosslessParameters().
    WithPredictor(1)

// Use with codec
codec := lossless.NewLosslessCodec(0)
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `Predictor` (int, default: 0): Prediction algorithm (0-7)
  - 0: Auto-select (default)
  - 1: No prediction (A) - recommended for medical imaging
  - 2: Horizontal prediction (A)
  - 3: Vertical prediction (B)
  - 4-7: Other prediction modes

### JPEG-LS Near-Lossless: `JPEGLSNearLosslessParameters`

```go
import "github.com/cocosip/go-dicom-codec/jpegls/nearlossless"

// Create with defaults
params := nearlossless.NewNearLosslessParameters()
params.NEAR = 2          // 0-255: maximum error per pixel

// Or use method chaining
params := nearlossless.NewNearLosslessParameters().
    WithNEAR(3)

// Use with codec
codec := nearlossless.NewJPEGLSNearLosslessCodec(2)
err := codec.Encode(src, dst, params)
```

**Parameters:**
- `NEAR` (int, default: 2): Maximum allowed error per pixel (0-255)
  - 0: Lossless (perfect reconstruction)
  - 1: Max error ±1 per pixel
  - 2: Max error ±2 per pixel (default)
  - 3-5: Low error, good compression
  - 10: Medium error, higher compression
  - 20+: High error, maximum compression

## Usage Patterns

### Pattern 1: Direct Field Access (Recommended)

Most convenient for simple cases:

```go
params := lossy.NewLossyParameters()
params.Quality = 95
params.NumLevels = 5

err := codec.Encode(src, dst, params)
```

### Pattern 2: Method Chaining

Clean and fluent for configuration:

```go
params := lossy.NewLossyParameters().
    WithQuality(95).
    WithNumLevels(5)

err := codec.Encode(src, dst, params)
```

### Pattern 3: Builder Pattern

For complex configurations:

```go
params := lossy.NewLossyParameters()

if highQuality {
    params.WithQuality(95)
} else {
    params.WithQuality(50)
}

params.WithNumLevels(calculateLevels(imageSize))

err := codec.Encode(src, dst, params)
```

### Pattern 4: Validation

Always validated automatically, but you can validate explicitly:

```go
params := lossy.NewLossyParameters()
params.Quality = 150  // Out of range

params.Validate()     // Resets to default (80)
```

## Backward Compatibility

The type-safe structures fully implement `codec.Parameters`, so old string-based code still works:

```go
// Old code still works
var params codec.Parameters = lossy.NewLossyParameters()
params.SetParameter("quality", 90)
quality := params.GetParameter("quality").(int)

// New code is better
params := lossy.NewLossyParameters()
params.Quality = 90
quality := params.Quality  // No type assertion!
```

## Migration Guide

### Before

```go
// Old string-based approach
func encodeImage(src *codec.PixelData) error {
    codec := lossy.NewCodec(80)

    // Had to remember parameter names
    params := someParameterFactory()
    params.SetParameter("quality", 95)
    params.SetParameter("numLevels", 5)  // What is this?

    // Type assertion needed
    q := params.GetParameter("quality").(int)

    return codec.Encode(src, dst, params)
}
```

### After

```go
// New type-safe approach
func encodeImage(src *codec.PixelData) error {
    codec := lossy.NewCodec(80)

    // IDE shows all available parameters!
    params := lossy.NewLossyParameters()
    params.Quality = 95      // Autocomplete!
    params.NumLevels = 5     // Documented inline!

    // No type assertion!
    q := params.Quality

    return codec.Encode(src, dst, params)
}
```

## Best Practices

### ✅ DO

```go
// Use direct field access for clarity
params.Quality = 95

// Use chaining for fluent configuration
params := NewLossyParameters().WithQuality(95).WithNumLevels(5)

// Rely on defaults
params := NewLossyParameters()  // Uses good defaults

// Pass typed parameters to codec
codec.Encode(src, dst, params)
```

### ❌ DON'T

```go
// Don't use string-based access for new code
params.SetParameter("quality", 95)  // Old way

// Don't skip validation for user input
params.Quality = userInput  // Dangerous!
// Do this instead:
params.Quality = userInput
params.Validate()  // Ensures valid range

// Don't hardcode magic numbers
params.Quality = 95  // ❌ What does 95 mean?
// Do this instead:
const ARCHIVAL_QUALITY = 95
params.Quality = ARCHIVAL_QUALITY  // ✅ Clear intent
```

## IDE Support

All modern Go IDEs provide:

1. **Autocomplete**: Type `params.` and see all available parameters
2. **Type Checking**: Catch errors at compile time
3. **Documentation**: Hover over fields to see descriptions
4. **Refactoring**: Rename fields safely across codebase
5. **Go to Definition**: Jump to parameter definitions

## Custom Parameters

For codec extensions, you can still use custom parameters:

```go
params := lossy.NewLossyParameters()

// Standard parameters
params.Quality = 95

// Custom extension parameters
params.SetParameter("vendorSpecific", customValue)
```

## Examples

See codec-specific documentation:
- [JPEG 2000 Lossy Parameters](jpeg2000/lossy/README.md)
- [JPEG Baseline Parameters](jpeg/baseline/README.md)

## Summary

| Aspect | String-Based | Type-Safe |
|--------|-------------|-----------|
| Type Safety | ❌ Runtime only | ✅ Compile-time |
| IDE Autocomplete | ❌ No | ✅ Yes |
| Performance | Slower (~0.77 ns/op) | **6x faster** (~0.13 ns/op) |
| Error Prone | ❌ Easy to mistype | ✅ Compile-time catch |
| Documentation | ❌ External docs needed | ✅ Self-documenting |
| Backward Compatible | N/A | ✅ Yes |

**Recommendation**: Use type-safe parameters for all new code. Old string-based code continues to work.
