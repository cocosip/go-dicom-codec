package t1

import (
	"fmt"
	"testing"
)

// TestMinimal5x5Trace - 最小的5x5 failing case,添加详细trace
func TestMinimal5x5Trace(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient pattern (已知会失败)
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	// 只关注第一个错误位置(1,0),打印相关信息
	target_x, target_y := 1, 0
	target_idx := target_y*size + target_x

	t.Logf("\nTarget position (%d,%d) expected value: %d", target_x, target_y, input[target_idx])

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	t.Logf("MaxBitplane: %d, NumPasses: %d\n", maxBitplane, numPasses)

	// Encode
	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("Encoded %d bytes\n", len(encoded))

	// Decode
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Check target position
	decoded_val := decoded[target_idx]
	if decoded_val != input[target_idx] {
		t.Errorf("Position (%d,%d): expected %d, got %d, diff=%d",
			target_x, target_y, input[target_idx], decoded_val, decoded_val-input[target_idx])

		// 打印周围的值
		t.Log("\nSurrounding values (decoded):")
		for dy := -1; dy <= 1; dy++ {
			line := ""
			for dx := -1; dx <= 1; dx++ {
				nx, ny := target_x+dx, target_y+dy
				if nx < 0 || nx >= size || ny < 0 || ny >= size {
					line += "   .  "
					continue
				}
				nidx := ny*size + nx
				if nidx == target_idx {
					line += fmt.Sprintf("[%4d]", decoded[nidx])
				} else {
					line += fmt.Sprintf(" %4d ", decoded[nidx])
				}
			}
			t.Log(line)
		}
	} else {
		t.Logf("Position (%d,%d): CORRECT value %d", target_x, target_y, decoded_val)
	}
}
