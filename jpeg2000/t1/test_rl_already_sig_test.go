package t1

import (
	"testing"
)

// TestRLAlreadySigBug tests the asymmetry between encoder and decoder
// when handling already-significant coefficients in RL path of Cleanup Pass
func TestRLAlreadySigBug(t *testing.T) {
	// Create a simple case where a coefficient becomes significant in SPP/MRP
	// and then gets processed again in CP RL path

	// This should create a scenario where:
	// 1. At bitplane 7, coefficient becomes significant via RL
	// 2. At bitplane 6, it's already significant but processed in RL again

	t.Run("simple_case", func(t *testing.T) {
		width, height := 4, 4
		input := []int32{
			-128, 0, 0, 0, // First becomes sig at bp7
			0, 0, 0, 0,
			0, 0, 0, 0,
			0, 0, 0, 0,
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("Encoded: %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		// Check if values match
		for i := range input {
			if decoded[i] != input[i] {
				t.Errorf("[%d] input=%d decoded=%d", i, input[i], decoded[i])
			}
		}
	})

	t.Run("encoder_decoder_sync", func(t *testing.T) {
		// Test case: coefficient becomes significant at high bitplane,
		// then needs refinement at lower bitplanes via CP
		width, height := 5, 1
		input := []int32{
			-128, -127, -126, -125, -124,
		}

		maxBitplane := 7
		numPasses := (maxBitplane + 1) * 3

		t.Logf("Input: %v", input)

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("Encoded: %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()
		t.Logf("Decoded: %v", decoded)

		errors := 0
		for i := range input {
			if decoded[i] != input[i] {
				t.Errorf("[%d] input=%d decoded=%d (diff=%d)",
					i, input[i], decoded[i], decoded[i]-input[i])
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("Total errors: %d/%d", errors, len(input))
		}
	})
}
