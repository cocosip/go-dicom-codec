# JPEG2000 Encoder Comparison - Example Usage

This guide demonstrates how to use the comparison tools with the `D:\1_raw_pixels.bin` file.

## Step 1: Detect Image Parameters

First, use the parameter detection tool to determine possible image configurations:

```bash
cd D:\Code\go\go-dicom-codec\comparison
go run detect_params.go D:\1_raw_pixels.bin
```

**Output for D:\1_raw_pixels.bin:**
- File size: 815,184 bytes (796 KB)
- This is a non-standard size that doesn't match common image dimensions

## Step 2: Determine Actual Parameters

Based on the file size (815,184 bytes), we need to determine the actual parameters. Here are some possibilities:

### Option 1: Custom Grayscale Image
```
918 × 888 × 1 (grayscale) × 1 byte = 815,184 bytes ✓
```
Command parameters: `918 888 1 8 0`

### Option 2: Custom RGB Image (if applicable)
```
522 × 521 × 3 (RGB) × 1 byte = 816,006 bytes (close, but not exact)
```

### Option 3: Check for Metadata
The file might contain metadata or padding. Let's use the most likely configuration.

## Step 3: Run Comparison

### Using Windows Batch Script

```batch
cd D:\Code\go\go-dicom-codec\comparison
run_comparison.bat D:\1_raw_pixels.bin 918 888 1 8 0
```

### Using Individual Commands

#### 3a. Build OpenJPEG Encoder (first time only)

```bash
cd D:\Code\go\go-dicom-codec\comparison\cpp
g++ -o openjpeg_encoder.exe openjpeg_encoder.cpp ^
  -I..\..\fo-dicom-codec-code\Native\Common\OpenJPEG ^
  ..\..\fo-dicom-codec-code\Native\Common\OpenJPEG\*.c ^
  -lm -lpthread -lstdc++
```

#### 3b. Encode with OpenJPEG

```bash
cd D:\Code\go\go-dicom-codec\comparison
cpp\openjpeg_encoder.exe D:\1_raw_pixels.bin output_openjpeg.j2k 918 888 1 8 0
```

**Expected Output:**
```
OpenJPEG Encoder Configuration:
  Input: D:\1_raw_pixels.bin
  Output: output_openjpeg.j2k
  Dimensions: 918x888
  Components: 1
  Bit depth: 8
  Signed: no
  File size: 815184 bytes
  Expected size: 815184 bytes

Encoding...
Encoding completed successfully!
Output file: output_openjpeg.j2k
```

#### 3c. Encode with Go

```bash
go run go_encoder.go D:\1_raw_pixels.bin output_go.j2k 918 888 1 8 0
```

**Expected Output:**
```
Go JPEG2000 Encoder Configuration:
  Input: D:\1_raw_pixels.bin
  Output: output_go.j2k
  Dimensions: 918x888
  Components: 1
  Bit depth: 8
  Signed: false
  File size: 815184 bytes
  Expected size: 815184 bytes

Converting raw data to components...
Encoding...
Encoding completed successfully!
Output file: output_go.j2k
Output size: XXXXX bytes
Compression ratio: X.XX:1
```

#### 3d. Compare Results

```bash
go run compare_j2k.go output_openjpeg.j2k output_go.j2k
```

**Expected Output:**
```
JPEG2000 Comparison Tool
============================================================

File Sizes:
  OpenJPEG: XXXXX bytes
  Go:       XXXXX bytes
  Difference: XXX bytes (X.XX%)

------------------------------------------------------------
Parsing OpenJPEG codestream...
  Total markers found: X
  Compressed data size: XXXXX bytes

Parsing Go codestream...
  Total markers found: X
  Compressed data size: XXXXX bytes

============================================================
MARKER SEQUENCE COMPARISON
============================================================

Idx   OpenJPEG                                 Go                                       Match
----------------------------------------------------------------------------------------------------
0     SOC (Start of Codestream) (0 bytes)     SOC (Start of Codestream) (0 bytes)     ✓
1     SIZ (Image and tile size) (XX bytes)    SIZ (Image and tile size) (XX bytes)    ✓
...
```

#### 3e. Detailed Comparison

```bash
go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed > comparison_detailed.txt
```

This creates a detailed report in `comparison_detailed.txt` showing:
- Byte-by-byte marker comparison
- Specific differences in marker payloads
- Pixel-level comparison after decoding

## Step 4: Analyze Results

### Key Metrics to Compare:

1. **File Size Difference**
   - Both should produce lossless compression
   - Small differences (< 5%) are acceptable
   - Large differences may indicate implementation issues

2. **Marker Sequence**
   - Should have identical marker types in the same order
   - Marker sizes may differ slightly due to encoding choices

3. **Decoded Pixels**
   - **Critical**: Decoded pixels should be IDENTICAL for lossless compression
   - Any pixel differences indicate a bug in one or both encoders

### Example Analysis:

```
File Sizes:
  OpenJPEG: 245,678 bytes
  Go:       247,123 bytes
  Difference: 1,445 bytes (0.59%)    <-- Small difference is OK

Marker sequence: 8/8 matched (100%)   <-- Perfect match is ideal

Decoded pixels: ✓ IDENTICAL           <-- MUST be identical for lossless!
```

## Troubleshooting

### Issue: "File size mismatch"

If you see a size mismatch warning:
```
Warning: File size (815184) is smaller than expected (XXXXXX)
```

Your parameters are likely incorrect. Try:
1. Use `detect_params.go` to find possible configurations
2. Check if the file has metadata/header (inspect first few bytes)
3. Try different dimension combinations

### Issue: "Failed to decode"

If decoding fails for either output:
1. Check that encoding completed successfully
2. Verify the encoded files are valid JPEG2000 (use external tools)
3. Check for error messages during encoding

### Issue: "Pixel differences found"

If decoded pixels differ (for lossless encoding):
```
✗ Pixel differences found: 1234/815184 (0.15%)
```

This indicates a BUG! Check:
1. Encoding parameters match between both encoders
2. Review encoding logs for errors
3. Compare marker payloads in detailed report
4. File a bug report with reproduction steps

## Expected Results

For **lossless** encoding (which both encoders use by default):

- ✓ File sizes should be similar (within ~5%)
- ✓ Marker sequences should match
- ✓ **Decoded pixels MUST be identical** (0 differences)
- ~ Compression ratios will be similar (typically 2:1 to 4:1 for medical images)

Any deviation from these expectations should be investigated!

## Next Steps

1. If results match: Both encoders are working correctly ✓
2. If results differ: Investigate differences using detailed comparison
3. Try different images: Test with various sizes and bit depths
4. Performance testing: Use `time` or benchmarking tools to compare speed

## Additional Testing

### Test Different Parameters

```bash
# Test RGB image (if you have one)
run_comparison.bat input_rgb.bin output 512 512 3 8 0

# Test 16-bit grayscale
run_comparison.bat input_16bit.bin output 512 512 1 16 0

# Test signed data
run_comparison.bat input_signed.bin output 512 512 1 12 1
```

### Decode with External Tools

Verify both outputs can be decoded by other tools:

```bash
# Using OpenJPEG opj_decompress (if installed)
opj_decompress -i output_openjpeg.j2k -o decoded_opj.raw
opj_decompress -i output_go.j2k -o decoded_go.raw

# Compare decoded outputs
fc /b decoded_opj.raw decoded_go.raw  # Windows
diff decoded_opj.raw decoded_go.raw   # Linux/Mac
```

## Support

If you encounter issues:
1. Check the main [README.md](README.md) for detailed documentation
2. Verify all prerequisites are installed
3. Review error messages carefully
4. Check file paths and permissions
