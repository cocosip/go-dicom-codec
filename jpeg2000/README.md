# JPEG 2000 Codec - Pure Go Implementation

Pure Go implementation of JPEG 2000 Part 1 (ISO/IEC 15444-1) encoder and decoder for medical imaging (DICOM).

## Status

Production-ready encoder/decoder (lossless & lossy) with multi-quality-layer support.

- Encoder: Complete (lossless & lossy), supports 1-N layers for progressive quality
- Decoder: Complete (lossless & lossy), handles layered codestreams
- Codestream parser and generator
- 5/3 reversible wavelet transform (lossless, multi-level)
- 9/7 irreversible wavelet transform (lossy, multi-level)
- MQ arithmetic encoder/decoder
- EBCOT Tier-1 encoder/decoder
- EBCOT Tier-2 packet encoding/decoding
- Tag tree implementation (ISO/IEC 15444-1 B.10.2)
- DC Level Shift
- Byte-stuffing handling
- Lossless: Perfect reconstruction (0 pixel errors)
- Lossy: High quality compression (typical 1-2 pixel max error for 64x64+ images)

## Features

### Supported

- Encoding and Decoding: Both directions fully supported
- Lossless compression: 5/3 reversible wavelet transform (perfect reconstruction)
- Lossy compression: 9/7 irreversible wavelet transform (high quality)
- Quality layers: Progressive quality with 1-N layers (LRCP/RLCP progression)
- Image formats: Grayscale (1 component), RGB (3 components)
- Bit depths: 8-bit and 16-bit
- Image sizes: All sizes from 8x8 to 1024x1024 and beyond (tested up to 1024x1024)
- Wavelet levels: 0-6 decomposition levels
- Tiling: single and multi-tile codestreams
- ROI coding: **Full support** - Multiple ROI regions, MaxShift & General Scaling styles, Rectangle/Polygon/Mask shapes, Main header & Tile-part RGN, COM marker metadata
- Transfer Syntax:
  - 1.2.840.10008.1.2.4.90 (JPEG 2000 Lossless)
  - 1.2.840.10008.1.2.4.91 (JPEG 2000 Lossy)
  - 1.2.840.10008.1.2.4.92 (JPEG 2000 Part 2 Multi-component Lossless)
  - 1.2.840.10008.1.2.4.93 (JPEG 2000 Part 2 Multi-component)
  - 1.2.840.10008.1.2.4.201 / .202 / .203 (HTJ2K) - registered, encode/decode pending
- Compression ratio:
  - Lossless: 5.5:1 to 6.8:1 for medical images
  - Lossy: 3:1+ (configurable)

### Quality Validation

- All roundtrip tests passing

**Lossless (5/3 wavelet):**
- Image sizes: 64��64, 128��128, 192��192, 256��256, 512��512, 1024��1024
- Wavelet levels: 0, 1, 2, 3 levels
- Test patterns: Gradients, uniform values, solid colors
- **0 pixel errors** (perfect reconstruction)

**Lossy (9/7 wavelet):**
- Image sizes: 16��16, 64��64, 32��32 RGB
- Wavelet levels: 1, 5 levels
- Max error: 1-2 pixels (64��64+), up to 163 pixels (16��16 due to boundary effects)
- Average error: < 1 pixel (64��64+), ~5 pixels (16��16)
- Compression ratio: ~3:1 for typical medical images

### Recently Implemented

- Multiple quality layers (2025-01-27)
  - Progressive quality encoding with 1-N layers
  - Decoding handles layered codestreams for progressive display
  - Simple layer allocation algorithm for balanced quality distribution
  - Compatible with LRCP and RLCP progression orders
  - Automatic pass distribution across layers
- **ROI (Region of Interest) - Complete Implementation** (2025-12-02)
  - Multiple ROI regions with per-component support
  - Two ROI styles: MaxShift (Srgn=0) and General Scaling (Srgn=1)
  - Three ROI shapes: Rectangle, Polygon, Mask (bitmap)
  - Main header RGN and Tile-part header RGN support
  - COM marker for automatic ROI metadata transmission (rectangles/polygons)
  - Mask downsampling with caching optimization
  - Full encode/decode support with 63 passing tests

### Not Yet Implemented
- Precincts
- Other progression orders (RPCL, PCRL, CPRL supported in spec but not yet tested)
- HTJ2K encode/decode (1.2.840.10008.1.2.4.201/.202/.203) pending implementation

## Installation

```
jpeg2000/
- codestream/      # JPEG 2000 marker and segment parsing
- wavelet/         # 5/3 reversible & 9/7 irreversible wavelet transforms
- mqc/             # MQ arithmetic encoder/decoder
- t1/              # EBCOT Tier-1 encoder/decoder (bit-plane coding)
- t2/              # EBCOT Tier-2 (packet encoding/decoding)
- lossless/        # Lossless codec (1.2.840.10008.1.2.4.90)
- lossy/           # Lossy codec (1.2.840.10008.1.2.4.91)
- testdata/        # Test data generator
- encoder.go       # Main encoder API
- decoder.go       # Main decoder API
```

### Key Components

- **Codestream Parser/Generator**: Parses and generates JPEG 2000 markers (SOC, SIZ, COD, QCD, SOT, SOD, EOC)
- **Wavelet Transform**:
  - 5/3 reversible integer wavelet (DWT53) for lossless compression
  - 9/7 irreversible floating-point wavelet (DWT97) for lossy compression
  - Multi-level decomposition (0-6 levels)
- **MQ Encoder/Decoder**: Arithmetic coder with 47-state probability model and context modeling
- **EBCOT Tier-1**: Context-based bit-plane coding with 19 contexts (3 passes: SPP, MRP, CP)
- **EBCOT Tier-2**: Packet encoding/decoding with tag trees and layer progression
- **Tile Encoder/Decoder**: Assembles and disassembles components with proper subband layout
- **effectiveBitDepth**: Automatic adjustment for wavelet coefficient range expansion (bitDepth + numLevels)

### Important Technical Notes

**Wavelet Coefficient Bit Depth:**

The 5/3 reversible wavelet transform increases the dynamic range of coefficients. After each decomposition level, coefficients can exceed the original data range by 1 bit. This implementation correctly handles this by using:

```
effectiveBitDepth = originalBitDepth + numDecompositionLevels
```

For example:
- 8-bit image with 1-level decomposition: effectiveBitDepth = 8 + 1 = 9 bits
- 8-bit image with 2-level decomposition: effectiveBitDepth = 8 + 2 = 10 bits

This ensures correct calculation of `zeroBitPlanes` and `maxBitplane` for all code blocks, which is critical for lossless reconstruction.

## Testing

```bash
# Run all tests
go test ./jpeg2000/...

# Run with coverage
go test -cover ./jpeg2000/...

# Run benchmarks
go test -bench=. ./jpeg2000

# Run specific test
go test ./jpeg2000/lossless -v
```

Test coverage:
- Unit tests: 100+ tests across all modules
- Roundtrip tests: 12 size/level combinations (64��64 to 1024��1024)
- Integration tests: 15+ tests
- End-to-end tests: Multiple test suites with various patterns
- Benchmark tests: 11 benchmarks
- **Result**: All tests passing with 0 pixel errors

## Development Status

### Completed

**Core Implementation:**
- Codestream parser and generator (markers, segments, tiles)
- 5/3 reversible wavelet transform (forward/inverse, multi-level) - Lossless
- 9/7 irreversible wavelet transform (forward/inverse, multi-level) - Lossy
- MQ arithmetic encoder/decoder (47-state machine)
- EBCOT Tier-1 encoder/decoder (3 coding passes, 19 contexts)
- EBCOT Tier-2 packet encoding/decoding (tag trees, packet headers)
- DC level shift for unsigned/signed data conversion
- Byte-stuffing handling in compressed data

**API & Integration:**
- Main encoder/decoder API
- Lossless codec implementation (1.2.840.10008.1.2.4.90)
- Lossy codec implementation (1.2.840.10008.1.2.4.91)
- Global registry auto-registration
- Multi-component (RGB) support

**Quality Assurance:**
- Comprehensive test suite (100+ tests)
- Lossless: Perfect reconstruction for all test cases (0 pixel errors)
- Lossy: High quality compression (1-2 pixel max error for 64x64+ images)
- Performance benchmarks
- Validated on images up to 1024x1024
- All known issues resolved

**Recent Additions (2025):**
- Multi-quality-layer encoding/decoding (progressive layers, LRCP/RLCP)
- Implemented 9/7 irreversible wavelet transform for lossy compression
- Added lossy codec package with Transfer Syntax 1.2.840.10008.1.2.4.91
- Modified encoder/decoder to support both 5/3 and 9/7 wavelets
- Fixed DC level shift bug in encoder.Encode() method
- Comprehensive testing for both lossless and lossy modes
- Quality parameter for lossy compression (1-100 scale)
- Quantization with per-subband step sizes
- Dequantization in decoder for lossy mode
- Type-safe parameter structures (JPEG2000LossyParameters)
  - Compile-time checking and IDE autocomplete
  - Direct field access (no string keys needed)
  - Method chaining support
  - Backward compatible with generic codec.Parameters

### Planned Enhancements

- Additional progression orders (currently LRCP only)
- Performance optimizations (SIMD, parallel processing)
- Precinct partitioning

## Contributing

## Contributing

This is part of the `go-dicom-codec` project. See the main repository for contribution guidelines.

## References

- ISO/IEC 15444-1:2019 - JPEG 2000 Image Coding System, Part 1
- DICOM PS3.5 - Transfer Syntax Specifications
- OpenJPEG - Reference implementation
- ITU-T T.800 - JPEG 2000 Image Coding System

## License

Same as parent project `go-dicom-codec`.

## Acknowledgments

- Based on JPEG 2000 standard (ISO/IEC 15444-1)
- Reference implementation insights from OpenJPEG
- Part of the go-dicom ecosystem



















