# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**go-dicom-codec** is a Go library providing image compression/decompression codecs for medical imaging (DICOM). It supports multiple codec families:
- JPEG (Baseline, Extended, Lossless)
- JPEG-LS (planned)
- JPEG 2000 (planned)

**Important**: This library focuses solely on codec implementation (encoding/decoding). DICOM-specific concerns (encapsulation, fragmentation, metadata, transfer syntax management) are handled by external DICOM libraries.

## Common Commands

### Building
```bash
go build ./...
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test ./jpeg/baseline
go test ./jpeg/lossless14sv1

# Run benchmarks
go test -bench=. ./...
```

### Development
```bash
# Format code
go fmt ./...

# Run linter (requires golangci-lint)
golangci-lint run

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify
```

## Architecture

### Directory Structure
```
codec/              # Core codec interfaces and registry
jpeg/               # JPEG family codecs
  common/           # Shared JPEG utilities (Huffman, DCT, markers)
  baseline/         # JPEG Baseline (Process 1) - Lossy 8-bit
  lossless14sv1/    # JPEG Lossless SV1 (Process 14, Predictor 1)
jpegls/             # JPEG-LS codecs (planned)
jpeg2000/           # JPEG 2000 codecs (planned)
```

### Core Concepts

**Codec Interface** (`codec/codec.go`):
- All codecs implement the `Codec` interface
- Each codec has a unique UID (typically DICOM Transfer Syntax UID)
- Codecs are registered in a global registry
- Can be accessed by name or UID

**Codec Registry** (`codec/registry.go`):
- Thread-safe codec registration and retrieval
- Codecs auto-register on package import
- Supports lookup by name or UID

**Encoding Parameters** (`codec.EncodeParams`):
- PixelData: Raw pixel data (byte array)
- Width, Height: Image dimensions
- Components: 1=grayscale, 3=RGB
- BitDepth: Bits per sample (8, 12, 16, etc.)
- Options: Codec-specific options (quality, etc.)

**Decoding Result** (`codec.DecodeResult`):
- Returns decoded pixel data and metadata
- Includes dimensions, components, bit depth

### JPEG Implementation Details

**JPEG Baseline** (`jpeg/baseline/`):
- UID: 1.2.840.10008.1.2.4.50
- 8-bit lossy DCT-based compression
- Supports grayscale and RGB (converted to YCbCr)
- Quality parameter: 1-100
- Uses standard Huffman and quantization tables

**JPEG Lossless SV1** (`jpeg/lossless14sv1/`):
- UID: 1.2.840.10008.1.2.4.70
- Lossless prediction-based (Predictor 1: left pixel)
- Supports 2-16 bit depth
- Perfect reconstruction (0 errors)
- Uses standard DC Huffman tables

**Common Utilities** (`jpeg/common/`):
- `markers.go`: JPEG marker constants
- `reader.go/writer.go`: Bitstream I/O
- `huffman.go/huffman_encoder.go`: Huffman coding
- `dct.go/idct.go`: Fast integer DCT/IDCT
- `tables.go`: Standard quantization and Huffman tables
- `errors.go`: Common error definitions

### Key Implementation Notes

**JPEG Byte Stuffing**:
- Any 0xFF byte in scan data must be followed by 0x00
- Decoder must handle stuffed bytes correctly

**Color Space Conversion**:
- RGB → YCbCr for baseline (with 4:2:0 subsampling)
- Proper scaling formulas for subsampled components

**Huffman Coding**:
- Standard tables defined in `jpeg/common/tables.go`
- Category-based encoding for DC/AC coefficients
- Bit packing with byte stuffing

**DCT/IDCT**:
- Fast integer implementation (no floating point)
- 8x8 block processing
- Proper rounding and clamping

## Testing Strategy

### Unit Tests
- Test each codec's encode/decode separately
- Test with various image sizes and bit depths
- Test edge cases (1x1 images, max dimensions, etc.)
- Validate error handling for invalid parameters

### Round-Trip Tests
- **Lossless codecs**: Perfect reconstruction (0 errors)
- **Lossy codecs**: Acceptable error bounds for given quality

### Integration Tests
- Verify compatibility with Go standard library `image/jpeg`
- Test interoperability with other JPEG implementations
- Validate against reference test images

### Benchmarks
- Measure encoding/decoding performance
- Track performance across different image sizes
- Identify optimization opportunities

### Test Data
- Grayscale: Gradient patterns, uniform values, random noise
- RGB: Color gradients, edges, medical imaging patterns
- Various dimensions: 64x64, 512x512, 1024x1024, etc.

## Adding New Codecs

To add a new codec (e.g., JPEG-LS):

1. **Create package structure**:
   ```
   jpegls/
     lossless/
       encoder.go
       decoder.go
       codec.go       # Implements codec.Codec interface
       options.go     # Codec-specific options
       lossless_test.go
   ```

2. **Implement codec.Codec interface**:
   ```go
   type Codec struct {}

   func (c *Codec) Encode(params codec.EncodeParams) ([]byte, error) { ... }
   func (c *Codec) Decode(data []byte) (*codec.DecodeResult, error) { ... }
   func (c *Codec) UID() string { return "1.2.840.10008.1.2.4.80" }
   func (c *Codec) Name() string { return "jpeg-ls-lossless" }
   ```

3. **Auto-register in init()**:
   ```go
   func init() {
       codec.Register(&Codec{})
   }
   ```

4. **Create comprehensive tests**:
   - Basic encode/decode round-trip
   - Various image formats and sizes
   - Edge cases and error handling
   - Benchmarks

## Development Guidelines

- **Pure Go**: No CGO dependencies
- **Performance**: Optimize hot paths, use benchmarks
- **Error Handling**: Return descriptive errors
- **Documentation**: Document public APIs
- **Testing**: Maintain high test coverage
- **Compatibility**: Follow DICOM standards strictly
- **Code Quality**: All code must pass golangci-lint without errors. Run `golangci-lint run` before committing changes

## Roadmap

See [TODO.md](TODO.md) for current development priorities and planned features.

## 源码目录

- OpenJPEG的源码在当前根目录下的 fo-dicom-codec-code/Native/Common/OpenJPEG

- OpenJPH的源码在当前根目录下的 fo-dicom-codec-code/Native/Common/OpenJPH
