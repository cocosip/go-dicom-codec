package t1

import (
	"testing"
)

// TestTracePosition1_0MRP - 追踪位置(1,0)在MRP中的处理
func TestTracePosition1_0MRP(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	target_x, target_y := 1, 0
	target_idx := target_y*size + target_x

	t.Logf("Target position (%d,%d) expected value: %d (abs=%d, binary=%08b)",
		target_x, target_y, input[target_idx], 127, 127)

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	// 创建一个小的5x5测试,只trace bitplane 0
	t.Logf("\n=== Testing position (1,0) in bitplane 0 MRP ===")
	t.Logf("Expected: value=-127 has all bits set in abs value")
	t.Logf("So bitplane 0 should have bit=1")

	// Encode
	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	// Decode
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Check result
	if decoded[target_idx] != input[target_idx] {
		t.Errorf("Position (%d,%d): expected=%d, got=%d, diff=%d",
			target_x, target_y, input[target_idx], decoded[target_idx],
			decoded[target_idx]-input[target_idx])

		// 分析bit差异
		expAbs := int32(127)
		gotAbs := decoded[target_idx]
		if gotAbs < 0 {
			gotAbs = -gotAbs
		}

		t.Logf("\nBit analysis:")
		t.Logf("  Expected abs: %d (0b%08b)", expAbs, expAbs)
		t.Logf("  Got abs:      %d (0b%08b)", gotAbs, gotAbs)

		for bp := 7; bp >= 0; bp-- {
			expBit := (expAbs >> uint(bp)) & 1
			gotBit := (gotAbs >> uint(bp)) & 1
			if expBit != gotBit {
				t.Logf("  Bitplane %d: expected=%d, got=%d *** MISMATCH ***", bp, expBit, gotBit)
			}
		}
	} else {
		t.Logf("Position (%d,%d): PASS - correctly decoded as %d", target_x, target_y, decoded[target_idx])
	}
}
