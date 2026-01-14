package t1

import (
	"testing"
)

// TestTERMALLBytesComparison compares normal vs TERMALL encoding byte-by-byte
func TestTERMALLBytesComparison(t *testing.T) {
	// Use a very small 4x4 block for easier analysis
	width, height := 4, 4
	data := make([]int32, width*height)

	// Simple pattern: first 4 values are 1, rest are 0
	// This should result in minimal encoding
	data[0] = 1
	data[1] = 1
	data[2] = 1
	data[3] = 1

	// Calculate max bitplane
	maxBitplane := 0 // Only bit 0 is set

	numPasses := (maxBitplane + 1) * 3 // 3 passes
	t.Logf("Testing %dx%d block, maxBitplane=%d, numPasses=%d", width, height, maxBitplane, numPasses)

	// Encode WITHOUT TERMALL (normal mode)
	encoder1 := NewT1Encoder(width, height, 0)
	encoded1, err := encoder1.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Normal encode failed: %v", err)
	}
	t.Logf("Normal encode: %d bytes", len(encoded1))
	t.Logf("Normal bytes: % 02x", encoded1)

	// Decode normal to verify
	decoder1 := NewT1Decoder(width, height, 0)
	if err := decoder1.DecodeWithBitplane(encoded1, numPasses, maxBitplane, 0); err != nil {
		t.Fatalf("Normal decode failed: %v", err)
	}
	decoded1 := decoder1.GetData()

	maxError1 := int32(0)
	for i := 0; i < len(data); i++ {
		diff := data[i] - decoded1[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError1 {
			maxError1 = diff
		}
	}
	t.Logf("Normal mode: error=%d", maxError1)

	// Encode WITH TERMALL
	encoder2 := NewT1Encoder(width, height, 0)
	layerBoundaries := []int{numPasses}
	passes, completeData, err := encoder2.EncodeLayered(data, numPasses, 0, layerBoundaries, 0x04)
	if err != nil {
		t.Fatalf("TERMALL encode failed: %v", err)
	}
	t.Logf("TERMALL encode: %d bytes, %d passes", len(completeData), len(passes))
	t.Logf("TERMALL bytes: % 02x", completeData)

	// Print each pass's bytes
	prevEnd := 0
	for i, p := range passes {
		currentEnd := p.ActualBytes
		passBytes := completeData[prevEnd:currentEnd]
		t.Logf("  Pass %d (type=%d): bytes[%d:%d] = % 02x", i, p.PassType, prevEnd, currentEnd, passBytes)
		prevEnd = currentEnd
	}

	// Extract pass lengths
	passLengths := make([]int, len(passes))
	for i, p := range passes {
		passLengths[i] = p.ActualBytes
	}

	// Decode WITH TERMALL using DecodeLayered
	decoder2 := NewT1Decoder(width, height, 0)
	if err := decoder2.DecodeLayered(completeData, passLengths, maxBitplane, 0); err != nil {
		t.Fatalf("TERMALL decode failed: %v", err)
	}
	decoded2 := decoder2.GetData()

	maxError2 := int32(0)
	for i := 0; i < len(data); i++ {
		diff := data[i] - decoded2[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError2 {
			maxError2 = diff
			if maxError2 > 0 {
				t.Logf("  Error at i=%d: expected=%d got=%d", i, data[i], decoded2[i])
			}
		}
	}
	t.Logf("TERMALL mode: error=%d", maxError2)

	if maxError2 > 0 {
		t.Errorf("TERMALL mode has error %d, expected 0", maxError2)
	}

	// EXPERIMENT: Try decoding just the last pass (CP) which has the real data
	t.Logf("\n=== Experiment: Decode only CP pass ===")
	decoder3 := NewT1Decoder(width, height, 0)

	// Extract only CP pass data (pass 2)
	cpData := completeData[5:10] // "00 ba b5 44 df"
	t.Logf("CP data only: % 02x", cpData)

	// Try to decode with just this pass
	if err := decoder3.DecodeLayered(cpData, []int{len(cpData)}, maxBitplane, 0); err != nil {
		t.Logf("CP-only decode failed: %v", err)
	} else {
		decoded3 := decoder3.GetData()
		maxError3 := int32(0)
		for i := 0; i < len(data); i++ {
			diff := data[i] - decoded3[i]
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError3 {
				maxError3 = diff
			}
		}
		t.Logf("CP-only decode: error=%d", maxError3)
	}

	// EXPERIMENT 2: Try decoding normal data with DecodeLayered
	t.Logf("\n=== Experiment: Decode normal data with DecodeLayered ===")
	decoder4 := NewT1Decoder(width, height, 0)
	t.Logf("Normal data: % 02x", encoded1)

	// Try with 3 passes (SPP, MRP, CP for bitplane 0), useTERMALL=false
	if err := decoder4.DecodeLayeredWithMode(encoded1, []int{len(encoded1), len(encoded1), len(encoded1)}, maxBitplane, 0, false, false); err != nil {
		t.Logf("Normal-via-Layered decode failed: %v", err)
	} else {
		decoded4 := decoder4.GetData()
		maxError4 := int32(0)
		for i := 0; i < len(data); i++ {
			diff := data[i] - decoded4[i]
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError4 {
				maxError4 = diff
			}
		}
		t.Logf("Normal-via-Layered decode (3 passes): error=%d", maxError4)

		// Print decoded values
		t.Logf("Decoded values: %v", decoded4[:16])
	}

	// EXPERIMENT 3: Decode normal data with DecodeWithBitplane for comparison
	t.Logf("\n=== Experiment: Decode normal data with DecodeWithBitplane ===")
	decoder5 := NewT1Decoder(width, height, 0)
	if err := decoder5.DecodeWithBitplane(encoded1, 3, maxBitplane, 0); err != nil {
		t.Logf("Normal-via-Bitplane decode failed: %v", err)
	} else {
		decoded5 := decoder5.GetData()
		maxError5 := int32(0)
		for i := 0; i < len(data); i++ {
			diff := data[i] - decoded5[i]
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError5 {
				maxError5 = diff
			}
		}
		t.Logf("Normal-via-Bitplane decode: error=%d", maxError5)

		// Print decoded values
		t.Logf("Decoded values: %v", decoded5[:16])
	}
}
