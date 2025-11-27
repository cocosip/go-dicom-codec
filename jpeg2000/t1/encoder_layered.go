package t1

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// PassData represents encoded data for a single coding pass
// Following OpenJPEG's design: rate is cumulative bytes, len is incremental
type PassData struct {
	PassIndex int     // Index of this pass (0-based)
	Bitplane  int     // Bit-plane this pass belongs to
	PassType  int     // 0=SPP, 1=MRP, 2=CP
	Rate      int     // Cumulative bytes up to and including this pass
	Len       int     // Length of this pass in bytes (Rate[i] - Rate[i-1])
	Distortion float64 // Cumulative distortion (for rate-distortion optimization)
}

// EncodeLayered encodes a code-block with per-pass data separation
// This enables layer allocation for quality-progressive encoding
// Following OpenJPEG's implementation
//
// Returns:
// - passes: array of PassData with rate/distortion info
// - encodedData: complete MQ-encoded data for all passes
// - error: any encoding error
func (t1 *T1Encoder) EncodeLayered(data []int32, numPasses int, roishift int) ([]PassData, []byte, error) {
	if len(data) != t1.width*t1.height {
		return nil, nil, fmt.Errorf("data size mismatch: expected %d, got %d",
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

	// Result array
	passes := make([]PassData, 0, numPasses)

	// Encode bit-planes from MSB to LSB
	passIdx := 0
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; t1.bitplane-- {
		// Clear VISIT flags at start of each bitplane
		paddedWidth := t1.width + 2
		paddedHeight := t1.height + 2
		for i := 0; i < paddedWidth*paddedHeight; i++ {
			t1.flags[i] &^= T1_VISIT
		}

		// Check if this bit-plane needs encoding
		if t1.roishift > 0 && t1.bitplane >= t1.roishift {
			continue
		}

		// Three coding passes per bit-plane
		// 1. Significance Propagation Pass (SPP)
		if passIdx < numPasses {
			if err := t1.encodeSigPropPass(); err != nil {
				return nil, nil, fmt.Errorf("significance propagation pass failed: %w", err)
			}

			// Record cumulative rate (following OpenJPEG)
			passData := PassData{
				PassIndex:  passIdx,
				Bitplane:   t1.bitplane,
				PassType:   0, // SPP
				Rate:       t1.mqe.NumBytes(),
				Distortion: 0, // TODO: calculate distortion
			}
			passes = append(passes, passData)
			passIdx++
		}

		// 2. Magnitude Refinement Pass (MRP)
		if passIdx < numPasses {
			if err := t1.encodeMagRefPass(); err != nil {
				return nil, nil, fmt.Errorf("magnitude refinement pass failed: %w", err)
			}

			passData := PassData{
				PassIndex:  passIdx,
				Bitplane:   t1.bitplane,
				PassType:   1, // MRP
				Rate:       t1.mqe.NumBytes(),
				Distortion: 0, // TODO: calculate distortion
			}
			passes = append(passes, passData)
			passIdx++
		}

		// 3. Cleanup Pass (CP)
		if passIdx < numPasses {
			if err := t1.encodeCleanupPass(); err != nil {
				return nil, nil, fmt.Errorf("cleanup pass failed: %w", err)
			}

			passData := PassData{
				PassIndex:  passIdx,
				Bitplane:   t1.bitplane,
				PassType:   2, // CP
				Rate:       t1.mqe.NumBytes(),
				Distortion: 0, // TODO: calculate distortion
			}
			passes = append(passes, passData)
			passIdx++
		}

		// Reset context if required
		if t1.resetctx && passIdx < numPasses {
			t1.mqe.ResetContexts()
		}
	}

	// Final flush to complete MQ encoding
	fullMQData := t1.mqe.Flush()

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
