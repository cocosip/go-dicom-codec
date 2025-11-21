package t1

import (
	"fmt"
	"testing"
)

// TestTraceContextUsage - 追踪4x4 vs 5x5在使用sign context时的差异
func TestTraceContextUsage(t *testing.T) {
	// 临时修改getSignCodingContext来记录调用
	testCases := []struct {
		size int
		name string
	}{
		{4, "4x4"},
		{5, "5x5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size
			numPixels := size * size
			input := make([]int32, numPixels)

			// Gradient pattern
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			// 打印输入矩阵
			t.Logf("\n%s Input pattern:", tc.name)
			for y := 0; y < size; y++ {
				line := ""
				for x := 0; x < size; x++ {
					idx := y*size + x
					line += fmt.Sprintf("%5d ", input[idx])
				}
				t.Logf("  %s", line)
			}

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

			// 检查错误
			errorCount := 0
			firstError := -1
			for i := range input {
				if decoded[i] != input[i] {
					if firstError == -1 {
						firstError = i
					}
					errorCount++
				}
			}

			if errorCount > 0 {
				t.Logf("\n%s: %d errors, first at index %d (pos %d,%d)",
					tc.name, errorCount, firstError, firstError%size, firstError/size)

				// 打印第一个错误附近的值
				if firstError >= 0 {
					y := firstError / size
					x := firstError % size
					t.Logf("\nAround first error (%d,%d):", x, y)

					for dy := -1; dy <= 1; dy++ {
						ny := y + dy
						if ny < 0 || ny >= size {
							continue
						}
						line := "  "
						for dx := -1; dx <= 1; dx++ {
							nx := x + dx
							if nx < 0 || nx >= size {
								line += "      "
								continue
							}
							idx := ny*size + nx
							if idx == firstError {
								line += fmt.Sprintf("[%4d] ", decoded[idx])
							} else {
								line += fmt.Sprintf(" %4d  ", decoded[idx])
							}
						}
						t.Logf("%s", line)
					}
				}
			} else {
				t.Logf("\n%s: PASS (0 errors)", tc.name)
			}
		})
	}
}
