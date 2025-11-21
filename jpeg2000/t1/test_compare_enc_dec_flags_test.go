package t1

import (
	"testing"
)

// TestCompareEncDecFlags - 对比encoder和decoder在每个bitplane后的flags状态
func TestCompareEncDecFlags(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient pattern
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	maxBitplane := CalculateMaxBitplane(input)
	t.Logf("MaxBitplane: %d\n", maxBitplane)

	// Encode with bitplane-by-bitplane access
	encoder := NewT1Encoder(size, size, 0)

	// 手动执行encoding,每个bitplane后检查flags
	paddedWidth := size + 2
	paddedHeight := size + 2
	paddedSize := paddedWidth * paddedHeight

	encoder.data = make([]int32, paddedSize)
	encoder.flags = make([]uint32, paddedSize)

	// Copy input data to padded array
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			srcIdx := y*size + x
			dstIdx := (y+1)*paddedWidth + (x + 1)
			encoder.data[dstIdx] = input[srcIdx]
		}
	}

	// 现在正常encode
	numPasses := (maxBitplane + 1) * 3
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	// 保存encoder的最终flags
	encFlagsSnapshot := make([]uint32, paddedSize)
	copy(encFlagsSnapshot, encoder.flags)

	// Decode
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	// 对比encoder和decoder的flags
	t.Log("\n=== Comparing Encoder vs Decoder Flags ===")

	mismatchCount := 0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			encFlags := encFlagsSnapshot[idx]
			decFlags := decoder.flags[idx]

			if encFlags != decFlags {
				mismatchCount++
				t.Logf("\nPosition (%d,%d) idx=%d value=%d:", x, y, y*size+x, input[y*size+x])
				debugPrintFlags("  ENC", encFlags)
				debugPrintFlags("  DEC", decFlags)

				// 显示差异
				diff := encFlags ^ decFlags
				t.Logf("  DIFF: 0x%x", diff)

				if mismatchCount >= 5 {
					t.Log("\n(Showing only first 5 mismatches...)")
					goto done
				}
			}
		}
	}

done:
	if mismatchCount > 0 {
		t.Errorf("Found %d flag mismatches between encoder and decoder", mismatchCount)
	} else {
		t.Log("All flags match between encoder and decoder!")
	}

	// 同时检查decoded值
	decoded := decoder.GetData()
	errorCount := 0
	for i := range input {
		if decoded[i] != input[i] {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Logf("\nValue errors: %d/%d (%.1f%%)", errorCount, numPixels,
			float64(errorCount)/float64(numPixels)*100)
	}
}
