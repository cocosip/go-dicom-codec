package t1

import (
	"fmt"
	"testing"
)

// TestAnalyzePosition1_0 - 详细分析5x5 gradient中位置(1,0)的处理
func TestAnalyzePosition1_0(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Gradient pattern
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	t.Log("\n5x5 input values:")
	for y := 0; y < size; y++ {
		line := ""
		for x := 0; x < size; x++ {
			idx := y*size + x
			line += fmt.Sprintf("%5d ", input[idx])
		}
		t.Logf("  %s", line)
	}

	// 分析位置(1,0)及其邻居在各bitplane的significant状态
	target_x, target_y := 1, 0
	target_idx := target_y*size + target_x
	target_val := input[target_idx] // -127

	t.Logf("\nTarget position (%d,%d) idx=%d value=%d", target_x, target_y, target_idx, target_val)

	// 计算这个值在哪些bitplane是significant
	absVal := target_val
	if absVal < 0 {
		absVal = -absVal
	}
	t.Logf("Absolute value: %d (binary: %08b)", absVal, absVal)

	for bp := 7; bp >= 0; bp-- {
		bit := (absVal >> uint(bp)) & 1
		if bit != 0 {
			t.Logf("  Bitplane %d: bit=1 (WILL become significant)", bp)
		} else {
			t.Logf("  Bitplane %d: bit=0", bp)
		}
	}

	// 分析邻居
	neighbors := []struct{
		name string
		dx, dy int
	}{
		{"North", 0, -1},
		{"South", 0, 1},
		{"West", -1, 0},
		{"East", 1, 0},
		{"NW", -1, -1},
		{"NE", 1, -1},
		{"SW", -1, 1},
		{"SE", 1, 1},
	}

	t.Log("\nNeighbors:")
	for _, n := range neighbors {
		nx := target_x + n.dx
		ny := target_y + n.dy
		if nx < 0 || nx >= size || ny < 0 || ny >= size {
			t.Logf("  %s: OUT OF BOUNDS", n.name)
			continue
		}
		nidx := ny*size + nx
		nval := input[nidx]
		absNval := nval
		if absNval < 0 {
			absNval = -absNval
		}

		// 找到这个邻居第一次变significant的bitplane
		firstSigBP := -1
		for bp := 7; bp >= 0; bp-- {
			if ((absNval >> uint(bp)) & 1) != 0 {
				firstSigBP = bp
				break
			}
		}

		t.Logf("  %s (%d,%d): val=%d abs=%d first_sig_bp=%d",
			n.name, nx, ny, nval, absNval, firstSigBP)
	}

	// 现在实际编码解码看看
	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	decoded_val := decoded[target_idx]
	t.Logf("\nResult at (%d,%d): expected=%d, decoded=%d, diff=%d",
		target_x, target_y, target_val, decoded_val, decoded_val-target_val)

	if decoded_val != target_val {
		t.Errorf("ERROR at position (%d,%d)", target_x, target_y)
	}
}
