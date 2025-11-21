package t1

import (
	"testing"
)

// TestDeepTrace5x4vs5x5 - Deep trace of encoder state differences
func TestDeepTrace5x4vs5x5(t *testing.T) {
	// Use identical first 20 values for both tests
	baseValues := make([]int32, 20)
	for i := 0; i < 20; i++ {
		baseValues[i] = int32(i%256) - 128
	}

	t.Run("5x4_trace", func(t *testing.T) {
		width, height := 5, 4
		input := make([]int32, 20)
		copy(input, baseValues)

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)

		// Manually trace through first few coefficients at BP7
		t.Logf("\n=== 5x4 Encoding Trace ===")
		t.Logf("Input values: %v", input[:10])
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

		// Check padding and initial state
		paddedWidth := width + 2
		paddedHeight := height + 2
		t.Logf("paddedWidth=%d, paddedHeight=%d, flags size=%d",
			paddedWidth, paddedHeight, len(encoder.flags))

		// Encode
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		t.Logf("Encoded size: %d bytes", len(encoded))
		t.Logf("First 20 encoded bytes: %v", encoded[:minInt(20, len(encoded))])

		// Check some flags after encoding
		// Check coefficient at (0,3) - this is the last row in 5x4
		idx_0_3 := (3+1)*paddedWidth + (0 + 1)
		flags_0_3 := encoder.flags[idx_0_3]
		t.Logf("\nAfter encoding:")
		t.Logf("  Coeff(0,3) idx=%d flags=0x%x SIG=%v",
			idx_0_3, flags_0_3, (flags_0_3&T1_SIG) != 0)

		// Check South neighbor of (0,3) - this is in the padding for 5x4
		southIdx := (3+2)*paddedWidth + (0 + 1)
		southFlags := encoder.flags[southIdx]
		t.Logf("  South of (0,3) idx=%d flags=0x%x (in padding)",
			southIdx, southFlags)

		// Decode
		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		// Verify
		errors := 0
		for i := range input {
			if decoded[i] != input[i] {
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("5x4: UNEXPECTED - %d errors", errors)
		} else {
			t.Logf("\n5x4: ✓ Perfect (as expected)")
		}
	})

	t.Run("5x5_trace", func(t *testing.T) {
		width, height := 5, 5
		input := make([]int32, 25)
		copy(input, baseValues)
		// Fill row 4 with continuation of pattern
		for i := 20; i < 25; i++ {
			input[i] = int32(i%256) - 128
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)

		t.Logf("\n=== 5x5 Encoding Trace ===")
		t.Logf("Input values (first 10): %v", input[:10])
		t.Logf("Input values (row 4): %v", input[20:])
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

		paddedWidth := width + 2
		paddedHeight := height + 2
		t.Logf("paddedWidth=%d, paddedHeight=%d, flags size=%d",
			paddedWidth, paddedHeight, len(encoder.flags))

		// Encode
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		t.Logf("Encoded size: %d bytes", len(encoded))
		t.Logf("First 20 encoded bytes: %v", encoded[:minInt(20, len(encoded))])

		// Check flags after encoding
		idx_0_3 := (3+1)*paddedWidth + (0 + 1)
		flags_0_3 := encoder.flags[idx_0_3]
		t.Logf("\nAfter encoding:")
		t.Logf("  Coeff(0,3) idx=%d flags=0x%x SIG=%v",
			idx_0_3, flags_0_3, (flags_0_3&T1_SIG) != 0)

		// Check South neighbor of (0,3) - this is coeff (0,4) in 5x5
		southIdx := (3+2)*paddedWidth + (0 + 1)
		southFlags := encoder.flags[southIdx]
		t.Logf("  South of (0,3) = Coeff(0,4) idx=%d flags=0x%x SIG_N=%v (REAL coeff)",
			southIdx, southFlags, (southFlags&T1_SIG_N) != 0)

		// Check the actual (0,4) flags
		idx_0_4 := (4+1)*paddedWidth + (0 + 1)
		flags_0_4 := encoder.flags[idx_0_4]
		t.Logf("  Coeff(0,4) idx=%d flags=0x%x SIG=%v",
			idx_0_4, flags_0_4, (flags_0_4&T1_SIG) != 0)

		// Decode
		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		// Check first 20 values (should be same as 5x4 input)
		errorsFirst20 := 0
		for i := 0; i < 20; i++ {
			if decoded[i] != input[i] {
				errorsFirst20++
				if errorsFirst20 <= 5 {
					t.Logf("  Error[%d]: expected=%d got=%d", i, input[i], decoded[i])
				}
			}
		}

		t.Logf("\n5x5: %d errors in first 20 values", errorsFirst20)

		// Check all 25
		errorsAll := 0
		for i := range input {
			if decoded[i] != input[i] {
				errorsAll++
			}
		}
		t.Logf("5x5: %d errors total (out of 25)", errorsAll)
	})

	t.Run("comparison", func(t *testing.T) {
		t.Logf("\n=== Key Observations ===")
		t.Logf("1. Both tests use identical first 20 input values")
		t.Logf("2. In 5x4: row 3 is last row, South neighbor is padding")
		t.Logf("3. In 5x5: row 3 is NOT last row, South neighbor is real coeff (row 4)")
		t.Logf("4. This causes different neighbor flags for row 3")
		t.Logf("5. Different flags → different contexts → different MQ encoding")
		t.Logf("6. Question: Why does this cause ERRORS instead of just different encoding?")
	})
}
