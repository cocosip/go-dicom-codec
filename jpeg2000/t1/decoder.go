package t1

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// T1Decoder implements EBCOT Tier-1 decoding
// Reference: ISO/IEC 15444-1:2019 Annex D
type T1Decoder struct {
	// Code-block dimensions
	width  int
	height int

	// Wavelet coefficients (output)
	// Stored in row-major order
	data []int32

	// State flags for each coefficient
	// Stores significance, refinement, visit flags and neighbor info
	flags []uint32

	// MQ decoder
	mqc *mqc.MQDecoder

	// Current bit-plane being decoded
	bitplane int

	// Subband orientation (0=LL, 1=HL, 2=LH, 3=HH)
	orientation int

	// Decoding parameters
	roishift     int  // ROI shift value
	cblkstyle    int  // Code-block style flags
	resetctx     bool // Reset context on each pass
	termall      bool // Terminate all passes
	segmentation bool // Use segmentation symbols
}

// NewT1Decoder creates a new Tier-1 decoder
func NewT1Decoder(width, height int, cblkstyle int) *T1Decoder {
	// Add padding for boundary conditions (1 pixel on each side)
	paddedWidth := width + 2
	paddedHeight := height + 2

	t1 := &T1Decoder{
		width:  width,
		height: height,
		data:   make([]int32, paddedWidth*paddedHeight),
		flags:  make([]uint32, paddedWidth*paddedHeight),
	}

	// Parse code-block style flags
	// Reference: ISO/IEC 15444-1:2019 Table A.18
	t1.cblkstyle = cblkstyle
	t1.resetctx = (cblkstyle & CblkStyleReset) != 0
	t1.termall = (cblkstyle & CblkStyleTermAll) != 0
	t1.segmentation = (cblkstyle & CblkStyleSegsym) != 0

	return t1
}

// SetOrientation sets the subband orientation for zero coding context lookup.
func (t1 *T1Decoder) SetOrientation(orient int) {
	t1.orientation = orient
}

// DecodeWithBitplane decodes a code-block starting from a specific bitplane
// This is used when the max bitplane is known (e.g., from packet header in T2)
func (t1 *T1Decoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error {
	return t1.DecodeWithOptions(data, numPasses, maxBitplane, roishift, false)
}

// DecodeLayered decodes a code-block encoded with TERMALL mode
// In TERMALL mode, each pass is independently terminated and requires MQC state reset
// passLengths[i] indicates the cumulative byte position after pass i (not used for now)
// This method assumes TERMALL mode is enabled
func (t1 *T1Decoder) DecodeLayered(data []byte, passLengths []int, maxBitplane int, roishift int) error {
	// By default: TERMALL mode with contexts preserved (not reset)
	// This matches OpenJPEG's behavior for TERMALL without RESET flag
	return t1.DecodeLayeredWithMode(data, passLengths, maxBitplane, roishift, true, false)
}

// DecodeLayeredWithMode decodes a code-block with optional TERMALL mode
// lossless parameter controls whether to reset MQ contexts between passes
func (t1 *T1Decoder) DecodeLayeredWithMode(data []byte, passLengths []int, maxBitplane int, roishift int, useTERMALL bool, lossless bool) error {

	if len(data) == 0 {
		return fmt.Errorf("empty code-block data")
	}
	if len(passLengths) == 0 {
		return fmt.Errorf("no pass lengths provided")
	}

	// 对于不启用 TERMALL 的情况，直接复用标准路径以保持行为一致
	if !useTERMALL {
		return t1.DecodeWithOptions(data, len(passLengths), maxBitplane, roishift, false)
	}

	// TERMALL mode: decode pass-by-pass using passLengths to slice data
	// Match encoder behavior:
	// - By default: PRESERVE contexts across passes (TERMALL without RESET flag)
	// - With RESET flag (lossless=true): reset contexts after each pass
	// Note: In TERMALL mode, each pass is flushed independently, so we need
	// to create a new MQC decoder for each pass's data, but contexts are preserved
	t1.roishift = roishift
	numPasses := len(passLengths)

	passIdx := 0
	prevEnd := 0

	// Track previous contexts for TERMALL mode (MQ only)
	var prevContexts []uint8
	resetContexts := lossless || (t1.cblkstyle&CblkStyleReset) != 0
	passType := 2

	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; {
		startBitplane := passType == 0 || (passType == 2 && passIdx == 0)
		if startBitplane {
			// Clear VISIT flags at start of each bitplane
			paddedWidth := t1.width + 2
			paddedHeight := t1.height + 2
			for i := 0; i < paddedWidth*paddedHeight; i++ {
				t1.flags[i] &^= T1_VISIT
			}

			if t1.roishift > 0 && t1.bitplane >= t1.roishift {
				passType = 0
				t1.bitplane--
				continue
			}
		}

		raw := isLazyRawPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle)
		currentEnd := passLengths[passIdx]
		if currentEnd < prevEnd || currentEnd > len(data) {
			return fmt.Errorf("invalid pass length at pass %d: %d (prevEnd=%d, dataLen=%d)", passIdx, currentEnd, prevEnd, len(data))
		}
		passData := data[prevEnd:currentEnd]

		// TERMALL mode: each pass is flushed independently by encoder
		// RAW passes use bypass decoding; MQ passes may preserve contexts.
		if raw {
			t1.mqc = mqc.NewRawDecoder(passData)
		} else if passIdx == 0 || resetContexts {
			t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			t1.mqc.SetContextState(CTX_UNI, 46)
			t1.mqc.SetContextState(CTX_RL, 3)
			t1.mqc.SetContextState(CTX_ZC_START, 4)
		} else {
			t1.mqc = mqc.NewMQDecoderWithContexts(passData, prevContexts)
		}

		prevEnd = currentEnd

		switch passType {
		case 0:
			if err := t1.decodeSigPropPass(raw); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
		case 1:
			if err := t1.decodeMagRefPass(raw); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
		case 2:
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			if t1.segmentation {
				for i := 0; i < 4; i++ {
					t1.mqc.Decode(CTX_UNI)
				}
			}
		}

		// Save contexts for next pass
		if !raw && !resetContexts {
			prevContexts = t1.mqc.GetContexts()
		}

		passIdx++
		if passType == 2 {
			passType = 0
			t1.bitplane--
		} else {
			passType++
		}
	}

	return nil
}

// DecodeWithOptions decodes a code-block with optional TERMALL mode support
// If useTERMALL is true, the decoder resets MQC state after each pass
// If useTERMALL is false, use the decoder's termall flag from code block style
func (t1 *T1Decoder) DecodeWithOptions(data []byte, numPasses int, maxBitplane int, roishift int, useTERMALL bool) error {
	// If useTERMALL parameter is false, use the decoder's own termall flag
	if !useTERMALL {
		useTERMALL = t1.termall
	}
	if len(data) == 0 {
		return fmt.Errorf("empty code-block data")
	}

	t1.roishift = roishift

	// Initialize MQ decoder with OpenJPEG default context states
	t1.mqc = mqc.NewMQDecoder(data, NUM_CONTEXTS)
	t1.mqc.SetContextState(CTX_UNI, 46)
	t1.mqc.SetContextState(CTX_RL, 3)
	t1.mqc.SetContextState(CTX_ZC_START, 4)

	// Decode passes using OpenJPEG sequencing.
	passIdx := 0
	passType := 2
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; {
		startBitplane := passType == 0 || (passType == 2 && passIdx == 0)
		if startBitplane {
			// Clear VISIT flags at start of each bitplane
			paddedWidth := t1.width + 2
			paddedHeight := t1.height + 2
			for i := 0; i < paddedWidth*paddedHeight; i++ {
				t1.flags[i] &^= T1_VISIT
			}

			// Check if this bit-plane needs decoding
			if t1.roishift > 0 && t1.bitplane >= t1.roishift {
				passType = 0
				t1.bitplane--
				continue
			}
		}

		raw := isLazyRawPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle)
		switch passType {
		case 0:
			if err := t1.decodeSigPropPass(raw); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
		case 1:
			if err := t1.decodeMagRefPass(raw); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
		case 2:
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			if t1.segmentation {
				for i := 0; i < 4; i++ {
					t1.mqc.Decode(CTX_UNI)
				}
			}
		}
		passIdx++

		// In TERMALL mode, reinitialize MQC decoder state and reset contexts after each pass
		// But only if there are more passes to decode
		if useTERMALL && passIdx < numPasses {
			t1.mqc.ReinitAfterTermination()
			t1.mqc.ResetContexts()
			t1.mqc.SetContextState(CTX_UNI, 46)
			t1.mqc.SetContextState(CTX_RL, 3)
			t1.mqc.SetContextState(CTX_ZC_START, 4)
		}

		// Reset context if required
		if t1.resetctx && passIdx < numPasses && !raw {
			t1.mqc.ResetContexts()
			t1.mqc.SetContextState(CTX_UNI, 46)
			t1.mqc.SetContextState(CTX_RL, 3)
			t1.mqc.SetContextState(CTX_ZC_START, 4)
		}

		if passType == 2 {
			passType = 0
			t1.bitplane--
		} else {
			passType++
		}
	}

	return nil
}

// Decode decodes a code-block
// This estimates the starting bitplane from numPasses
//
// Performance notes:
// - Most computationally intensive part of JPEG 2000 decoding
// - Processes coefficients bit-plane by bit-plane (MSB to LSB)
// - First bit-plane starts with Cleanup, then SPP/MRP/CP for remaining bit-planes
// - Context modeling using 8-neighbor flags (cached for speed)
// - MQ decoding is the inner loop bottleneck
// - Typical workload: 32x32 block = 1024 coefficients × 12-16 bit-planes
// - Optimization opportunities: vectorization, parallel code-blocks
func (t1 *T1Decoder) Decode(data []byte, numPasses int, roishift int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty code-block data")
	}

	t1.roishift = roishift

	// Initialize MQ decoder with OpenJPEG default context states
	t1.mqc = mqc.NewMQDecoder(data, NUM_CONTEXTS)
	t1.mqc.SetContextState(CTX_UNI, 46)
	t1.mqc.SetContextState(CTX_RL, 3)
	t1.mqc.SetContextState(CTX_ZC_START, 4)

	// Determine starting bit-plane from number of passes
	// OpenJPEG sequencing: numBitplanes = (numPasses + 2) / 3
	numBitplanes := (numPasses + 2) / 3

	// Start from the highest bit-plane that was encoded
	// In a full implementation, this would come from the packet header
	// For now, estimate based on number of passes
	startBitplane := numBitplanes - 1
	// DEBUG removed

	// Decode passes using OpenJPEG sequencing.
	passIdx := 0
	passType := 2
	for t1.bitplane = startBitplane; t1.bitplane >= 0 && passIdx < numPasses; {
		startBitplanePass := passType == 0 || (passType == 2 && passIdx == 0)
		if startBitplanePass {
			// Clear VISIT flags at start of each bitplane
			paddedWidth := t1.width + 2
			paddedHeight := t1.height + 2
			for i := 0; i < paddedWidth*paddedHeight; i++ {
				t1.flags[i] &^= T1_VISIT
			}

			// Check if this bit-plane needs decoding
			if t1.roishift > 0 && t1.bitplane >= t1.roishift {
				passType = 0
				t1.bitplane--
				continue
			}
		}

		raw := isLazyRawPass(t1.bitplane, startBitplane, passType, t1.cblkstyle)
		switch passType {
		case 0:
			if err := t1.decodeSigPropPass(raw); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
		case 1:
			if err := t1.decodeMagRefPass(raw); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
		case 2:
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			if t1.segmentation {
				for i := 0; i < 4; i++ {
					t1.mqc.Decode(CTX_UNI)
				}
			}
		}
		passIdx++

		// Reset context if required
		if t1.resetctx && passIdx < numPasses && !raw {
			t1.mqc.ResetContexts()
			t1.mqc.SetContextState(CTX_UNI, 46)
			t1.mqc.SetContextState(CTX_RL, 3)
			t1.mqc.SetContextState(CTX_ZC_START, 4)
		}

		if passType == 2 {
			passType = 0
			t1.bitplane--
		} else {
			passType++
		}
	}

	return nil
}

// GetData returns the decoded coefficients (without padding)
// Note: Does NOT apply T1_NMSEDEC_FRACBITS inverse scaling - that should be done by the caller (tile decoder)
func (t1 *T1Decoder) GetData() []int32 {
	// Extract data without padding
	result := make([]int32, t1.width*t1.height)
	paddedWidth := t1.width + 2

	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			// Skip padding (1 pixel border)
			idx := (y+1)*paddedWidth + (x + 1)
			result[y*t1.width+x] = t1.data[idx]
		}
	}

	return result
}

// decodeSigPropPass decodes the Significance Propagation Pass
// This pass encodes coefficients that:
// - Are not yet significant
// - Have at least one significant neighbor
func (t1 *T1Decoder) decodeSigPropPass(raw bool) error {
	paddedWidth := t1.width + 2

	// JPEG 2000 passes are stripe-coded: process 4-row groups, then columns, then rows in stripe.
	for k := 0; k < t1.height; k += 4 {
		for x := 0; x < t1.width; x++ {
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (x + 1)
				flags := t1.flags[idx]

				// Skip if already significant
				if flags&T1_SIG != 0 {
					continue
				}

				// Check if has significant neighbor
				if flags&T1_SIG_NEIGHBORS == 0 {
					continue
				}

				// Decode significance bit
				ctx := getZeroCodingContext(flags, t1.orientation)
				bit := 0
				if raw {
					bit = t1.mqc.RawDecode()
				} else {
					bit = t1.mqc.Decode(int(ctx))
				}

				// Mark as visited in SPP regardless of significance result (OpenJPEG PI flag behavior).
				t1.flags[idx] |= T1_VISIT

				if bit != 0 {
					// Coefficient becomes significant
					// Decode sign bit
					sign := 0
					if raw {
						sign = t1.mqc.RawDecode()
					} else {
						signCtx := getSignCodingContext(flags)
						signBit := t1.mqc.Decode(int(signCtx))
						signPred := getSignPrediction(flags)
						sign = signBit ^ signPred
					}

					// Set coefficient value (2^bitplane) and sign
					// Note: This is the first time this coefficient becomes significant
					val := int32(1) << uint(t1.bitplane)
					if sign != 0 {
						t1.flags[idx] |= T1_SIGN
					}
					// Apply sign to value
					if t1.flags[idx]&T1_SIGN != 0 {
						t1.data[idx] = -val
					} else {
						t1.data[idx] = val
					}

					// Mark as significant (VISIT already set for this SPP sample).
					t1.flags[idx] |= T1_SIG

					// Update neighbor flags
					t1.updateNeighborFlags(x, y, idx)
				}
			}
		}
	}

	return nil
}

// decodeMagRefPass decodes the Magnitude Refinement Pass
// This pass refines coefficients that are already significant
func (t1 *T1Decoder) decodeMagRefPass(raw bool) error {
	paddedWidth := t1.width + 2

	// JPEG 2000 passes are stripe-coded: process 4-row groups, then columns, then rows in stripe.
	for k := 0; k < t1.height; k += 4 {
		for x := 0; x < t1.width; x++ {
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (x + 1)
				flags := t1.flags[idx]

				// Only refine significant coefficients not visited in this bit-plane
				if (flags&T1_SIG) == 0 || (flags&T1_VISIT) != 0 {
					continue
				}

				// Decode refinement bit
				ctx := getMagRefinementContext(flags)
				bit := 0
				if raw {
					bit = t1.mqc.RawDecode()
				} else {
					bit = t1.mqc.Decode(int(ctx))
				}

				// Update coefficient magnitude
				if bit != 0 {
					if t1.data[idx] >= 0 {
						t1.data[idx] += int32(1) << uint(t1.bitplane)
					} else {
						t1.data[idx] -= int32(1) << uint(t1.bitplane)
					}
				}

				// Mark as refined (OpenJPEG MU flag behavior).
				t1.flags[idx] |= T1_REFINE
			}
		}
	}

	return nil
}

// decodeCleanupPass decodes the Cleanup Pass
// IMPORTANT: Process in VERTICAL order (column-first) with 4-row groups for RL decoding
// This matches OpenJPEG's opj_t1_dec_clnpass() implementation and the encoder
func (t1 *T1Decoder) decodeCleanupPass() error {
	paddedWidth := t1.width + 2

	// Process in groups of 4 rows (vertical RL decoding)
	for k := 0; k < t1.height; k += 4 {
		for i := 0; i < t1.width; i++ {
			// Try run-length decoding for this column (4 vertical coefficients)
			// Only if all 4 rows are available
			if k+3 < t1.height {
				// Check if run-length coding can be applied to this 4-coeff vertical run
				canUseRL := true

				for dy := 0; dy < 4; dy++ {
					y := k + dy
					idx := (y+1)*paddedWidth + (i + 1)

					// Skip if already visited
					if (t1.flags[idx] & T1_VISIT) != 0 {
						canUseRL = false
						break
					}

					// Check if coefficient or neighbors are already significant
					if (t1.flags[idx]&T1_SIG) != 0 || (t1.flags[idx]&T1_SIG_NEIGHBORS) != 0 {
						canUseRL = false
						break
					}
				}

				if canUseRL {
					// Decode run-length bit
					rlBit := t1.mqc.Decode(CTX_RL)

					if rlBit == 0 {
						continue // Move to next column
					}

					// At least one is significant, decode uniformly which one
					runlen := 0
					bit1 := t1.mqc.Decode(CTX_UNI)
					runlen |= bit1 << 1
					bit2 := t1.mqc.Decode(CTX_UNI)
					runlen |= bit2

					// In RL path, the first sample at runlen is implicitly significant
					partial := true
					for dy := runlen; dy < 4; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						flags := t1.flags[idx]

						if (flags&T1_VISIT) != 0 || (flags&T1_SIG) != 0 {
							t1.flags[idx] &^= T1_VISIT
							continue
						}

						isSig := 0
						if partial {
							isSig = 1
							partial = false
						} else {
							ctx := getZeroCodingContext(flags, t1.orientation)
							isSig = t1.mqc.Decode(int(ctx))
						}

						if isSig != 0 {
							// Decode sign bit with prediction (same as OpenJPEG clnpass)
							signCtx := getSignCodingContext(flags)
							signBit := t1.mqc.Decode(int(signCtx))
							signPred := getSignPrediction(flags)
							sign := signBit ^ signPred

							// Set coefficient value (2^bitplane) and sign
							val := int32(1) << uint(t1.bitplane)
							if sign != 0 {
								t1.flags[idx] |= T1_SIGN
							}
							if t1.flags[idx]&T1_SIGN != 0 {
								t1.data[idx] = -val
							} else {
								t1.data[idx] = val
							}

							// Mark as significant. Cleanup pass does not keep PI/VISIT set.
							t1.flags[idx] |= T1_SIG

							// Update neighbor flags
							t1.updateNeighborFlags(i, y, idx)
						}

						// Match OpenJPEG PI behavior: cleanup pass clears PI/VISIT after handling a sample.
						t1.flags[idx] &^= T1_VISIT
					}

					continue // RL decoding handled this column, move to next
				}
			}

			// Normal processing (not part of RL decoding, or partial row group)
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (i + 1)
				flags := t1.flags[idx]

				if (flags&T1_VISIT) != 0 || (flags&T1_SIG) != 0 {
					t1.flags[idx] &^= T1_VISIT
					continue
				}

				// Decode significance bit
				ctx := getZeroCodingContext(flags, t1.orientation)
				bit := t1.mqc.Decode(int(ctx))

				if bit != 0 {
					// Decode sign bit with prediction (same as OpenJPEG clnpass)
					signCtx := getSignCodingContext(flags)
					signBit := t1.mqc.Decode(int(signCtx))
					signPred := getSignPrediction(flags)
					sign := signBit ^ signPred

					// Set coefficient value (2^bitplane) and sign
					val := int32(1) << uint(t1.bitplane)
					if sign != 0 {
						t1.flags[idx] |= T1_SIGN
					}
					if t1.flags[idx]&T1_SIGN != 0 {
						t1.data[idx] = -val
					} else {
						t1.data[idx] = val
					}

					// Mark as significant. Cleanup pass does not keep PI/VISIT set.
					t1.flags[idx] |= T1_SIG

					// Update neighbor flags
					t1.updateNeighborFlags(i, y, idx)
				}

				// Match OpenJPEG PI behavior: cleanup pass clears PI/VISIT after handling a sample.
				t1.flags[idx] &^= T1_VISIT
			}
		}
	}

	return nil
}

// updateNeighborFlags updates the neighbor significance flags
// when a coefficient becomes significant
func (t1 *T1Decoder) updateNeighborFlags(x, y, idx int) {
	paddedWidth := t1.width + 2
	sign := t1.flags[idx] & T1_SIGN

	// Update 8 neighbors
	// Padding ensures all neighbors are valid, no boundary checks needed

	// North
	nIdx := (y)*paddedWidth + (x + 1)
	t1.flags[nIdx] |= T1_SIG_S
	if sign != 0 {
		t1.flags[nIdx] |= T1_SIGN_S
	}

	// South
	sIdx := (y+2)*paddedWidth + (x + 1)
	t1.flags[sIdx] |= T1_SIG_N
	if sign != 0 {
		t1.flags[sIdx] |= T1_SIGN_N
	}

	// West
	wIdx := (y+1)*paddedWidth + x
	t1.flags[wIdx] |= T1_SIG_E
	if sign != 0 {
		t1.flags[wIdx] |= T1_SIGN_E
	}

	// East
	eIdx := (y+1)*paddedWidth + (x + 2)
	t1.flags[eIdx] |= T1_SIG_W
	if sign != 0 {
		t1.flags[eIdx] |= T1_SIGN_W
	}

	// Northwest
	t1.flags[(y)*paddedWidth+x] |= T1_SIG_SE

	// Northeast
	t1.flags[(y)*paddedWidth+(x+2)] |= T1_SIG_SW

	// Southwest
	t1.flags[(y+2)*paddedWidth+x] |= T1_SIG_NE

	// Southeast
	t1.flags[(y+2)*paddedWidth+(x+2)] |= T1_SIG_NW
}
