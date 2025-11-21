package t1

import (
	"testing"
)

// TestWidth5Height1 tests if width=5 works correctly without RL encoding
// Height=1 means no RL encoding (need 4 rows for RL)
func TestWidth5Height1(t *testing.T) {
	width, height := 5, 1
	input := []int32{-128, -127, -126, -125, -124}

	t.Logf("Testing width=5, height=1 (no RL encoding)")
	t.Logf("Input: %v", input)

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("Encoded: %d bytes, maxBitplane=%d", len(encoded), maxBitplane)

	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()
	t.Logf("Decoded: %v", decoded)

	errors := 0
	for i := 0; i < len(input); i++ {
		if decoded[i] != input[i] {
			errors++
			t.Errorf("[%d] expected=%d got=%d diff=%d",
				i, input[i], decoded[i], decoded[i]-input[i])
		}
	}

	if errors == 0 {
		t.Logf("✓ Perfect: width=5 height=1 works correctly")
	}
}

// TestWidth5Height3 tests width=5 with 3 rows (also no RL - need 4 rows)
func TestWidth5Height3(t *testing.T) {
	width, height := 5, 3
	input := make([]int32, 15)
	for i := 0; i < 15; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Logf("Testing width=5, height=3 (no RL encoding - need 4 rows)")

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
	for i := 0; i < len(input); i++ {
		if decoded[i] != input[i] {
			errors++
			t.Errorf("[%d] expected=%d got=%d diff=%d",
				i, input[i], decoded[i], decoded[i]-input[i])
		}
	}

	if errors == 0 {
		t.Logf("✓ Perfect: width=5 height=3 works correctly")
	} else {
		t.Errorf("width=5 height=3: %d errors", errors)
	}
}

// TestWidth5Height4 tests width=5 with exactly 4 rows (just enough for RL)
func TestWidth5Height4(t *testing.T) {
	width, height := 5, 4
	input := make([]int32, 20)
	for i := 0; i < 20; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Logf("Testing width=5, height=4 (exactly 1 RL group)")

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
	for i := 0; i < len(input); i++ {
		if decoded[i] != input[i] {
			errors++
			t.Logf("[%d] expected=%d got=%d diff=%d",
				i, input[i], decoded[i], decoded[i]-input[i])
		}
	}

	if errors == 0 {
		t.Logf("✓ Perfect: width=5 height=4 works correctly")
	} else {
		t.Errorf("width=5 height=4: %d errors", errors)
	}
}
