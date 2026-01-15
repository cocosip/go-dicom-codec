package t1

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// TestTERMALLDetailed provides detailed debugging of TERMALL decoding
func TestTERMALLDetailed(t *testing.T) {
	// Use a very simple case: 4x4 block with only first pixel = 1
	width, height := 4, 4
	data := make([]int32, width*height)
	data[0] = 1 // Only first pixel is 1, rest are 0

	numPasses := 3 // SPP, MRP, CP for bitplane 0

	t.Logf("Input data: %v", data)

	// First encode with NORMAL mode for comparison
	encoderNormal := NewT1Encoder(width, height, 0)
	encodedNormal, err := encoderNormal.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Normal encode failed: %v", err)
	}
	t.Logf("\nNormal mode encoding: %d bytes", len(encodedNormal))
	t.Logf("Normal data: % 02x", encodedNormal)

	// Decode normal to verify
	decoderNormal := NewT1Decoder(width, height, 0)
	if err := decoderNormal.DecodeWithBitplane(encodedNormal, numPasses, 0, 0); err != nil {
		t.Fatalf("Normal decode failed: %v", err)
	}
	decodedNormal := decoderNormal.GetData()
	t.Logf("Normal decoded: %v", decodedNormal[:16])

	// Encode with TERMALL
	encoder := NewT1Encoder(width, height, 0)
	layerBoundaries := []int{numPasses}
	passes, completeData, err := encoder.EncodeLayered(data, numPasses, 0, layerBoundaries, 0x04)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("\nEncoded %d passes, %d bytes total", len(passes), len(completeData))
	t.Logf("Complete data: % 02x", completeData)

	// Print each pass
	prevEnd := 0
	passLengths := make([]int, len(passes))
	for i, p := range passes {
		passLengths[i] = p.ActualBytes
		currentEnd := p.ActualBytes
		passBytes := completeData[prevEnd:currentEnd]
		t.Logf("Pass %d (bp=%d, type=%d): bytes[%d:%d] = % 02x",
			i, p.Bitplane, p.PassType, prevEnd, currentEnd, passBytes)
		prevEnd = currentEnd
	}

	// Now try to decode each pass individually and see what coefficients get set
	t.Logf("\n=== Decoding Each Pass Individually ===")

	// Decode pass 0 (SPP) only
	decoder0 := NewT1Decoder(width, height, 0)
	passData0 := completeData[0:passLengths[0]]
	t.Logf("\nPass 0 data: % 02x", passData0)
	decoder0.mqc = mqc.NewMQDecoder(passData0, NUM_CONTEXTS)
	// Set initial context states to match OpenJPEG
	decoder0.mqc.SetContextState(CTX_UNI, 46)
	decoder0.mqc.SetContextState(CTX_RL, 3)
	decoder0.mqc.SetContextState(0, 4)
	decoder0.roishift = 0
	decoder0.bitplane = 0
	if err := decoder0.decodeSigPropPass(); err != nil {
		t.Logf("Pass 0 decode failed: %v", err)
	} else {
		decoded0 := decoder0.GetData()
		t.Logf("After pass 0: %v", decoded0[:16])
	}

	// Decode pass 0 + 1 (SPP + MRP)
	decoder1 := NewT1Decoder(width, height, 0)
	// First do SPP
	decoder1.mqc = mqc.NewMQDecoder(completeData[0:passLengths[0]], NUM_CONTEXTS)
	decoder1.mqc.SetContextState(CTX_UNI, 46)
	decoder1.mqc.SetContextState(CTX_RL, 3)
	decoder1.mqc.SetContextState(0, 4)
	decoder1.roishift = 0
	decoder1.bitplane = 0
	decoder1.decodeSigPropPass()
	// Then do MRP with preserved contexts
	passData1 := completeData[passLengths[0]:passLengths[1]]
	t.Logf("\nPass 1 data: % 02x", passData1)
	prevContexts := decoder1.mqc.GetContexts()
	decoder1.mqc = mqc.NewMQDecoderWithContexts(passData1, prevContexts)
	if err := decoder1.decodeMagRefPass(); err != nil {
		t.Logf("Pass 1 decode failed: %v", err)
	} else {
		decoded1 := decoder1.GetData()
		t.Logf("After pass 0+1: %v", decoded1[:16])
	}

	// Decode all 3 passes (SPP + MRP + CP)
	decoder2 := NewT1Decoder(width, height, 0)
	// SPP
	decoder2.mqc = mqc.NewMQDecoder(completeData[0:passLengths[0]], NUM_CONTEXTS)
	decoder2.mqc.SetContextState(CTX_UNI, 46)
	decoder2.mqc.SetContextState(CTX_RL, 3)
	decoder2.mqc.SetContextState(0, 4)
	decoder2.roishift = 0
	decoder2.bitplane = 0
	decoder2.decodeSigPropPass()
	// MRP with preserved contexts
	prevContexts2 := decoder2.mqc.GetContexts()
	decoder2.mqc = mqc.NewMQDecoderWithContexts(completeData[passLengths[0]:passLengths[1]], prevContexts2)
	decoder2.decodeMagRefPass()
	// CP with preserved contexts
	passData2 := completeData[passLengths[1]:passLengths[2]]
	t.Logf("\nPass 2 data: % 02x", passData2)
	prevContexts3 := decoder2.mqc.GetContexts()
	decoder2.mqc = mqc.NewMQDecoderWithContexts(passData2, prevContexts3)
	if err := decoder2.decodeCleanupPass(); err != nil {
		t.Logf("Pass 2 decode failed: %v", err)
	} else {
		decoded2 := decoder2.GetData()
		t.Logf("After all passes: %v", decoded2[:16])

		// Check error
		maxError := int32(0)
		for i := 0; i < len(data); i++ {
			diff := data[i] - decoded2[i]
			if diff < 0 {
				diff = -diff
			}
			if diff > maxError {
				maxError = diff
			}
		}
		t.Logf("Final error: %d", maxError)

		if maxError > 0 {
			t.Errorf("Expected perfect reconstruction, got error=%d", maxError)
		}
	}
}
