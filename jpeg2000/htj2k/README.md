# HTJ2K (High-Throughput JPEG 2000) Implementation

This package implements the HTJ2K block coder based on **ISO/IEC 15444-15:2019 / ITU-T T.814**.

## Implementation Status

### ✅ Completed Components

1. **MEL (Adaptive Run-Length Coder)** - `mel_spec.go`
   - Fully spec-compliant implementation
   - State machine with 13 states (MEL_E table)
   - Based on Clause 7.3.3 of ISO/IEC 15444-15

2. **MagSgn (Magnitude-Sign) Encoder/Decoder** - `magsgn.go`
   - Bit-level packing and unpacking
   - Little-endian bit order

3. **Framework and Data Structures**
   - HT block encoder/decoder structure
   - Quad-based scanning pattern
   - Segment assembly/disassembly

### ⚠️ Partial Implementation

1. **VLC (Variable Length Coding)** - `vlc_tables.go`
   - CxtVLC_table_0 and CxtVLC_table_1 are loaded from OpenJPH (Annex C)
   - Encoding/decoding integration is still partial (not production-ready)

2. **HT Cleanup Pass**
   - Core algorithm structure implemented
   - **TODO**: Full integration with complete VLC tables
   - **TODO**: Quad-pair interleaving and HT cleanup wiring (Clause 7.3.4)

## Technical Details

### MEL Algorithm

The MEL encoder uses a 13-state adaptive run-length coder:

```
State (k): 0  1  2  3  4  5  6  7  8  9  10 11 12
Exponent:  0  0  0  1  1  1  2  2  2  3  3  4  5
```

### VLC Tables

HTJ2K uses context-adaptive VLC tables (`CxtVLC_table_0` and `CxtVLC_table_1`) defined in Annex C.

Each entry contains:
- Context value (neighbor significance pattern)
- Rho (significance state)
- Unsigned offset (u_off)
- E_k value
- Codeword (little-endian)
- Code length

### HT Segments

HT code-blocks contain three byte-streams:

1. **MagSgn**: Magnitude and sign bits (grows forward)
2. **MEL**: Run-length coded symbols (grows forward)
3. **VLC**: Variable-length codes (grows backward)

## Usage Example

```go
// Create HT encoder
encoder := t1ht.NewHTEncoder(width, height)

// Encode code-block
data := []int32{...} // Wavelet coefficients
encoded, err := encoder.Encode(data, numPasses, roishift)

// Create HT decoder
decoder := t1ht.NewHTDecoder(width, height)

// Decode code-block
decoded, err := decoder.Decode(encoded, numPasses)
```

## Future Work

To achieve full HTJ2K compliance:

1. **VLC Table Validation** (High Priority)
   - Keep CxtVLC tables synced with OpenJPH source
   - Expand lookup-generation coverage

2. **Context Integration** (High Priority)
   - Validate context selection against OpenJPH edge cases
   - Wire context usage through the full HT cleanup pass

3. **Quad-Pair Interleaving** (Medium Priority)
   - Implement VLC bit-stream interleaving (Clause 7.3.4)

4. **U-VLC Decoding** (Medium Priority)
   - Implement unsigned residual VLC decoding (Clause 7.3.6)

5. **Testing and Validation**
   - Test against reference HTJ2K implementations (OpenJPH, OpenJPEG)
   - Validate with conformance test images

## Image Interoperability Fixtures

HTJ2K interoperability in this package is validated at the image codec layer:

```text
raw image samples + image parameters <-> HTJ2K codestream
```

The fixture tests intentionally do not validate DICOM datasets, transfer syntax
metadata, pixel-data encapsulation, frame item padding, or tag rewriting. Those
concerns belong to the DICOM integration layer that consumes this codec package.

Reference codestreams live under the repository-level
`test-data/htj2k/interop` directory and are described by
`test-data/htj2k/interop/manifest.json`. The manifest records image geometry,
bit depth, signedness, raw sample layout, byte order, and the reference
codestream files generated offline by fo-dicom.Codecs/OpenJPH.

The in-repo Go tests verify:

1. Go decoding of fo-dicom.Codecs/OpenJPH reference codestreams.
2. Lossless decoded samples are byte-identical to the reference raw files.
3. Reference codestreams carry HTJ2K profile signals such as SIZ Rsiz bit 14,
   CAP, COD HT code-block style, and normal COD signalling.

Go encoder output is not expected to be byte-identical to fo-dicom.Codecs output.
JPEG 2000 codestreams can differ while remaining interoperable. Encoder
interoperability should be checked by an offline acceptance pass:

1. Generate `go_*.j2c` files from the current Go encoder.
2. Decode those files with fo-dicom.Codecs/OpenJPH outside this Go test suite.
3. Compare the decoded raw samples with the manifest `inputRaw` files.
4. Add or update committed reference fixtures only after the offline result is
   understood and reproducible.

## References

- **ISO/IEC 15444-15:2019**: Information technology — JPEG 2000 image coding system — Part 15: High-Throughput JPEG 2000
- **ITU-T T.814 (06/2019)**: Same standard, ITU-T designation
- **OpenJPH**: Reference C++ implementation by Aous Naman
- **OpenJPEG 2.5+**: Includes HTJ2K decoding support

## License

This implementation is based on the publicly available ISO/IEC 15444-15 standard and is provided for educational and research purposes.
