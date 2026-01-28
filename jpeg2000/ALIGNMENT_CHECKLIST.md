# JPEG2000 / HTJ2K Alignment Checklist

Legend: [ ] = pending, [x] = done

## JPEG2000 (align with OpenJPEG)

### MQ arithmetic coder
- [x] 47-state table values and transitions (Qe/MPS/LPS)
- [x] Renormalization/byteout/carry propagation rules
- [x] Byte stuffing (0xFF/0x00) and flush behavior

### Tier-1 (EBCOT) coding
- [x] Context modeling (19 contexts) and neighbor significance rules
- [x] Pass sequencing (SPP/MRP/CP) and termination conditions
- [x] Bitplane range (zero-bitplane/max-bitplane) and ROI shift handling

### Wavelet transforms (DWT)
- [x] 5/3 reversible lifting steps and boundary extension
- [x] 9/7 irreversible lifting steps and rounding policy
- [x] Multi-level decomposition alignment and precision growth

### Quantization & dequantization
- [x] QCD/QCC step size derivation per subband
- [x] Reversible vs irreversible quantization paths
- [x] Dequantization precision and rounding rules

### Tile/component/subband layout
- [x] Tile grid boundary rules and component sampling alignment
- [x] Resolution/subband coordinate mapping and clipping
- [x] Codeblock partitioning rules at image edges

### Precincts & geometry
- [x] Precinct size scaling across resolutions
- [x] Subband-to-precinct coordinate mapping
- [x] Packet iteration order for precincts

### Tier-2 packetization & progression
- [x] Progression iterator order (LRCP/RLCP/RPCL/PCRL/CPRL)
- [x] Packet header tag-tree coding rules
- [x] Empty packet handling and layer allocation

### Rate-distortion / PCRD
- [x] Distortion metric computation and pass weighting
- [x] Layer target rate allocation and truncation policy
- [x] Multi-tile rate budget aggregation

### ROI (Region of Interest)
- [x] MaxShift behavior and bitplane scaling rules
- [x] General scaling ROI semantics and marker handling
- [x] ROI priority when multiple regions exist

### Colorspace
- [x] RCT/ICT coefficients and rounding behavior
- [x] Channel order and signed/unsigned conversions
- [x] RGB<->YCbCr transform precision

### Multi-component (Part 2)
- [x] MCT/MCC/MCO marker parsing and execution order
- [x] AssocType semantics and record order precedence
- [x] Precision/rounding rules and reversible conditions

### Codestream parsing & generation
- [x] Marker set and ordering (SOC/SIZ/COD/COC/QCD/QCC/POC/RGN/SOT/SOD/EOC)
- [x] Default segment inheritance rules (COD->COC, QCD->QCC)
- [x] Tile-part length/count fields and concatenation rules

## HTJ2K (align with OpenJPH)

### VLC / UVLC tables
- [x] Annex C table completeness and indexing
- [x] Context selection (neighbor significance)
- [x] UVLC residual decoding rules

### MEL (run-length coder)
- [x] 13-state machine and exponent mapping
- [x] Bit packing and flush behavior
- [x] Decoder resync and boundary cases

### MagSgn
- [x] Magnitude/sign packing order
- [x] Bit significance and sign handling
- [x] Byte alignment at block end

### Exponent predictor / quad-pair
- [x] Exponent prediction inputs and edge handling
- [x] Quad-pair decoding order and state updates
- [x] Coupling with VLC context selection

### HT block framework
- [x] Quad scanning order and block segmentation
- [x] MagSgn/MEL/VLC stream growth direction and interleave
- [x] HT block header and termination rules

### HTJ2K pipeline integration
- [x] Packet header integration with HT codeblocks
- [ ] Rate control interaction with HT passes
- [ ] Decoder error resilience and resync points
