# CharLS Complete Alignment Checklist

## CharLS Source Files Structure

### Core Algorithm Files
- [ ] **scan.h** - Main encoding/decoding logic
  - [ ] `jls_codec` class template
  - [ ] `do_line()` - Process scan line
  - [ ] `do_regular()` - Regular mode encoding/decoding
  - [ ] `do_run_mode()` - Run mode encoding/decoding
  - [ ] `encode_mapped_value()` - Golomb encoding
  - [ ] `decode_value()` - Golomb decoding
  - [ ] `get_predicted_value()` - MED predictor
  - [ ] `apply_sign()` - Sign symmetry
  - [ ] `map_error_value()` / `unmap_error_value()` - Error mapping
  - [ ] `compute_context_id()` - Context ID calculation

### Traits (Parameter Management)
- [ ] **default_traits.h** - Default implementation for near-lossless
  - [ ] `quantize()` - Quantize error
  - [ ] `dequantize()` - Dequantize error
  - [ ] `compute_error_value()` - Combined quantize + modulo
  - [ ] `compute_reconstructed_sample()` - Reconstruct pixel
  - [ ] `modulo_range()` - Error range reduction
  - [ ] `correct_prediction()` - Clamp prediction
  - [ ] `fix_reconstructed_value()` - Handle wraparound
  - [ ] `is_near()` - Check if values are within NEAR

- [ ] **lossless_traits.h** - Optimized for lossless (NEAR=0)
  - [ ] `lossless_traits_impl` - Base template
  - [ ] Specializations for 8-bit, 16-bit
  - [ ] Specializations for triplet/quad (RGB/RGBA)

### Context Management
- [ ] **context_regular_mode.h** - Regular mode context
  - [ ] `get_golomb_coding_parameter()` - Compute k
  - [ ] `get_error_correction()` - Error correction for k==0
  - [ ] `update_variables_and_bias()` - Update A, B, C, N
  - [ ] Context initialization
  - [ ] Reset threshold handling

- [ ] **context_run_mode.h** - Run mode context
  - [ ] Run length encoding
  - [ ] Run interruption handling
  - [ ] J table usage

### Coding Parameters
- [ ] **coding_parameters.h** - Frame and coding parameters
  - [ ] `frame_info` - Image dimensions, bit depth, components
  - [ ] `coding_parameters` - Interleave mode, transformation
  - [ ] `jpegls_pc_parameters` - Preset coding parameters (LSE marker)

- [ ] **constants.h** - Constants and thresholds
  - [ ] `default_reset_value` (64)
  - [ ] `max_k_value` (16)
  - [ ] Threshold calculations
  - [ ] Range parameter calculations

### Stream I/O
- [ ] **jpeg_stream_reader.h/cpp** - JPEG-LS stream reading
  - [ ] Marker reading (SOI, SOF55, LSE, SOS, EOI)
  - [ ] Frame header parsing
  - [ ] LSE marker handling
  - [ ] SOS marker parsing
  - [ ] Scan data reading with byte unstuffing

- [ ] **jpeg_stream_writer.h/cpp** - JPEG-LS stream writing
  - [ ] Marker writing
  - [ ] Frame header generation
  - [ ] LSE marker generation
  - [ ] SOS marker generation
  - [ ] Scan data writing with byte stuffing

### Bit Stream Operations
- [ ] Bit buffer management
  - [ ] `append_to_bit_stream()` - Write bits
  - [ ] `read_value()` - Read n bits
  - [ ] `read_high_bits()` - Read unary code
  - [ ] `peek_byte()` - Look ahead
  - [ ] `skip()` - Skip bits
  - [ ] Byte stuffing (0xFF → 0xFF 0x00)

### Lookup Tables
- [ ] **lookup_table.h** - Gradient quantization LUT
  - [ ] Quantization tables for different bit depths
  - [ ] `quantization_lut_lossless_8/10/12/16`

- [ ] **jls_codec_factory.h** - Codec factory
  - [ ] Create encoder/decoder based on traits
  - [ ] Strategy pattern implementation

### Utilities
- [ ] **util.h** - Utility functions
  - [ ] `log2_ceil()` - Ceiling of log2
  - [ ] `bit_wise_sign()` - Sign bit extraction
  - [ ] `compute_range_parameter()` - Range calculation
  - [ ] `compute_limit_parameter()` - Limit calculation
  - [ ] `initialization_value_for_a()` - Context A initialization

### Color Transforms
- [ ] **color_transform.h** - Color space transformations
  - [ ] `transform_none` - No transformation
  - [ ] `transform_hp1` - HP transformation 1
  - [ ] `transform_hp2` - HP transformation 2
  - [ ] `transform_hp3` - HP transformation 3

### Process Line (Data Layout)
- [ ] **process_line.h** - Handle different pixel formats
  - [ ] `post_process_single_component` - Grayscale
  - [ ] `post_process_single_component_masked` - Bit masking
  - [ ] `process_transformed` - Color transformations
  - [ ] Interleaved vs planar handling

### Encoder/Decoder Strategies
- [ ] **encoder_strategy.h** - Encoding strategy interface
- [ ] **decoder_strategy.h** - Decoding strategy interface

## Our Go Implementation Structure

### Core Files to Align
```
jpegls/
├── lossless/           # NEAR=0 implementation
│   ├── codec.go        → Matches jpeg_stream_reader/writer
│   ├── encoder.go      → Matches scan.h encoder logic
│   ├── decoder.go      → Matches scan.h decoder logic
│   ├── context.go      → Matches context_regular_mode.h
│   ├── traits.go       → Matches lossless_traits.h
│   ├── predictor.go    → Matches MED predictor
│   ├── golomb.go       → Matches encode/decode_value
│   ├── runmode.go      → Matches context_run_mode.h
│   └── [tests]
│
├── nearlossless/       # NEAR>0 implementation
│   ├── codec.go        → Matches jpeg_stream_reader/writer
│   ├── encoder.go      → Matches scan.h encoder with default_traits
│   ├── decoder.go      → Matches scan.h decoder with default_traits
│   └── [uses lossless/* for shared code]
│
└── common/
    └── utils.go        → Matches util.h
```

## Detailed Alignment Checklist

### 1. Constants and Parameters (constants.h, util.h)
- [x] `default_reset_value` = 64
- [x] `max_k_value` = 16
- [x] `compute_range_parameter()` - Range = (MaxVal+2*NEAR)/(2*NEAR+1)+1
- [x] `compute_limit_parameter()` - Limit = 2*(qbpp + max(8, qbpp))
- [x] `log2_ceil()` - Ceiling of log2
- [ ] `initialization_value_for_a()` - A_init = max(2, (RANGE+32)/64)
- [x] Threshold calculations (T1, T2, T3)

### 2. Context Management (context_regular_mode.h)
- [x] Context struct (A, B, C, N)
- [x] `get_golomb_coding_parameter()` - k computation
- [x] `get_error_correction()` - Sign of (2*B+N-1)
- [x] `update_variables_and_bias()` - Update and reset logic
- [ ] Overflow protection (limit = 65536*256)
- [x] Bias update thresholds (min_c=-128, max_c=127)

### 3. Traits Implementation (default_traits.h)
- [x] `quantize()` - Error quantization
- [x] `dequantize()` - Error dequantization
- [x] `modulo_range()` - Error range reduction
- [x] `correct_prediction()` - Clamp to [0, MaxVal]
- [x] `fix_reconstructed_value()` - Wraparound handling
- [x] `compute_error_value()` - modulo_range(quantize(e))
- [x] `compute_reconstructed_sample()` - fix(pred + dequantize(err))
- [x] `is_near()` - abs(a-b) <= NEAR

### 4. Golomb Coding (scan.h lines 253-293)
- [x] `encode_mapped_value()` - Normal case
- [x] `encode_mapped_value()` - Escape case (high_bits >= limit)
- [x] `encode_mapped_value()` - Split long unary codes (>31 bits)
- [x] `decode_value()` - Normal case
- [x] `decode_value()` - Escape case
- [x] `decode_value()` - k==0 optimization

### 5. Regular Mode (scan.h do_regular)
- [x] Encoder: Sign symmetry
- [x] Encoder: Context lookup
- [x] Encoder: Prediction correction
- [x] Encoder: Error computation
- [x] Encoder: Error correction XOR
- [x] Encoder: Context update
- [x] Encoder: Sample reconstruction
- [x] Decoder: (same steps)

### 6. Run Mode (scan.h do_run_mode, context_run_mode.h)
- [x] Run length encoding with J table
- [x] Run continuation check (is_near)
- [ ] Run interruption encoding
- [ ] Run interruption decoding
- [x] Run index increment/decrement
- [ ] End-of-line handling

### 7. Line Processing (scan.h do_line)
- [x] Line-by-line processing
- [x] Neighbor pixel tracking (ra, rb, rc, rd)
- [x] Context computation
- [x] Run mode vs regular mode switching
- [ ] First line handling
- [ ] Left edge handling

### 8. JPEG Markers (jpeg_stream_reader/writer)
- [x] SOI (0xFFD8)
- [x] SOF55 (0xFFF7) - Start of Frame for JPEG-LS
- [x] LSE (0xFFF8) - JPEG-LS Extension
- [x] SOS (0xFFDA) - Start of Scan
- [x] EOI (0xFFD9)
- [ ] APP markers handling
- [ ] COM markers handling

### 9. Byte Stuffing
- [x] Encoding: 0xFF → 0xFF 0x00
- [x] Decoding: 0xFF 0x00 → 0xFF
- [x] Proper handling at byte boundaries

### 10. Edge Cases
- [ ] First pixel in image (no left neighbor)
- [ ] First line (no top neighbors)
- [ ] Left column (no left neighbor)
- [ ] Right column + line wrap
- [ ] Single pixel images
- [ ] Single line images
- [ ] Very small Range values (NEAR close to MaxVal/2)

## Priority Order

### Phase 1: Core Algorithm (Critical)
1. ✅ Constants and parameters
2. ✅ Traits (default_traits for near-lossless)
3. ✅ Context management
4. ✅ Golomb coding
5. ✅ Regular mode logic
6. 🔄 Run mode logic (partially done)

### Phase 2: Complete Implementation
7. ⬜ Edge case handling
8. ⬜ Run mode edge cases
9. ⬜ All marker types
10. ⬜ Color transformations (if needed)

### Phase 3: Verification
11. ⬜ Bit-exact output comparison with fo-dicom
12. ⬜ All NEAR values (0-255)
13. ⬜ All bit depths (2-16)
14. ⬜ All image sizes

## Testing Strategy

For each component:
1. Create unit test matching CharLS behavior
2. Test with NEAR=0 first (lossless)
3. Then test NEAR=1,2,3,5,7,10
4. Test edge cases separately
5. Compare bit stream with CharLS output

## Current Status

✅ = Completed and tested
🔄 = Partially done
⬜ = Not started

**Overall Progress: ~70% (core algorithm complete, edge cases remain)**
