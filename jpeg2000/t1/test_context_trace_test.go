package t1

import (
	"testing"
)

// TestContextTrace - trace context values during encoding
func TestContextTrace(t *testing.T) {
	// Temporarily modify encoder to output context for each encode

	baseValues := make([]int32, 20)
	for i := 0; i < 20; i++ {
		baseValues[i] = int32(i%256) - 128
	}

	t.Run("analyze_5x4", func(t *testing.T) {
		width, height := 5, 4
		input := make([]int32, 20)
		copy(input, baseValues)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		t.Logf("=== 5x4 Analysis ===")
		t.Logf("maxBitplane=%d", maxBitplane)

		// Check which coefficients become significant at BP7
		sigAtBP7 := 0
		for i, val := range input {
			absVal := val
			if absVal < 0 {
				absVal = -absVal
			}
			if ((absVal >> uint(maxBitplane)) & 1) != 0 {
				sigAtBP7++
				if i < 5 {
					t.Logf("  Coeff[%d] = %d, absVal=%d, sig at BP%d", i, val, absVal, maxBitplane)
				}
			}
		}
		t.Logf("Total coefficients significant at BP%d: %d", maxBitplane, sigAtBP7)

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		t.Logf("Encoded %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()
		errors := 0
		for i := range input {
			if decoded[i] != input[i] {
				errors++
			}
		}
		t.Logf("Decoding errors: %d", errors)
	})

	t.Run("analyze_5x5", func(t *testing.T) {
		width, height := 5, 5
		input := make([]int32, 25)
		copy(input, baseValues)
		for i := 20; i < 25; i++ {
			input[i] = int32(i%256) - 128
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		t.Logf("\n=== 5x5 Analysis ===")
		t.Logf("maxBitplane=%d", maxBitplane)

		sigAtBP7 := 0
		for i, val := range input {
			absVal := val
			if absVal < 0 {
				absVal = -absVal
			}
			if ((absVal >> uint(maxBitplane)) & 1) != 0 {
				sigAtBP7++
				if i < 5 || i >= 20 {
					t.Logf("  Coeff[%d] = %d, absVal=%d, sig at BP%d", i, val, absVal, maxBitplane)
				}
			}
		}
		t.Logf("Total coefficients significant at BP%d: %d (including row 4)", maxBitplane, sigAtBP7)

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		t.Logf("Encoded %d bytes", len(encoded))

		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()
		errorsFirst20 := 0
		for i := 0; i < 20; i++ {
			if decoded[i] != input[i] {
				errorsFirst20++
			}
		}
		t.Logf("Decoding errors in first 20: %d", errorsFirst20)

		errorsAll := 0
		for i := range input {
			if decoded[i] != input[i] {
				errorsAll++
			}
		}
		t.Logf("Decoding errors total: %d", errorsAll)
	})

	t.Run("hypothesis", func(t *testing.T) {
		t.Logf("\n=== Hypothesis ===")
		t.Logf("1. In 5x4: rows 0-3 are encoded in one RL group")
		t.Logf("2. In 5x5: rows 0-3 are encoded in one RL group, row 4 in another")
		t.Logf("3. When row 3 coefficients become significant, updateNeighborFlags is called")
		t.Logf("4. In 5x4: boundary check prevents updating padding")
		t.Logf("5. In 5x5: boundary check allows updating row 4's T1_SIG_N flags")
		t.Logf("6. This causes row 4 in 5x5 to have different neighbor flags")
		t.Logf("7. But why does this affect rows 0-3 encoding?")
		t.Logf("")
		t.Logf("WAIT! Maybe the issue is the OPPOSITE!")
		t.Logf("Maybe in 5x5, row 4 becoming significant sets flags that affect row 3!")
		t.Logf("And row 3's different encoding cascades upward!")
	})
}
