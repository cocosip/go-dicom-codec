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
	t1.cblkstyle = cblkstyle
	t1.resetctx = (cblkstyle & 0x01) != 0     // Selective arithmetic coding bypass
	t1.termall = (cblkstyle & 0x02) != 0      // Reset context probabilities
	t1.segmentation = (cblkstyle & 0x04) != 0 // Segmentation symbols

	return t1
}

// DecodeWithBitplane decodes a code-block starting from a specific bitplane
// This is used when the max bitplane is known (e.g., from packet header in T2)
func (t1 *T1Decoder) DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty code-block data")
	}

	t1.roishift = roishift

	// DEBUG: Log resetctx
	fmt.Printf("[T1-DEC] resetctx=%v\n", t1.resetctx)

	// Initialize MQ decoder
	t1.mqc = mqc.NewMQDecoder(data, NUM_CONTEXTS)

	// Decode bit-planes from MSB to LSB
	// Each bit-plane has up to 3 coding passes
	passIdx := 0
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Check if this bit-plane needs decoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			// Skip this bit-plane (ROI region)
			continue
		}

		// DEBUG: Log first coefficient value at start of bit-plane
		paddedWidth := t1.width + 2
		firstIdx := paddedWidth + 1
		fmt.Printf("[BP %d START] First coeff data=%d\n", t1.bitplane, t1.data[firstIdx])

		// Three coding passes per bit-plane:
		// 1. Significance Propagation Pass (SPP)
		// 2. Magnitude Refinement Pass (MRP)
		// 3. Cleanup Pass (CP)

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

	// Decode bit-planes from MSB to LSB
	// Each bit-plane has up to 3 coding passes
	passIdx := 0
	for t1.bitplane = startBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Check if this bit-plane needs decoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			// Skip this bit-plane (ROI region)
			continue
		}

		// Three coding passes per bit-plane:
		// 1. Significance Propagation Pass (SPP)
		// 2. Magnitude Refinement Pass (MRP)
		// 3. Cleanup Pass (CP)

		fmt.Printf("[T1-DEC] BP %d: passIdx=%d, numPasses=%d\n", t1.bitplane, passIdx, numPasses)

		if passIdx < numPasses {
			fmt.Printf("[T1-DEC]   Executing SPP\n")
			if err := t1.decodeSigPropPass(); err != nil {
				return fmt.Errorf("significance propagation pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			fmt.Printf("[T1-DEC]   Executing MRP\n")
			if err := t1.decodeMagRefPass(); err != nil {
				return fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			fmt.Printf("[T1-DEC]   Executing CP\n")
			if err := t1.decodeCleanupPass(); err != nil {
				return fmt.Errorf("cleanup pass failed: %w", err)
			}
			passIdx++
			fmt.Printf("[T1-DEC]   CP done, passIdx=%d\n", passIdx)
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

	// DEBUG: Track if SPP processes anything for first coefficient
	sppProcessed := false

	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			flags := t1.flags[idx]

			// DEBUG: Track first coefficient
			isFirst := (x == 0 && y == 0)

			// Skip if already significant
			if flags&T1_SIG != 0 {
				continue
			}

			// Check if has significant neighbor
			if flags&T1_SIG_NEIGHBORS == 0 {
				continue
			}

			if isFirst {
				sppProcessed = true
				fmt.Printf("[SPP bp=%d] Processing first coeff: SIG_NEIGHBORS=1 ctx=", t1.bitplane)
			}

			// Decode significance bit
			ctx := getZeroCodingContext(flags)

			if isFirst {
				fmt.Printf("%d ", ctx)
			}

			bit := t1.mqc.Decode(int(ctx))

			if isFirst {
				fmt.Printf("sigBit=%d\n", bit)
			}

			if bit != 0 {
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
//			t1.flags[idx] &^= T1_VISIT
		}
	}

	// DEBUG: Log if SPP didn't process first coefficient
	if !sppProcessed {
		fmt.Printf("[SPP bp=%d] Skip first coeff (no SIG_NEIGHBORS or already SIG)\n", t1.bitplane)
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

			// DEBUG: Track first coefficient (0,0)
			isFirst := (x == 0 && y == 0)

			// Only refine significant coefficients not visited in this bit-plane
			if (flags&T1_SIG) == 0 || (flags&T1_VISIT) != 0 {
				if isFirst {
					fmt.Printf("[MRP bp=%d] Skip first coeff: SIG=%d VISIT=%d data=%d\n",
						t1.bitplane, flags&T1_SIG, flags&T1_VISIT, t1.data[idx])
				}
				continue
			}

			// Decode refinement bit
			ctx := getMagRefinementContext(flags)

			if isFirst {
				fmt.Printf("[MRP bp=%d] Process first coeff: data_before=%d ctx=%d ", t1.bitplane, t1.data[idx], ctx)
			}

			bit := t1.mqc.Decode(int(ctx))

			if isFirst {
				fmt.Printf("refBit=%d ", bit)
			}

			// Update coefficient magnitude
			if bit != 0 {
				if t1.data[idx] >= 0 {
					t1.data[idx] += int32(1) << uint(t1.bitplane)
				} else {
					t1.data[idx] -= int32(1) << uint(t1.bitplane)
				}
			}

			if isFirst {
				fmt.Printf("data_after=%d\n", t1.data[idx])
			}

			// Mark as refined
			t1.flags[idx] |= T1_REFINE

			// Clear visit flag (ready for next bit-plane)
//			t1.flags[idx] &^= T1_VISIT
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
						// All 4 remain insignificant, clear visit flags
						for dy := 0; dy < 4; dy++ {
							y := k + dy
							idx := (y+1)*paddedWidth + (i + 1)
							t1.flags[idx] &^= T1_VISIT
						}
						continue // Move to next column
					}

					// At least one is significant, decode uniformly which one
					runlen := 0
					runlen |= t1.mqc.Decode(CTX_UNI) << 1
					runlen |= t1.mqc.Decode(CTX_UNI)

					// Clear VISIT flags for all coefficients before runlen (they remain insignificant)
					for dy := 0; dy < runlen; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						t1.flags[idx] &^= T1_VISIT
					}

					// Process all coefficients from runlen to 3
					for dy := runlen; dy < 4; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						flags := t1.flags[idx]

						// DEBUG: Track first coefficient (0,0)
						isFirst := (i == 0 && y == 0)

						// Decode significance bit
						ctx := getZeroCodingContext(flags)

						if isFirst {
							fmt.Printf("[CP bp=%d] RL path: decode sigBit ctx=%d ", t1.bitplane, ctx)
						}

						bit := t1.mqc.Decode(int(ctx))

						if isFirst {
							fmt.Printf("sigBit=%d ", bit)
						}

						if bit != 0 {
						// Check if already significant
						alreadySig := (flags & T1_SIG) != 0

						if !alreadySig {
								// Coefficient becomes significant
								// Decode sign bit
								signBit := t1.mqc.Decode(CTX_UNI)
	
								if isFirst {
									fmt.Printf("signBit=%d ", signBit)
								}
	
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
	
								if isFirst {
									fmt.Printf("data=%d\n", t1.data[idx])
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
						} else if isFirst {
							fmt.Printf("insignificant\n")
						}

						// Clear visit flag
						t1.flags[idx] &^= T1_VISIT
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

				// DEBUG
				isFirst := (i == 0 && y == 0)

				// Skip if already visited
				if (flags & T1_VISIT) != 0 {
					if isFirst {
						fmt.Printf("[DEC CP bp=%d] Normal: skip (already visited)\n", t1.bitplane)
					}
					t1.flags[idx] &^= T1_VISIT
					continue
				}

				// Check if already significant
				alreadySig := (flags & T1_SIG) != 0

				if isFirst {
					fmt.Printf("[DEC CP bp=%d] Normal: alreadySig=%v decode sigBit ", t1.bitplane, alreadySig)
				}

				// Decode significance bit (always, even for already-significant coefficients)
				ctx := getZeroCodingContext(flags)
				bit := t1.mqc.Decode(int(ctx))

				if isFirst {
					fmt.Printf("ctx=%d bit=%d\n", ctx, bit)
				}

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

				// Clear visit flag
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
	// North
	if y > 0 {
		nIdx := (y)*paddedWidth + (x + 1)
		t1.flags[nIdx] |= T1_SIG_S
		if sign != 0 {
			t1.flags[nIdx] |= T1_SIGN_S
		}
	}

	// South
	if y < t1.height-1 {
		sIdx := (y+2)*paddedWidth + (x + 1)
		t1.flags[sIdx] |= T1_SIG_N
		if sign != 0 {
			t1.flags[sIdx] |= T1_SIGN_N
		}
	}

	// West
	if x > 0 {
		wIdx := (y+1)*paddedWidth + x
		t1.flags[wIdx] |= T1_SIG_E
		if sign != 0 {
			t1.flags[wIdx] |= T1_SIGN_E
		}
	}

	// East
	if x < t1.width-1 {
		eIdx := (y+1)*paddedWidth + (x + 2)
		t1.flags[eIdx] |= T1_SIG_W
		if sign != 0 {
			t1.flags[eIdx] |= T1_SIGN_W
		}
	}

	// Northwest
	if y > 0 && x > 0 {
		t1.flags[(y)*paddedWidth+x] |= T1_SIG_SE
	}

	// Northeast
	if y > 0 && x < t1.width-1 {
		t1.flags[(y)*paddedWidth+(x+2)] |= T1_SIG_SW
	}

	// Southwest
	if y < t1.height-1 && x > 0 {
		t1.flags[(y+2)*paddedWidth+x] |= T1_SIG_NE
	}

	// Southeast
	if y < t1.height-1 && x < t1.width-1 {
		t1.flags[(y+2)*paddedWidth+(x+2)] |= T1_SIG_NW
	}
}
