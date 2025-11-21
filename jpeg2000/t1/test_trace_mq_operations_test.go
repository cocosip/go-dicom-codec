package t1

import (
	"testing"
)

// MQOperation records an MQ encode or decode operation
type MQOperation struct {
	OpType  string // "encode" or "decode"
	Context int
	Value   int
	SeqNum  int // Sequence number for matching
}

// TestTraceMQOperations - 追踪MQ encoder和decoder的所有操作
func TestTraceMQOperations(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient pattern (known to fail)
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Testing 5x5 gradient, maxBitplane=%d, numPasses=%d\n", maxBitplane, numPasses)

	// 先正常encode
	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded %d bytes\n", len(encoded))

	// 然后decode并检查前几个不匹配的值
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// 找出所有错误
	errors := []struct {
		idx      int
		x, y     int
		expected int32
		got      int32
	}{}

	for i := range input {
		if decoded[i] != input[i] {
			y := i / size
			x := i % size
			errors = append(errors, struct {
				idx      int
				x, y     int
				expected int32
				got      int32
			}{i, x, y, input[i], decoded[i]})
		}
	}

	t.Logf("\nFound %d errors:", len(errors))
	for i, e := range errors {
		if i >= 5 {
			t.Log("(showing only first 5...)")
			break
		}
		diff := e.got - e.expected
		t.Logf("  [%d] Pos(%d,%d): expected=%d, got=%d, diff=%d",
			e.idx, e.x, e.y, e.expected, e.got, diff)
	}

	// 分析第一个错误的bit差异
	if len(errors) > 0 {
		e := errors[0]
		t.Logf("\nAnalyzing first error at position (%d,%d):", e.x, e.y)

		expAbs := e.expected
		if expAbs < 0 {
			expAbs = -expAbs
		}
		gotAbs := e.got
		if gotAbs < 0 {
			gotAbs = -gotAbs
		}

		t.Logf("  Expected abs: %d (0b%08b)", expAbs, expAbs)
		t.Logf("  Got abs:      %d (0b%08b)", gotAbs, gotAbs)
		t.Logf("  XOR:          %d (0b%08b)", expAbs^gotAbs, expAbs^gotAbs)

		// 找出哪个bitplane有差异
		for bp := maxBitplane; bp >= 0; bp-- {
			expBit := (expAbs >> uint(bp)) & 1
			gotBit := (gotAbs >> uint(bp)) & 1
			if expBit != gotBit {
				t.Logf("  Bitplane %d: expected bit=%d, got bit=%d *** MISMATCH ***", bp, expBit, gotBit)
			} else {
				t.Logf("  Bitplane %d: bit=%d (ok)", bp, expBit)
			}
		}
	}
}
