package t1

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// T1Encoder implements EBCOT Tier-1 encoding
// Reference: ISO/IEC 15444-1:2019 Annex D
type T1Encoder struct {
	// Code-block dimensions
	width  int
	height int

	// Wavelet coefficients (input)
	// Stored in row-major order
	data []int32

	// State flags for each coefficient
	// Stores significance, refinement, visit flags and neighbor info
	flags []uint32

	// MQ encoder
	mqe *mqc.MQEncoder

	// Current bit-plane being encoded
	bitplane int

	// Encoding parameters
	roishift     int  // ROI shift value
	cblkstyle    int  // Code-block style flags
	resetctx     bool // Reset context on each pass
	termall      bool // Terminate all passes
	segmentation bool // Use segmentation symbols
}

// NewT1Encoder creates a new Tier-1 encoder
func NewT1Encoder(width, height int, cblkstyle int) *T1Encoder {
	// Add padding for boundary conditions (1 pixel on each side)
	paddedWidth := width + 2
	paddedHeight := height + 2

	t1 := &T1Encoder{
		width:  width,
		height: height,
		flags:  make([]uint32, paddedWidth*paddedHeight),
	}

	// Parse code-block style flags
	t1.cblkstyle = cblkstyle
	t1.resetctx = (cblkstyle & 0x01) != 0     // Selective arithmetic coding bypass
	t1.termall = (cblkstyle & 0x02) != 0      // Reset context probabilities
	t1.segmentation = (cblkstyle & 0x04) != 0 // Segmentation symbols

	return t1
}

// Encode encodes a code-block
//
// Performance notes:
// - Most computationally intensive part of JPEG 2000 encoding
// - Processes coefficients bit-plane by bit-plane (MSB to LSB)
// - Three coding passes per bit-plane (Sig Prop, Mag Ref, Cleanup)
// - Context modeling using 8-neighbor flags (cached for speed)
// - MQ encoding is the inner loop bottleneck
// - Typical workload: 32x32 block = 1024 coefficients × 12-16 bit-planes
func (t1 *T1Encoder) Encode(data []int32, numPasses int, roishift int) ([]byte, error) {
	if len(data) != t1.width*t1.height {
		return nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			t1.width*t1.height, len(data))
	}

	t1.roishift = roishift

	// Copy data with padding
	t1.data = make([]int32, (t1.width+2)*(t1.height+2))
	paddedWidth := t1.width + 2
	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			t1.data[idx] = data[y*t1.width+x]
		}
	}

	// Determine maximum bit-plane
	maxBitplane := t1.findMaxBitplane()
	// DEBUG removed
	if maxBitplane < 0 {
		// All coefficients are zero
		t1.mqe = mqc.NewMQEncoder(NUM_CONTEXTS)
		return t1.mqe.Flush(), nil
	}

	// Initialize MQ encoder
	t1.mqe = mqc.NewMQEncoder(NUM_CONTEXTS)

	// Encode bit-planes from MSB to LSB
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

		// Check if this bit-plane needs encoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			// Skip this bit-plane (ROI region)
			continue
		}

		// Three coding passes per bit-plane:
		// 1. Significance Propagation Pass (SPP)
		// 2. Magnitude Refinement Pass (MRP)
		// 3. Cleanup Pass (CP)

		if passIdx < numPasses {
			if err := t1.encodeSigPropPass(); err != nil {
				return nil, fmt.Errorf("significance propagation pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			if err := t1.encodeMagRefPass(); err != nil {
				return nil, fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
			passIdx++
		}

		if passIdx < numPasses {
			if err := t1.encodeCleanupPass(); err != nil {
				return nil, fmt.Errorf("cleanup pass failed: %w", err)
			}
			passIdx++
		}

		// DEBUG removed

		// Reset context if required
		if t1.resetctx && passIdx < numPasses {
			t1.mqe.ResetContexts()
		}
	}

	// Flush MQ encoder
	return t1.mqe.Flush(), nil
}

// findMaxBitplane finds the maximum bit-plane that contains significant bits
func (t1 *T1Encoder) findMaxBitplane() int {
	maxAbs := int32(0)

	// Find maximum absolute value
	for _, val := range t1.data {
		if val < 0 {
			if -val > maxAbs {
				maxAbs = -val
			}
		} else {
			if val > maxAbs {
				maxAbs = val
			}
		}
	}

	if maxAbs == 0 {
		return -1 // All zeros
	}

	// Find the highest bit set
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}
	return bitplane - 1
}

// encodeSigPropPass encodes the Significance Propagation Pass
// This pass encodes coefficients that:
// - Are not yet significant
// - Have at least one significant neighbor
func (t1 *T1Encoder) encodeSigPropPass() error {
	paddedWidth := t1.width + 2

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

			// Check if coefficient is significant at this bit-plane
			absVal := t1.data[idx]
			if absVal < 0 {
				absVal = -absVal
			}
			isSig := (absVal >> uint(t1.bitplane)) & 1

			// Encode significance bit
			ctx := getZeroCodingContext(flags)
			t1.mqe.Encode(int(isSig), int(ctx))

			if isSig != 0 {
				// Coefficient becomes significant
				// Encode sign bit with prediction
				signBit := 0
				if t1.data[idx] < 0 {
					signBit = 1
					t1.flags[idx] |= T1_SIGN
				}

				signCtx := getSignCodingContext(flags)
				signPred := getSignPrediction(flags)
				t1.mqe.Encode(signBit^signPred, int(signCtx))

				// Mark as significant and visited (prevent MRP from processing)
				t1.flags[idx] |= T1_SIG | T1_VISIT

				// Update neighbor flags
				t1.updateNeighborFlags(x, y, idx)
			}
			// Note: Do not clear VISIT here - it prevents MRP from re-processing
		}
	}

	return nil
}

// encodeMagRefPass encodes the Magnitude Refinement Pass
// This pass refines coefficients that are already significant
func (t1 *T1Encoder) encodeMagRefPass() error {
	paddedWidth := t1.width + 2

	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			flags := t1.flags[idx]

			// Only refine significant coefficients not visited in this bit-plane
			if (flags&T1_SIG) == 0 || (flags&T1_VISIT) != 0 {
				continue
			}

			// Get refinement bit at current bit-plane
			absVal := t1.data[idx]
			if absVal < 0 {
				absVal = -absVal
			}
			refBit := (absVal >> uint(t1.bitplane)) & 1

			// Encode refinement bit
			ctx := getMagRefinementContext(flags)
			t1.mqe.Encode(int(refBit), int(ctx))

			// Mark as refined and visited (so CP won't refine again)
			t1.flags[idx] |= T1_REFINE | T1_VISIT
		}
	}

	return nil
}

// encodeCleanupPass encodes the Cleanup Pass
// This pass encodes all remaining coefficients not encoded in previous passes
// IMPORTANT: Process in VERTICAL order (column-first) with 4-row groups for RL encoding
// This matches OpenJPEG's opj_t1_enc_clnpass() implementation
func (t1 *T1Encoder) encodeCleanupPass() error {
	paddedWidth := t1.width + 2

	// Process in groups of 4 rows (vertical RL encoding)
	for k := 0; k < t1.height; k += 4 {
		for i := 0; i < t1.width; i++ {
			// Try run-length encoding for this column (4 vertical coefficients)
			// Only if all 4 rows are available
			if k+3 < t1.height {
				// Check if run-length coding can be applied to this 4-coeff vertical run
				canUseRL := true
				rlSigPos := -1 // Position (0-3) of first significant coeff in vertical run

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

					// Check if this coefficient is significant at current bitplane
					if rlSigPos == -1 {
						absVal := t1.data[idx]
						if absVal < 0 {
							absVal = -absVal
						}
						if ((absVal >> uint(t1.bitplane)) & 1) != 0 {
							rlSigPos = dy
						}
					}
				}

				if canUseRL {
					// Encode run-length bit (0 = all insignificant, 1 = at least one significant)
					rlBit := 0
					if rlSigPos >= 0 {
						rlBit = 1
					}
					t1.mqe.Encode(rlBit, CTX_RL)

					if rlBit == 0 {
						// All 4 remain insignificant
						// VISIT flags will be cleared at start of next bitplane
						continue // Move to next column
					}

					t1.mqe.Encode((rlSigPos>>1)&1, CTX_UNI)
					t1.mqe.Encode(rlSigPos&1, CTX_UNI)

					// Coefficients before rlSigPos remain insignificant
					// VISIT flags will be cleared at start of next bitplane

					// Process all coefficients from rlSigPos to 3
					for dy := rlSigPos; dy < 4; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						flags := t1.flags[idx]

						// Check if coefficient is significant at this bit-plane
						absVal := t1.data[idx]
						if absVal < 0 {
							absVal = -absVal
						}
						isSig := (absVal >> uint(t1.bitplane)) & 1

						// Encode significance bit
						ctx := getZeroCodingContext(flags)

						t1.mqe.Encode(int(isSig), int(ctx))

						if isSig != 0 {
							// Check if already significant
							alreadySig := (flags & T1_SIG) != 0

							if !alreadySig {
								// Coefficient becomes significant
								// Encode sign bit (uniform context in cleanup pass)
								signBit := 0
								if t1.data[idx] < 0 {
									signBit = 1
									t1.flags[idx] |= T1_SIGN
								}

								t1.mqe.Encode(signBit, CTX_UNI)

								// Mark as significant
								t1.flags[idx] |= T1_SIG | T1_VISIT

								// Update neighbor flags
								t1.updateNeighborFlags(i, y, idx)
							}
							// If already significant: bit was just a refinement, no sign bit needed
						}

						// VISIT flag will be cleared at start of next bitplane
					}

					continue // RL encoding handled this column, move to next
				}
			}

			// Normal processing (not part of RL encoding, or partial row group)
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

				// Check if coefficient is significant at this bit-plane
				absVal := t1.data[idx]
				if absVal < 0 {
					absVal = -absVal
				}
				isSig := (absVal >> uint(t1.bitplane)) & 1

				// Encode significance bit
				ctx := getZeroCodingContext(flags)
				t1.mqe.Encode(int(isSig), int(ctx))

				if isSig != 0 {
					// Check if already significant
					alreadySig := (flags & T1_SIG) != 0

					if !alreadySig {
						// Coefficient becomes significant
						// Encode sign bit (uniform context in cleanup pass)
						signBit := 0
						if t1.data[idx] < 0 {
							signBit = 1
							t1.flags[idx] |= T1_SIGN
						}
						t1.mqe.Encode(signBit, CTX_UNI)

						// Mark as significant
						t1.flags[idx] |= T1_SIG | T1_VISIT

						// Update neighbor flags
						t1.updateNeighborFlags(i, y, idx)
					}
					// If already significant: bit was just a refinement, no sign bit needed
				}

				// VISIT flag will be cleared at start of next bitplane
			}
		}
	}

	return nil
}

// updateNeighborFlags updates the neighbor significance flags
// when a coefficient becomes significant
func (t1 *T1Encoder) updateNeighborFlags(x, y, idx int) {
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

// ComputeDistortion computes the distortion for rate-distortion optimization
// This is a simplified version - full implementation would use MSE reduction tables
func (t1 *T1Encoder) ComputeDistortion() float64 {
	distortion := 0.0

	paddedWidth := t1.width + 2
	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)

			// Compute quantization error
			// For now, use simple MSE calculation
			// Full implementation would use pre-computed tables
			val := float64(t1.data[idx])
			distortion += val * val
		}
	}

	return distortion
}

// GetRate returns the current encoding rate (in bytes)
func (t1 *T1Encoder) GetRate() int {
	if t1.mqe == nil {
		return 0
	}
	// This is an approximation - actual rate would need to flush and measure
	return 0 // Placeholder
}

// SetQuantization applies quantization to the coefficients
// This modifies the input data by quantizing based on step size
func SetQuantization(data []int32, stepSize float64) {
	if stepSize <= 0 {
		return
	}

	invStepSize := 1.0 / stepSize
	for i := range data {
		// Quantize: val = sign(val) * floor(abs(val) / stepSize)
		val := float64(data[i])
		if val >= 0 {
			data[i] = int32(math.Floor(val * invStepSize))
		} else {
			data[i] = -int32(math.Floor(-val * invStepSize))
		}
	}
}
