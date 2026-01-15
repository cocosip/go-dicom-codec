# JPEG2000 Encoder Comparison Tool

This directory contains tools for comparing JPEG2000 encoding results between OpenJPEG (C++) and the Go implementation.

## Overview

The comparison suite consists of three main components:

1. **OpenJPEG Encoder** (`cpp/openjpeg_encoder.cpp`) - C++ encoder using OpenJPEG library
2. **Go Encoder** (`go_encoder.go`) - Go encoder using go-dicom-codec
3. **Comparison Tool** (`compare_j2k.go`) - Analyzes differences between encoded outputs

## Prerequisites

### For C++ (OpenJPEG):
- C++ compiler (g++, clang++, or MSVC)
- OpenJPEG library source code (included in ../fo-dicom-codec-code/Native/Common/OpenJPEG)

### For Go:
- Go 1.18 or later
- go-dicom-codec library

## Building

### Build OpenJPEG Encoder (Windows)

```bash
cd comparison/cpp
g++ -o openjpeg_encoder.exe openjpeg_encoder.cpp -I../../fo-dicom-codec-code/Native/Common/OpenJPEG ../../fo-dicom-codec-code/Native/Common/OpenJPEG/*.c -lm -lpthread -lstdc++
```

### Build OpenJPEG Encoder (Linux/Mac)

```bash
cd comparison/cpp
g++ -o openjpeg_encoder openjpeg_encoder.cpp -I../../fo-dicom-codec-code/Native/Common/OpenJPEG ../../fo-dicom-codec-code/Native/Common/OpenJPEG/*.c -lm -lpthread
```

### Build Go Programs

No build required - use `go run` or build with:

```bash
cd comparison
go build go_encoder.go
go build compare_j2k.go
```

## Usage

### Step 1: Determine Image Parameters

First, you need to know the parameters of your raw pixel file:
- Width (pixels)
- Height (pixels)
- Components (1=grayscale, 3=RGB)
- Bit depth (8, 12, 16, etc.)
- Signed/unsigned (0=unsigned, 1=signed)

For the example `D:\1_raw_pixels.bin` (797KB):
- If RGB 8-bit: 512x512x3 = 786,432 bytes ≈ 797KB ✓

### Step 2: Encode with OpenJPEG

```bash
# Windows
cpp\openjpeg_encoder.exe D:\1_raw_pixels.bin output_openjpeg.j2k 512 512 3 8 0

# Linux/Mac
cpp/openjpeg_encoder D:/1_raw_pixels.bin output_openjpeg.j2k 512 512 3 8 0
```

Parameters:
1. Input file path
2. Output file path
3. Width
4. Height
5. Number of components
6. Bit depth
7. Signed (0=no, 1=yes)

### Step 3: Encode with Go

```bash
go run go_encoder.go D:\1_raw_pixels.bin output_go.j2k 512 512 3 8 0
```

### Step 4: Compare Results

```bash
# Basic comparison
go run compare_j2k.go output_openjpeg.j2k output_go.j2k

# Detailed comparison (shows byte-level differences)
go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed
```

## Automated Comparison Script

For convenience, use the provided script to run all steps:

### Windows

```bash
run_comparison.bat D:\1_raw_pixels.bin 512 512 3 8 0
```

### Linux/Mac

```bash
chmod +x run_comparison.sh
./run_comparison.sh D:/1_raw_pixels.bin 512 512 3 8 0
```

## Understanding the Output

The comparison tool provides:

### File Size Analysis
- Compressed size comparison
- Compression ratio differences

### Marker Sequence Comparison
- JPEG2000 marker structure
- Marker presence/absence
- Marker size differences

### Detailed Marker Analysis (with `detailed` flag)
- Byte-by-byte marker payload comparison
- Identifies specific differences in headers

### Decoding Verification
- Attempts to decode both outputs
- Compares decoded pixels
- Reports pixel-level differences

## Example Output

```
JPEG2000 Comparison Tool
============================================================

File Sizes:
  OpenJPEG: 245,678 bytes
  Go:       247,123 bytes
  Difference: 1,445 bytes (0.59%)

============================================================
Parsing OpenJPEG codestream...
  Total markers found: 8
  Compressed data size: 245,123 bytes

Parsing Go codestream...
  Total markers found: 8
  Compressed data size: 246,568 bytes

============================================================
MARKER SEQUENCE COMPARISON
============================================================

Idx   OpenJPEG                                 Go                                       Match
----------------------------------------------------------------------------------------------------
0     SOC (Start of Codestream) (0 bytes)     SOC (Start of Codestream) (0 bytes)     ✓
1     SIZ (Image and tile size) (47 bytes)    SIZ (Image and tile size) (47 bytes)    ✓
2     COD (Coding style default) (12 bytes)   COD (Coding style default) (12 bytes)   ✓
...
```

## Troubleshooting

### OpenJPEG Compilation Issues

If you encounter compilation errors:

1. Check that OpenJPEG source files exist in `../fo-dicom-codec-code/Native/Common/OpenJPEG`
2. Ensure you have a C++11 compatible compiler
3. Try compiling individual `.c` files first to identify problematic files

### Image Parameter Mismatch

If encoders fail with size mismatches:

1. Verify file size: `file_size = width × height × components × bytes_per_sample`
2. For 8-bit: bytes_per_sample = 1
3. For >8-bit: bytes_per_sample = 2

Common sizes:
- 512×512 grayscale 8-bit = 262,144 bytes
- 512×512 RGB 8-bit = 786,432 bytes
- 1024×1024 grayscale 16-bit = 2,097,152 bytes

### Decoding Failures

If comparison tool can't decode:
1. Check if encoded files are valid JPEG2000
2. Verify both encoders completed successfully
3. Try decoding with external tools (opj_decompress, GDAL, etc.)

## Notes

- Both encoders use lossless compression (5/3 reversible wavelet)
- Code-block size: 64×64
- Decomposition levels: 5
- Single tile (no tiling)
- LRCP progression order
- Both should produce similar (but not necessarily identical) results

## Advanced Usage

### Custom Encoding Parameters

Modify the source files to adjust encoding parameters:

**OpenJPEG** (`cpp/openjpeg_encoder.cpp`, line ~155):
```cpp
parameters.numresolution = 6;  // Change decomposition levels
parameters.cblockw_init = 64;   // Change code-block width
parameters.cblockh_init = 64;   // Change code-block height
```

**Go** (`go_encoder.go`, line ~65):
```go
NumLevels:       5,    // Change decomposition levels
CodeBlockWidth:  64,   // Change code-block width
CodeBlockHeight: 64,   // Change code-block height
```

## License

Same as parent project.
