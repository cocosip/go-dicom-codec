# 16-bit Image Support Issues

## Issue 1: JPEG Lossless SV1 - Limited Huffman Table Support

### Status
🔴 **BLOCKED** - Requires extended Huffman tables

### Description
JPEG Lossless SV1 codec fails to correctly encode/decode 16-bit medical images because the standard DC Huffman tables only support category 0-11 (value range: [-2047, 2047]).

For 16-bit data:
- Value range: [0, 65535]
- Difference range: [-65535, 65535]
- Required category: up to 16
- Current maximum category: 11

### Example
```
Original pixel: 590
Encoded diff: -32178 (590 - 32768)
Required category: 16
Available categories: 0-11
Result: Encoding fails or produces corrupted data
```

### Root Cause
Located in `jpeg/common/tables.go`:
```go
// StandardDCLuminanceValues contains the Huffman values (DC luminance)
var StandardDCLuminanceValues = []byte{
    0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,  // Only categories 0-11
}
```

For 16-bit support, need categories 0-15 (or 0-16).

### Files Affected
- `jpeg/lossless14sv1/encoder.go` - Line 269: encodes with standard tables
- `jpeg/lossless14sv1/decoder.go` - Line 309: decodes with standard tables
- `jpeg/common/tables.go` - Lines 60-78: defines standard Huffman tables
- `jpeg/lossless14sv1/lossless14sv1_test.go` - Line 138: Test skipped with TODO comment

### Test Results
```bash
cd test_jpeg_lossless && go run test_16bit_simple.go

Original: 590, 591, 592, ...
Decoded:  32768, 32768, 32767, ...  ❌ WRONG
```

### Solution Required
1. Create extended DC Huffman tables supporting category 0-15/16
2. Use extended tables for precision > 12 bits
3. Write appropriate DHT markers in JPEG stream
4. Update decoder to handle extended tables

### Workaround
For 16-bit medical images, use:
- ✅ JPEG 2000 Lossless (works perfectly)
- ✅ JPEG 2000 Lossy (works perfectly)
- ✅ JPEG-LS Lossless (if Issue 2 is fixed)

### Related
- See existing TODO comment in `jpeg/lossless14sv1/lossless14sv1_test.go:139`
- JPEG standard ITU-T T.81 Section F.1.2.1.1

---

## Issue 2: JPEG-LS Lossless - Large Image Decoding Errors

### Status
🔴 **CRITICAL BUG** - Produces corrupted output for large images

### Description
JPEG-LS Lossless codec produces corrupted pixel data when encoding/decoding large 16-bit images. Small test images (4×4) work perfectly, but realistic medical image sizes (888×459) produce massive pixel errors.

### Test Results

**Small image (4×4 pixels):**
```bash
cd test_jpeg_lossless && go run test_jpegls_16bit.go
✓ Perfect reconstruction!  # All pixels correct
```

**Large image (888×459 pixels = 407,592 pixels):**
```bash
cd test_jpeg_lossless && go run test_jpegls_large.go
First error at pixel 600: original=590, decoded=594
✗ Total errors: 788,442 bytes  # 96.7% of data corrupted!
```

**Real DICOM file (888×459 CT scan):**
```bash
./verify.exe "D:\1_transcoded\1_jpegls_lossless.dcm"
✗ Decompression failed: failed to decode frame: JPEG-LS Lossless decode failed: EOF
```

### Root Cause
**Unknown** - Likely one of:
1. **Scanline boundary bug**: Errors start around pixel 600, suggesting row/column boundary handling issue
2. **Context model corruption**: Context states not properly maintained across scanlines
3. **Run mode bug**: Run-length encoding breaks on larger images
4. **Buffer overflow**: Internal buffer sizing incorrect for large images

### Files Affected
- `jpegls/lossless/encoder.go` - Line 202-250: `encodeComponent()` method
- `jpegls/lossless/decoder.go` - Corresponding decode logic
- `jpegls/lossless/context.go` - Context model state management
- `jpegls/lossless/runmode.go` - Run-mode encoding/decoding

### Impact
- ❌ **Cannot be used for medical imaging** (typical CT/MR images are 512×512 or larger)
- ✅ Works for small test images only
- 🔴 **Silently produces corrupted data** (no error during encode/decode, but wrong pixel values)

### Next Steps
1. Add comprehensive logging to identify where first pixel corruption occurs
2. Compare encoder/decoder behavior on 4×4 vs 888×459 images
3. Test intermediate sizes (64×64, 128×128, 256×256) to find threshold
4. Verify scanline boundary calculations and context resets
5. Compare with reference JPEG-LS implementation (CharLS, libjpeg-ls)

---

## Working Codecs for 16-bit Images

✅ **JPEG 2000 Lossless** (1.2.840.10008.1.2.4.90)
- Encoding: ✓ Success
- Decoding: ✓ Success
- Pixel accuracy: ✓ Perfect

✅ **JPEG 2000 Lossy** (1.2.840.10008.1.2.4.91)
- Encoding: ✓ Success
- Decoding: ✓ Success
- Pixel accuracy: ✓ Within expected lossy range

---

## Impact on DICOM Transcoder

Current status of `examples/dicom_transcoder`:

**Supported formats for 16-bit CT/MR images:**
- ✅ JPEG 2000 Lossless
- ✅ JPEG 2000 Lossy
- ⊘ JPEG Baseline (intentionally skipped - 8-bit only)
- ❌ JPEG Lossless SV1 (produces corrupted data)
- ❌ JPEG-LS Lossless (decode fails)

**Success rate:** 2/5 formats (40%)

---

## Test Environment

- Test file: `D:\1.dcm`
- Image properties:
  - Dimensions: 888×459
  - Bits Stored: 16
  - Modality: CT
  - Original Transfer Syntax: 1.2.840.10008.1.2 (Implicit VR Little Endian)
  - Photometric Interpretation: MONOCHROME2
  - Sample pixel values: 590, 591, 592, ...

---

## Priority

**High Priority** - 16-bit support is critical for medical imaging:
- CT scans typically use 12-16 bits
- MR scans typically use 12-16 bits
- X-rays may use 10-16 bits
- Only JPEG 2000 currently works for these modalities
