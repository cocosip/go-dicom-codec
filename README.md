# go-dicom-codec

A Go library providing image compression/decompression codecs for medical imaging (DICOM), including JPEG, JPEG-LS, and JPEG 2000 families.

## Features

### JPEG Family
- ✅ **JPEG Baseline** (Process 1) - Lossy, 8-bit [1.2.840.10008.1.2.4.50]
- ✅ **JPEG Extended** (Process 2 & 4) - Lossy, 8/12-bit [1.2.840.10008.1.2.4.51]
- ✅ **JPEG Lossless** (Process 14, Selection 1) - All 7 predictors [1.2.840.10008.1.2.4.57]
- ✅ **JPEG Lossless SV1** (Process 14, Selection 1) - Predictor 1 only [1.2.840.10008.1.2.4.70]

### JPEG-LS Family (Planned)
- ⏳ **JPEG-LS Lossless** [1.2.840.10008.1.2.4.80]
- ⏳ **JPEG-LS Near-Lossless** [1.2.840.10008.1.2.4.81]

### JPEG 2000 Family (Planned)
- ⏳ **JPEG 2000 Lossless** [1.2.840.10008.1.2.4.90]
- ⏳ **JPEG 2000** (Lossy or Lossless) [1.2.840.10008.1.2.4.91]
- ⏳ **JPEG 2000 Multi-component Lossless** [1.2.840.10008.1.2.4.92]
- ⏳ **JPEG 2000 Multi-component** [1.2.840.10008.1.2.4.93]
- ⏳ **JPEG 2000 High-Throughput** variants [1.2.840.10008.1.2.4.201/202/203]

## Installation

```bash
go get github.com/cocosip/go-dicom-codec
```

## Architecture

The library is organized into the following packages:

- `codec/` - Core codec interfaces and registry
- `jpeg/` - JPEG family implementations
  - `jpeg/common/` - Shared utilities (Huffman, DCT, markers, etc.)
  - `jpeg/baseline/` - JPEG Baseline codec
  - `jpeg/lossless/` - JPEG Lossless codec (all 7 predictors)
  - `jpeg/lossless14sv1/` - JPEG Lossless SV1 codec (predictor 1 only)
- `jpegls/` - JPEG-LS implementations (planned)
- `jpeg2000/` - JPEG 2000 implementations (planned)

## Usage

### Using the Codec Registry

```go
package main

import (
    "github.com/cocosip/go-dicom-codec/codec"
    _ "github.com/cocosip/go-dicom-codec/jpeg/baseline" // Auto-register
)

func main() {
    // Get codec by UID
    c, err := codec.Get("1.2.840.10008.1.2.4.50")
    if err != nil {
        panic(err)
    }

    // Encode
    params := codec.EncodeParams{
        PixelData:  pixelData,
        Width:      512,
        Height:     512,
        Components: 1, // Grayscale
        BitDepth:   8,
        Options:    nil, // Use defaults
    }

    compressed, err := c.Encode(params)
    if err != nil {
        panic(err)
    }

    // Decode
    result, err := c.Decode(compressed)
    if err != nil {
        panic(err)
    }
}
```

### Direct Package Usage

```go
import "github.com/cocosip/go-dicom-codec/jpeg/baseline"

func main() {
    // Encode with quality 85
    jpegData, err := baseline.Encode(pixelData, width, height, components, bitDepth, 85)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := baseline.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG Lossless (All Predictors)

```go
import "github.com/cocosip/go-dicom-codec/jpeg/lossless"

func main() {
    // Lossless encoding with predictor 4 (best compression)
    predictor := 4 // 1-7, or 0 for auto-select
    jpegData, err := lossless.Encode(pixelData, width, height, components, bitDepth, predictor)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := lossless.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG Lossless SV1 (Predictor 1 only)

```go
import "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"

func main() {
    // Lossless encoding (perfect reconstruction)
    jpegData, err := lossless14sv1.Encode(pixelData, width, height, components, bitDepth)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := lossless14sv1.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

## Codec Details

### JPEG Baseline
- **UID**: 1.2.840.10008.1.2.4.50
- **Compression**: Lossy DCT-based
- **Bit Depth**: 8-bit
- **Color Spaces**: Grayscale, RGB (auto-converted to YCbCr)
- **Options**: Quality (1-100)
- **Typical Compression**: 4-10x (quality dependent)

### JPEG Lossless (All Predictors)
- **UID**: 1.2.840.10008.1.2.4.57
- **Compression**: Lossless prediction-based (7 predictors)
- **Bit Depth**: 2-16 bits (8-11 bit fully tested)
- **Color Spaces**: Grayscale, RGB
- **Predictors**:
  - Predictor 1 (Left): 1.90x compression
  - Predictor 2 (Above): 1.53x compression
  - Predictor 3 (Above-Left): 1.50x compression
  - **Predictor 4 (Ra+Rb-Rc): 3.64x compression** ⭐ **Recommended**
  - Predictor 5 (Adaptive): 1.91x compression
  - Predictor 6 (Adaptive): 1.89x compression
  - Predictor 7 (Average): 1.52x compression
- **Perfect Reconstruction**: Yes (0 errors)
- **Status**: ✅ Production ready

### JPEG Lossless SV1
- **UID**: 1.2.840.10008.1.2.4.70
- **Compression**: Lossless prediction-based (Predictor 1 only)
- **Bit Depth**: 2-16 bits
- **Color Spaces**: Grayscale, RGB
- **Typical Compression**: 1.90x
- **Perfect Reconstruction**: Yes (0 errors)
- **Status**: ✅ Production ready

## Performance

Benchmarks on 512x512 grayscale images:

- **JPEG Baseline** - Encode: ~1.17ms, Decode: ~2.97ms
- **JPEG Lossless** - Encode: ~12.5ms, Decode: ~8.3ms (predictor 1)
- **JPEG Lossless SV1** - Encode: ~3.65ms, Decode: ~40.2ms

## Examples

See the [examples/](examples/) directory for complete working examples:

- `all_codecs_example.go` - Comprehensive example using all three codecs
- `codec_usage.go` - Basic codec registry usage
- `complete_example.go` - Complete DICOM integration example

Run examples:
```bash
go run examples/all_codecs_example.go
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./...
```

## Note

This library focuses solely on codec implementation. DICOM-specific concerns (encapsulation, fragmentation, metadata) are handled by external DICOM libraries.

## Roadmap

See [TODO.md](TODO.md) for detailed development plans.

## License

MIT License

## Contributing

Contributions are welcome! Please submit issues or pull requests.
