package t1

import (
	"testing"
)

// TestMQDetailed5x4vs5x5 performs deep MQ-level analysis
// Comparing 5x4 (pass) vs 5x5 (fail) to find the divergence point
func TestMQDetailed5x4vs5x5(t *testing.T) {
	// Use identical first 20 values
	baseValues := make([]int32, 20)
	for i := 0; i < 20; i++ {
		baseValues[i] = int32(i%256) - 128
	}

	t.Run("5x4_baseline", func(t *testing.T) {
		width, height := 5, 4
		input := make([]int32, 20)
		copy(input, baseValues)

		t.Logf("=== 5x4 (BASELINE - PASS) ===")
		t.Logf("First 10 values: %v", input[:10])

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

		// Encode with detailed logging
		encoder := NewT1Encoder(width, height, 0)

		// We'll need to examine internal state, so let's encode and check
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("Encoded size: %d bytes", len(encoded))

		// Decode
		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		// Verify correctness
		errors := 0
		for i := range input {
			if decoded[i] != input[i] {
				errors++
			}
		}

		if errors > 0 {
			t.Errorf("5x4: UNEXPECTED FAILURE - %d errors", errors)
		} else {
			t.Logf("5x4: ✓ Perfect (as expected)")
		}

		// Log encoding statistics
		t.Logf("5x4 Encoded bytes: %v", encoded[:minInt(20, len(encoded))])
	})

	t.Run("5x5_failing", func(t *testing.T) {
		width, height := 5, 5
		input := make([]int32, 25)
		copy(input, baseValues)
		for i := 20; i < 25; i++ {
			input[i] = int32(i%256) - 128
		}

		t.Logf("\n=== 5x5 (FAILING) ===")
		t.Logf("First 10 values: %v (same as 5x4)", input[:10])
		t.Logf("Last 5 values: %v", input[20:])

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3
		t.Logf("maxBitplane=%d, numPasses=%d", maxBitplane, numPasses)

		// Encode
		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}
		t.Logf("Encoded size: %d bytes", len(encoded))

		// Decode
		decoder := NewT1Decoder(width, height, 0)
		err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decoding failed: %v", err)
		}

		decoded := decoder.GetData()

		// Check first 20 values (same as 5x4)
		errorsInFirst20 := 0
		for i := 0; i < 20; i++ {
			if decoded[i] != input[i] {
				errorsInFirst20++
			}
		}

		t.Logf("\n5x5 Results:")
		t.Logf("  Errors in first 20 values (rows 0-3, same as 5x4): %d", errorsInFirst20)
		t.Logf("  5x5 Encoded bytes: %v", encoded[:minInt(20, len(encoded))])

		if errorsInFirst20 > 0 {
			t.Logf("\n⚠️  CRITICAL: First 20 values have errors even though they're identical to 5x4!")
			t.Logf("  This confirms that row 4 affects rows 0-3")
		}
	})

	t.Run("comparison_analysis", func(t *testing.T) {
		t.Logf("\n=== COMPARISON ANALYSIS ===")
		t.Logf("Key observations:")
		t.Logf("1. Both use same maxBitplane (7)")
		t.Logf("2. Both use same numPasses (24)")
		t.Logf("3. First 20 input values are IDENTICAL")
		t.Logf("4. 5x4: first 20 values → perfect")
		t.Logf("5. 5x5: first 20 values → have errors")
		t.Logf("\nConclusion: The presence of row 4 (5th row) changes how rows 0-3 are encoded/decoded")
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
