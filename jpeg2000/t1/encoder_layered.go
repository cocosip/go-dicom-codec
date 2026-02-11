package t1

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// PassData represents encoded data for a single coding pass
// Following OpenJPEG's design: rate is cumulative bytes, len is incremental
type PassData struct {
	PassIndex   int     // Index of this pass (0-based)
	Bitplane    int     // Bit-plane this pass belongs to
	PassType    int     // 0=SPP, 1=MRP, 2=CP
	Rate        int     // Cumulative bytes for R-D optimization (includes rate_extra_bytes)
	ActualBytes int     // Actual cumulative bytes in buffer (for slicing data)
	Len         int     // Length of this pass in bytes (Rate[i] - Rate[i-1])
	Distortion  float64 // Cumulative distortion (for rate-distortion optimization)
}

// isLayerBoundary checks if a pass index is a layer boundary
func isLayerBoundary(passIdx int, layerBoundaries []int) bool {
	for _, boundary := range layerBoundaries {
		if passIdx == boundary-1 { // boundary is 1-indexed (num passes), passIdx is 0-indexed
			return true
		}
	}
	return false
}

func shouldTerminateLayer(passIdx int, layerBoundaries []int, cblksty uint8) bool {
	if (cblksty & CblkStyleTermAll) != 0 {
		return true
	}
	if len(layerBoundaries) > 1 && isLayerBoundary(passIdx, layerBoundaries) {
		return true
	}
	return false
}

// EncodeLayered encodes a code-block with per-pass data separation
// This enables layer allocation for quality-progressive encoding
// Following OpenJPEG's implementation
//
// Parameters:
// - data: coefficient data to encode
// - numPasses: number of passes to encode
// - roishift: ROI bitplane shift
// - layerBoundaries: pass indices that end each layer (for selective termination)
// - cblksty: code-block style flags (0x04 = TERMALL, 0x02 = RESET)
//
// Returns:
// - passes: array of PassData with rate/distortion info
// - encodedData: complete MQ-encoded data for all passes
// - error: any encoding error
func (t1 *T1Encoder) EncodeLayered(data []int32, numPasses int, roishift int, layerBoundaries []int, cblksty uint8) ([]PassData, []byte, error) {
	if len(data) != t1.width*t1.height {
		return nil, nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			t1.width*t1.height, len(data))
	}

	t1.cblkstyle = int(cblksty)
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
	if maxBitplane < 0 {
		// All coefficients are zero - return empty pass
		return []PassData{{
			PassIndex:  0,
			Bitplane:   0,
			PassType:   2, // CP
			Rate:       0,
			Len:        0,
			Distortion: 0,
		}}, []byte{}, nil
	}

	// Initialize MQ encoder
	t1.mqe = mqc.NewMQEncoder(NUM_CONTEXTS)

	// Set initial context states (match OpenJPEG's opj_mqc_setstate calls)
	// These initial states optimize encoding by providing better probability estimates
	// State byte format: bits 0-6 = state number, bit 7 = MPS value
	t1.mqe.SetContextState(CTX_UNI, 46) // Uniform context: state 46, MPS=0
	t1.mqe.SetContextState(CTX_RL, 3)   // Run-length/Aggregate context: state 3, MPS=0
	t1.mqe.SetContextState(0, 4)        // Zero-coding context 0: state 4, MPS=0

	// Result array
	passes := make([]PassData, 0, numPasses)

	// Calculate initial distortion (all bits unencoded)
	// This is the baseline: sum of squared coefficients
	initialDist := calculateDistortion(t1.data, t1.width, t1.height, maxBitplane+1, 0)
	cumDistReduction := 0.0 // Cumulative distortion reduction from initial state

	// Encode passes using OpenJPEG sequencing.
	passIdx := 0
	passType := 2
	prevTerminated := false
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; {
		startBitplane := passType == 0 || (passType == 2 && passIdx == 0)
		if startBitplane {
			// Clear VISIT flags at start of each bitplane
			paddedWidth := t1.width + 2
			paddedHeight := t1.height + 2
			for i := 0; i < paddedWidth*paddedHeight; i++ {
				t1.flags[i] &^= T1_VISIT
			}

			// Check if this bit-plane needs encoding
			if t1.roishift > 0 && t1.bitplane >= t1.roishift {
				passType = 0
				t1.bitplane--
				continue
			}
		}

		raw := isLazyRawPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle)
		t1.setTracePassState(passIdx, passType, raw)
		if prevTerminated {
			if raw {
				t1.mqe.BypassInitEnc()
			} else {
				t1.mqe.RestartInitEnc()
			}
			prevTerminated = false
		}

		switch passType {
		case 0:
			if err := t1.encodeSigPropPass(raw); err != nil {
				return nil, nil, fmt.Errorf("significance propagation pass failed: %w", err)
			}
		case 1:
			if err := t1.encodeMagRefPass(raw); err != nil {
				return nil, nil, fmt.Errorf("magnitude refinement pass failed: %w", err)
			}
		case 2:
			if err := t1.encodeCleanupPass(); err != nil {
				return nil, nil, fmt.Errorf("cleanup pass failed: %w", err)
			}
			if (t1.cblkstyle & CblkStyleSegsym) != 0 {
				t1.mqe.SegmarkEnc()
			}
		}

		shouldTerminate := isTerminatingPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle) || shouldTerminateLayer(passIdx, layerBoundaries, cblksty)
		if shouldTerminate {
			if raw {
				t1.mqe.BypassFlushEnc((t1.cblkstyle & CblkStylePterm) != 0)
			} else if (t1.cblkstyle & CblkStylePterm) != 0 {
				t1.mqe.ErtermEnc()
			} else {
				t1.mqe.FlushToOutput()
			}
			prevTerminated = true
		}

		// RESET flag: reset contexts after each pass
		if (cblksty & CblkStyleReset) != 0 {
			t1.mqe.ResetContexts()
			t1.mqe.SetContextState(CTX_UNI, 46)
			t1.mqe.SetContextState(CTX_RL, 3)
			t1.mqe.SetContextState(CTX_ZC_START, 4)
		}

		actualBytes := t1.mqe.NumBytes()
		rate := actualBytes
		if !shouldTerminate {
			if raw {
				rate += t1.mqe.BypassExtraBytes((t1.cblkstyle & CblkStylePterm) != 0)
			} else {
				rate += 3
			}
		}

		remainingDist := calculateDistortion(t1.data, t1.width, t1.height, t1.bitplane, passType)
		cumDistReduction = initialDist - remainingDist

		passData := PassData{
			PassIndex:   passIdx,
			Bitplane:    t1.bitplane,
			PassType:    passType,
			Rate:        rate,
			ActualBytes: actualBytes,
			Distortion:  cumDistReduction,
		}
		passes = append(passes, passData)
		passIdx++

		if passType == 2 {
			passType = 0
			t1.bitplane--
		} else {
			passType++
		}
	}

	// Get final MQ data
	var fullMQData []byte
	if prevTerminated {
		fullMQData = t1.mqe.GetBuffer()
	} else {
		fullMQData = t1.mqe.Flush()
	}

	// Calculate Len for each pass (Rate[i] - Rate[i-1])
	// Following OpenJPEG's implementation
	for i := range passes {
		if i == 0 {
			passes[i].Len = passes[i].Rate
		} else {
			passes[i].Len = passes[i].Rate - passes[i-1].Rate
		}
	}

	// Return passes with rate/distortion info and the complete MQ data
	return passes, fullMQData, nil
}

// rdDistortionDelta approximates distortion reduction contributed by a pass (DEPRECATED).
// This is the old estimation formula. Use calculateDistortion() for accurate results.
func rdDistortionDelta(bitplane int, passType int) float64 {
	base := math.Pow(2.0, float64(2*(bitplane+1)))
	switch passType {
	case 0: // SPP
		return base
	case 1: // MRP
		return base * 0.5
	default: // CP
		return base * 0.75
	}
}

// calculateDistortion computes accurate distortion based on reconstruction error.
// This follows the JPEG 2000 standard approach (ISO/IEC 15444-1 Annex J).
//
// Distortion is measured as the sum of squared errors (SSE) between original
// and reconstructed coefficients. After encoding bitplane b, the reconstructed
// value has precision down to bitplane b, and all bits below b are unknown (set to 0).
//
// The distortion is: D = sum((original - reconstructed)^2) for all coefficients.
// Where reconstructed value has all bits below current bitplane masked to 0.
//
// Parameters:
//   - data: original coefficient data (with padding)
//   - width, height: code-block dimensions (without padding)
//   - currentBitplane: bitplane just encoded (0 = LSB)
//   - passType: 0=SPP, 1=MRP, 2=CP (affects which bits are considered refined)
//
// Returns: total distortion (SSE) remaining after this pass
func calculateDistortion(data []int32, width, height int, currentBitplane int, passType int) float64 {
	if currentBitplane < 0 {
		// All bits encoded, distortion is 0
		return 0.0
	}

	paddedWidth := width + 2
	distortion := 0.0

	// For each coefficient, calculate the error due to unencoded bits
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			original := data[idx]

			// Reconstructed value: original with bits below current bitplane masked to 0
			// After encoding bitplane b, we have precision down to bit b
			// Bits b-1, b-2, ..., 0 are still unknown (contribute to distortion)

			// Mask for bits below current bitplane
			var reconstructed int32
			if currentBitplane < 31 {
				// Keep sign and all bits at or above current bitplane
				sign := int32(0)
				if original < 0 {
					sign = -1
					original = -original
				}

				// Mask to keep bits >= currentBitplane
				// For bitplane b, we want to keep bits [31..b] and zero out [b-1..0]
				mask := int32(-1) << uint(currentBitplane)
				reconstructed = (original & mask)

				// For MRP and CP within a bitplane, we have better reconstruction
				// SPP only codes significance, MRP/CP refine the magnitude
				// Add a correction for refinement passes
				if passType > 0 && currentBitplane > 0 {
					// Refinement passes reduce uncertainty in current bitplane
					// Approximate reconstructed value at bitplane center
					correction := int32(1) << uint(currentBitplane-1)
					reconstructed |= correction
				}

				if sign < 0 {
					reconstructed = -reconstructed
					original = -original // Restore
				}
			} else {
				reconstructed = original // All bits encoded
			}

			// Squared error
			diff := float64(original - reconstructed)
			distortion += diff * diff
		}
	}

	return distortion
}
