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
	t1.resetctx = (cblkstyle & 0x01) != 0     // Bit 0: Selective arithmetic coding bypass
	t1.termall = (cblkstyle & 0x02) != 0      // Bit 1: Termination on each coding pass
	t1.segmentation = (cblkstyle & 0x04) != 0 // Bit 2: Segmentation symbols

	return t1
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
	return t1.DecodeLayeredWithMode(data, passLengths, maxBitplane, roishift, true, true)
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

	// DEBUG: Track LH subband decode
	debugLH := len(data) == 58 && len(passLengths) == 18
	if debugLH {
		fmt.Printf("[T1 DECODE LH] dataLen=%d, passLengths len=%d, maxBitplane=%d, useTERMALL=%v, lossless=%v\n",
			len(data), len(passLengths), maxBitplane, useTERMALL, lossless)
		if len(passLengths) >= 3 {
			fmt.Printf("[T1 DECODE LH] First 3 passLengths: %v\n", passLengths[:3])
		}
	}

	// 对于不启用 TERMALL 的情况，直接复用标准路径以保持行为一致
	if !useTERMALL {
		return t1.DecodeWithOptions(data, len(passLengths), maxBitplane, roishift, false)
	}

	// TERMALL mode: decode pass-by-pass using passLengths to slice data
	// Match encoder behavior:
	// - Lossless: reset contexts after each pass (create new decoder)
	// - Lossy: preserve contexts across passes (use SetData to update data while keeping contexts)
	t1.roishift = roishift
	numPasses := len(passLengths)

	passIdx := 0
	prevEnd := 0

	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Clear VISIT flags at start of each bitplane
		paddedWidth := t1.width + 2
		paddedHeight := t1.height + 2
		for i := 0; i < paddedWidth*paddedHeight; i++ {
			t1.flags[i] &^= T1_VISIT
		}

		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			continue
		}

		// Decode Significance Propagation Pass (SPP)
		if passIdx < numPasses {
			currentEnd := passLengths[passIdx]
			if currentEnd < prevEnd || currentEnd > len(data) {
				return fmt.Errorf("invalid pass length for SPP at pass %d: %d (prevEnd=%d, dataLen=%d)", passIdx, currentEnd, prevEnd, len(data))
			}
			passData := data[prevEnd:currentEnd]

			// DEBUG: Track first SPP
			if debugLH && passIdx == 0 {
				fmt.Printf("[T1 DECODE LH SPP0] bitplane=%d, passIdx=%d, prevEnd=%d, currentEnd=%d, passDataLen=%d\n",
					t1.bitplane, passIdx, prevEnd, currentEnd, len(passData))
				if len(passData) > 0 {
					fmt.Printf("[T1 DECODE LH SPP0] passData first bytes: %v\n", passData[:min(8, len(passData))])
				}
			}

			// TERMALL mode: each pass is flushed independently by encoder
			// Encoder resets C/A/ct after flush but preserves contexts in lossy mode
			if lossless {
				// Lossless: always use fresh decoder (reset contexts)
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			} else {
				// Lossy: WORKAROUND - use fresh decoder until context preservation is fixed
				// TODO: Should use NewMQDecoderWithContexts to preserve contexts
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			}
			prevEnd = currentEnd

			if err := t1.decodeSigPropPass(); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}

			// DEBUG: Check contexts after SPP
			if debugLH && !lossless {
				contexts := t1.mqc.GetContexts()
				nonZeroCtx := 0
				for _, ctx := range contexts {
					if ctx != 0 {
						nonZeroCtx++
					}
				}
				fmt.Printf("[AFTER SPP %d] bp=%d, nonZeroContexts=%d/%d\n", passIdx, t1.bitplane, nonZeroCtx, len(contexts))
			}

			passIdx++
		}

		// Decode Magnitude Refinement Pass (MRP)
		if passIdx < numPasses {
			currentEnd := passLengths[passIdx]
			if currentEnd < prevEnd || currentEnd > len(data) {
				return fmt.Errorf("invalid pass length for MRP at pass %d: %d (prevEnd=%d, dataLen=%d)", passIdx, currentEnd, prevEnd, len(data))
			}
			passData := data[prevEnd:currentEnd]

			// TERMALL mode: each pass is flushed independently by encoder
			// Encoder resets C/A/ct after flush but preserves contexts in lossy mode
			if lossless {
				// Lossless: always use fresh decoder (reset contexts)
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			} else {
				// Lossy: WORKAROUND - use fresh decoder until context preservation is fixed
				// TODO: Should use NewMQDecoderWithContexts to preserve contexts
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			}
			prevEnd = currentEnd

			if err := t1.decodeMagRefPass(); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}

			passIdx++
		}

		// Decode Cleanup Pass (CP)
		if passIdx < numPasses {
			currentEnd := passLengths[passIdx]
			if currentEnd < prevEnd || currentEnd > len(data) {
				return fmt.Errorf("invalid pass length for CP at pass %d: %d (prevEnd=%d, dataLen=%d)", passIdx, currentEnd, prevEnd, len(data))
			}
			passData := data[prevEnd:currentEnd]

			// DEBUG: Track first CP call
			if debugLH && t1.bitplane == 5 {
				fmt.Printf("[BEFORE CP] bitplane=%d, passIdx=%d, passDataLen=%d, width=%d, height=%d\n",
					t1.bitplane, passIdx, len(passData), t1.width, t1.height)
				if len(passData) > 0 {
					fmt.Printf("[BEFORE CP] passData bytes: %v\n", passData[:min(len(passData), 12)])
				}
			}

			// TERMALL mode: each pass is flushed independently by encoder
			// Encoder resets C/A/ct after flush but preserves contexts in lossy mode
			if lossless {
				// Lossless: always use fresh decoder (reset contexts)
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			} else {
				// Lossy: WORKAROUND - use fresh decoder until context preservation is fixed
				// TODO: Should use NewMQDecoderWithContexts to preserve contexts
				t1.mqc = mqc.NewMQDecoder(passData, NUM_CONTEXTS)
			}
			prevEnd = currentEnd

			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}

			passIdx++
		}
	}

	// DEBUG: Check final data state
	if debugLH {
		nonZero := 0
		for _, v := range t1.data {
			if v != 0 {
				nonZero++
			}
		}
		fmt.Printf("[T1 DECODE LH DONE] Decoded %d passes, data len=%d, nonZero count=%d\n",
			passIdx, len(t1.data), nonZero)
		if len(t1.data) >= 4 {
			fmt.Printf("[T1 DECODE LH DONE] First 4 values: %v\n", t1.data[:4])
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

	// DEBUG removed

	// Initialize MQ decoder
	t1.mqc = mqc.NewMQDecoder(data, NUM_CONTEXTS)

	// Decode bit-planes from MSB to LSB
	// Each bit-plane has up to 3 coding passes
	passIdx := 0
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Clear VISIT flags at start of each bitplane
		// This ensures coefficients can be processed in passes of this bitplane
		paddedWidth := t1.width + 2
		paddedHeight := t1.height + 2
		for i := 0; i < paddedWidth*paddedHeight; i++ {
			t1.flags[i] &^= T1_VISIT
		}

		// Check if this bit-plane needs decoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			// Skip this bit-plane (ROI region)
			continue
		}

		// DEBUG removed

		// Three coding passes per bit-plane:
		// 1. Significance Propagation Pass (SPP)
		// 2. Magnitude Refinement Pass (MRP)
		// 3. Cleanup Pass (CP)

		if passIdx < numPasses {
			if err := t1.decodeSigPropPass(); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
			passIdx++

			// In TERMALL mode, reinitialize MQC decoder state and reset contexts after each pass
			// But only if there are more passes to decode
			if useTERMALL && passIdx < numPasses {
				t1.mqc.ReinitAfterTermination()
				t1.mqc.ResetContexts()
			}
		}

		if passIdx < numPasses {
			if err := t1.decodeMagRefPass(); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
			passIdx++

			// In TERMALL mode, reinitialize MQC decoder state and reset contexts after each pass
			// But only if there are more passes to decode
			if useTERMALL && passIdx < numPasses {
				t1.mqc.ReinitAfterTermination()
				t1.mqc.ResetContexts()
			}
		}

		if passIdx < numPasses {
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			passIdx++

			// In TERMALL mode, reinitialize MQC decoder state and reset contexts after each pass
			// But only if there are more passes to decode
			if useTERMALL && passIdx < numPasses {
				t1.mqc.ReinitAfterTermination()
				t1.mqc.ResetContexts()
			}
		}

		// DEBUG removed

		// Reset context if required
		if t1.resetctx && passIdx < numPasses {
			t1.mqc.ResetContexts()
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
// - Three coding passes per bit-plane (Sig Prop, Mag Ref, Cleanup)
// - Context modeling using 8-neighbor flags (cached for speed)
// - MQ decoding is the inner loop bottleneck
// - Typical workload: 32x32 block = 1024 coefficients × 12-16 bit-planes
// - Optimization opportunities: vectorization, parallel code-blocks
func (t1 *T1Decoder) Decode(data []byte, numPasses int, roishift int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty code-block data")
	}

	t1.roishift = roishift

	// Initialize MQ decoder
	t1.mqc = mqc.NewMQDecoder(data, NUM_CONTEXTS)

	// Determine starting bit-plane from number of passes
	// Each bit-plane has 3 passes, so numBitplanes = ceil(numPasses / 3)
	numBitplanes := (numPasses + 2) / 3

	// Start from the highest bit-plane that was encoded
	// In a full implementation, this would come from the packet header
	// For now, estimate based on number of passes
	startBitplane := numBitplanes - 1
	// DEBUG removed

	// Decode bit-planes from MSB to LSB
	// Each bit-plane has up to 3 coding passes
	passIdx := 0
	for t1.bitplane = startBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Clear VISIT flags at start of each bitplane
		// This ensures coefficients can be processed in passes of this bitplane
		paddedWidth := t1.width + 2
		paddedHeight := t1.height + 2
		for i := 0; i < paddedWidth*paddedHeight; i++ {
			t1.flags[i] &^= T1_VISIT
		}

		// Check if this bit-plane needs decoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			// Skip this bit-plane (ROI region)
			continue
		}

		// Three coding passes per bit-plane:
		// 1. Significance Propagation Pass (SPP)
		// 2. Magnitude Refinement Pass (MRP)
		// 3. Cleanup Pass (CP)

		// DEBUG removed

		if passIdx < numPasses {
			if err := t1.decodeSigPropPass(); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			if err := t1.decodeMagRefPass(); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			passIdx++
		}

		// Reset context if required
		if t1.resetctx && passIdx < numPasses {
			t1.mqc.ResetContexts()
		}
	}

	return nil
}

// GetData returns the decoded coefficients (without padding)
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
func (t1 *T1Decoder) decodeSigPropPass() error {
	paddedWidth := t1.width + 2
	sigCount := 0 // DEBUG counter
	debugLH := t1.width == 8 && t1.height == 8 && t1.bitplane == 5 // LH subband detection
	decodeCount := 0

	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
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
			ctx := getZeroCodingContext(flags)
			bit := t1.mqc.Decode(int(ctx))
			decodeCount++

			// DEBUG: Track first few MQ decodes for LH
			if debugLH && decodeCount <= 5 {
				fmt.Printf("[SPP MQ] pos=(%d,%d), ctx=%d, bit=%d, hasNeighbors=%v\n",
					x, y, ctx, bit, flags&T1_SIG_NEIGHBORS != 0)
			}

			if bit != 0 {
				sigCount++ // DEBUG
				// Coefficient becomes significant
				// Decode sign bit
				signCtx := getSignCodingContext(flags)
				signBit := t1.mqc.Decode(int(signCtx))

				// Apply sign prediction
				signPred := getSignPrediction(flags)
				sign := signBit ^ signPred

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

				// Mark as significant
				t1.flags[idx] |= T1_SIG | T1_VISIT

				// Update neighbor flags
				t1.updateNeighborFlags(x, y, idx)
			}

			// Clear visit flag (ready for next pass/bit-plane)
			//          t1.flags[idx] &^= T1_VISIT
		}
	}

	// DEBUG: Print significance count for first pass
	if sigCount > 0 {
		fmt.Printf("[SPP DEBUG] bitplane=%d, sigCount=%d\n", t1.bitplane, sigCount)
	}

	return nil
}

// decodeMagRefPass decodes the Magnitude Refinement Pass
// This pass refines coefficients that are already significant
func (t1 *T1Decoder) decodeMagRefPass() error {
	paddedWidth := t1.width + 2

	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			flags := t1.flags[idx]

			// Only refine significant coefficients not visited in this bit-plane
			if (flags&T1_SIG) == 0 || (flags&T1_VISIT) != 0 {
				continue
			}

			// Decode refinement bit
			ctx := getMagRefinementContext(flags)
			bit := t1.mqc.Decode(int(ctx))

			// Update coefficient magnitude
			if bit != 0 {
				if t1.data[idx] >= 0 {
					t1.data[idx] += int32(1) << uint(t1.bitplane)
				} else {
					t1.data[idx] -= int32(1) << uint(t1.bitplane)
				}
			}

			// Mark as refined and visited (so CP won't refine again)
			t1.flags[idx] |= T1_REFINE | T1_VISIT
		}
	}

	return nil
}

// decodeCleanupPass decodes the Cleanup Pass
// IMPORTANT: Process in VERTICAL order (column-first) with 4-row groups for RL decoding
// This matches OpenJPEG's opj_t1_dec_clnpass() implementation and the encoder
func (t1 *T1Decoder) decodeCleanupPass() error {
	paddedWidth := t1.width + 2
	sigCount := 0 // DEBUG counter
	debugLH := t1.width == 8 && t1.height == 8 && t1.bitplane == 5 // LH subband detection
	mqDecodeCount := 0

	if debugLH {
		fmt.Printf("[CP START] width=%d, height=%d, bitplane=%d\n", t1.width, t1.height, t1.bitplane)
	}

	// Process in groups of 4 rows (vertical RL decoding)
	for k := 0; k < t1.height; k += 4 {
		for i := 0; i < t1.width; i++ {
			// Try run-length decoding for this column (4 vertical coefficients)
			// Only if all 4 rows are available
			if k+3 < t1.height {
				// Check if run-length coding can be applied to this 4-coeff vertical run
				canUseRL := true
				failReason := ""

				for dy := 0; dy < 4; dy++ {
					y := k + dy
					idx := (y+1)*paddedWidth + (i + 1)

					// Skip if already visited
					if (t1.flags[idx] & T1_VISIT) != 0 {
						canUseRL = false
						failReason = "visited"
						break
					}

					// Check if coefficient or neighbors are already significant
					if (t1.flags[idx]&T1_SIG) != 0 || (t1.flags[idx]&T1_SIG_NEIGHBORS) != 0 {
						canUseRL = false
						failReason = "sig or sig_neighbors"
						break
					}
				}

				// DEBUG: Track first few canUseRL checks
				if debugLH && (i*10+k) < 5 {
					fmt.Printf("[CP CANUSE] col=%d, rowGroup=%d, canUseRL=%v, reason=%s\n",
						i, k, canUseRL, failReason)
				}

				if canUseRL {
					// Decode run-length bit
					rlBit := t1.mqc.Decode(CTX_RL)
					mqDecodeCount++

					// DEBUG: Track LH RL decoding for first few columns
					if debugLH && mqDecodeCount <= 5 {
						fmt.Printf("[CP RL] col=%d, row_group=%d, rlBit=%d, canUseRL=%v\n",
							i, k, rlBit, canUseRL)
					}

					if rlBit == 0 {
						// All 4 remain insignificant
						// VISIT flags will be cleared at start of next bitplane
						continue // Move to next column
					}

					// At least one is significant, decode uniformly which one
					runlen := 0
					bit1 := t1.mqc.Decode(CTX_UNI)
					runlen |= bit1 << 1
					bit2 := t1.mqc.Decode(CTX_UNI)
					runlen |= bit2

					// DEBUG: Track LH RL runlen
					if debugLH && mqDecodeCount <= 5 {
						fmt.Printf("[CP RL] rlBit=1, runlen=%d, bit1=%d, bit2=%d\n", runlen, bit1, bit2)
					}

					// Coefficients before runlen remain insignificant
					// VISIT flags will be cleared at start of next bitplane

					// Process all coefficients from runlen to 3
					for dy := runlen; dy < 4; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						flags := t1.flags[idx]

						// Decode significance bit
						ctx := getZeroCodingContext(flags)
						bit := t1.mqc.Decode(int(ctx))

						if bit != 0 {
							// Check if already significant
							alreadySig := (flags & T1_SIG) != 0

							if !alreadySig {
								sigCount++ // DEBUG
								// Coefficient becomes significant
								// Decode sign bit
								signBit := t1.mqc.Decode(CTX_UNI)

								// Set coefficient value (2^bitplane) and sign
								// Note: This is the first time this coefficient becomes significant
								val := int32(1) << uint(t1.bitplane)
								if signBit != 0 {
									t1.flags[idx] |= T1_SIGN
								}
								// Apply sign to value
								if t1.flags[idx]&T1_SIGN != 0 {
									t1.data[idx] = -val
								} else {
									t1.data[idx] = val
								}

								// Mark as significant
								t1.flags[idx] |= T1_SIG | T1_VISIT

								// Update neighbor flags
								t1.updateNeighborFlags(i, y, idx)
							} else {
								// Already-significant coefficient in CP RL path
								// Encoder only encodes bit-plane value, no sign bit
								// Update coefficient value with this bit-plane's bit
								absVal := t1.data[idx]
								if absVal < 0 {
									absVal = -absVal
								}
								absVal |= (1 << uint(t1.bitplane))
								if t1.flags[idx]&T1_SIGN != 0 {
									t1.data[idx] = -absVal
								} else {
									t1.data[idx] = absVal
								}
							}
						}

						// VISIT flag will be cleared at start of next bitplane
					}

					continue // RL decoding handled this column, move to next
				}
			}

			// Normal processing (not part of RL decoding, or partial row group)
			// Process remaining rows in this group
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (i + 1)
				flags := t1.flags[idx]

				// Skip if already visited
				if (flags & T1_VISIT) != 0 {
					// Do not clear VISIT here - it will be cleared at start of next bitplane
					continue
				}

				// Check if already significant
				alreadySig := (flags & T1_SIG) != 0

				// Decode significance bit (always, even for already-significant coefficients)
				ctx := getZeroCodingContext(flags)
				bit := t1.mqc.Decode(int(ctx))

				if bit != 0 {
					if !alreadySig {
						// Coefficient becomes significant for the first time
						// Decode sign bit
						signBit := t1.mqc.Decode(CTX_UNI)

						// Set coefficient value (2^bitplane) and sign
						// Note: This is the first time this coefficient becomes significant
						val := int32(1) << uint(t1.bitplane)
						if signBit != 0 {
							t1.flags[idx] |= T1_SIGN
						}
						// Apply sign to value
						if t1.flags[idx]&T1_SIGN != 0 {
							t1.data[idx] = -val
						} else {
							t1.data[idx] = val
						}

						// Mark as significant
						t1.flags[idx] |= T1_SIG | T1_VISIT

						// Update neighbor flags
						t1.updateNeighborFlags(i, y, idx)
					} else {
						// Already-significant coefficient - update bit-plane value
						// Encoder encoded the bit-plane value, no sign bit
						absVal := t1.data[idx]
						if absVal < 0 {
							absVal = -absVal
						}
						absVal |= (1 << uint(t1.bitplane))
						if t1.flags[idx]&T1_SIGN != 0 {
							t1.data[idx] = -absVal
						} else {
							t1.data[idx] = absVal
						}
					}
				}

				// VISIT flag will be cleared at start of next bitplane
			}
		}
	}

	// DEBUG: Print significance count for cleanup pass
	if sigCount > 0 {
		fmt.Printf("[CP DEBUG] bitplane=%d, sigCount=%d\n", t1.bitplane, sigCount)
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
