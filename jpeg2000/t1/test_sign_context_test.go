package t1

import (
	"testing"
)

// TestSignContext tests sign coding context with simple patterns
func TestSignContext(t *testing.T) {
	t.Run("all_positive", func(t *testing.T) {
		width, height := 3, 3
		input := []int32{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

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
				t.Errorf("[%d] expected=%d got=%d", i, input[i], decoded[i])
			}
		}

		if errors == 0 {
			t.Logf("✓ All positive: perfect")
		}
	})

	t.Run("all_negative", func(t *testing.T) {
		width, height := 3, 3
		input := []int32{
			-1, -2, -3,
			-4, -5, -6,
			-7, -8, -9,
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

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
				t.Errorf("[%d] expected=%d got=%d", i, input[i], decoded[i])
			}
		}

		if errors == 0 {
			t.Logf("✓ All negative: perfect")
		}
	})

	t.Run("mixed_signs", func(t *testing.T) {
		width, height := 3, 3
		input := []int32{
			-10, 10, -10,
			10, -10, 10,
			-10, 10, -10,
		}

		maxBitplane := CalculateMaxBitplane(input)
		numPasses := (maxBitplane + 1) * 3

		encoder := NewT1Encoder(width, height, 0)
		encoded, err := encoder.Encode(input, numPasses, 0)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

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
				t.Errorf("[%d] expected=%d got=%d", i, input[i], decoded[i])
			}
		}

		if errors == 0 {
			t.Logf("✓ Mixed signs: perfect")
		}
	})
}
