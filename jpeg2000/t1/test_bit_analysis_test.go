package t1

import (
	"fmt"
	"testing"
)

// TestBitAnalysis analyzes the bit-level errors in 5x5 decoding
func TestBitAnalysis(t *testing.T) {
	// Test a few specific values that show +1 error
	testCases := []struct {
		input    int32
		expected int32
		name     string
	}{
		{-127, -127, "minus_127"},
		{-125, -125, "minus_125"},
		{-123, -123, "minus_123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode and decode a single value in different positions
			width, height := 5, 5
			input := make([]int32, 25)

			// Fill with zeros except test position
			input[1] = tc.input // Position [0,1] - error prone position

			t.Logf("Testing value %d at position [0,1]", tc.input)
			t.Logf("Binary: %08b (abs=%d)", uint8(-tc.input), -tc.input)

			maxBitplane := 7
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

			if decoded[1] != tc.expected {
				t.Errorf("Position [0,1]: expected=%d, got=%d, diff=%d",
					tc.expected, decoded[1], decoded[1]-tc.expected)

				// Analyze bit difference
				expAbs := -tc.expected
				gotAbs := -decoded[1]
				t.Logf("Expected abs: %d = %08b", expAbs, uint8(expAbs))
				t.Logf("Got abs:      %d = %08b", gotAbs, uint8(gotAbs))
				t.Logf("XOR:              %08b", uint8(expAbs^gotAbs))
			}
		})
	}
}

// TestMinimal5x5 creates the minimal failing case for 5x5
func TestMinimal5x5(t *testing.T) {
	width, height := 5, 5

	// Create the exact pattern from the failing test
	input := make([]int32, 25)
	for i := 0; i < 25; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Logf("=== INPUT ===")
	printMatrixWithBinary(t, input, width, height)

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("\nEncoded: %d bytes", len(encoded))

	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	t.Logf("\n=== OUTPUT ===")
	printMatrixWithBinary(t, decoded, width, height)

	t.Logf("\n=== ERRORS ===")
	errorCount := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			if decoded[i] != input[i] {
				errorCount++
				diff := decoded[i] - input[i]
				inpAbs := input[i]
				if inpAbs < 0 {
					inpAbs = -inpAbs
				}
				decAbs := decoded[i]
				if decAbs < 0 {
					decAbs = -decAbs
				}

				t.Logf("[%d,%d] expected=%d (%08b) got=%d (%08b) diff=%+d xor=%08b",
					y, x, input[i], uint8(inpAbs), decoded[i], uint8(decAbs), diff, uint8(inpAbs^decAbs))
			}
		}
	}

	if errorCount > 0 {
		t.Errorf("Total errors: %d/%d (%.1f%%)", errorCount, 25, float64(errorCount)*100/25)
	}
}

func printMatrixWithBinary(t *testing.T, data []int32, width, height int) {
	for y := 0; y < height; y++ {
		line := ""
		for x := 0; x < width; x++ {
			val := data[y*width+x]
			absVal := val
			if absVal < 0 {
				absVal = -absVal
			}
			line += fmt.Sprintf("%4d(%08b) ", val, uint8(absVal))
		}
		t.Logf("  Row %d: %s", y, line)
	}
}
