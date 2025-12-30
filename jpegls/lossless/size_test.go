package lossless

import (
	"testing"
)

func TestDifferentSizes(t *testing.T) {
	testSizes := []struct {
		width  int
		height int
	}{
		{50, 50},
		{100, 100},
		{200, 100},
		{100, 200},
		{300, 300},
		{500, 500},
	}

	for _, size := range testSizes {
		width, height := size.width, size.height
		bitDepth := 8

		t.Logf("\n测试 %dx%d...", width, height)

		pixelData := make([]byte, width*height)
		for i := 0; i < len(pixelData); i++ {
			pixelData[i] = byte(i % 256)
		}

		encoded, err := Encode(pixelData, width, height, 1, bitDepth)
		if err != nil {
			t.Errorf("%dx%d: 编码失败: %v", width, height, err)
			continue
		}

		decoded, w, h, _, _, err := Decode(encoded)
		if err != nil {
			t.Errorf("%dx%d: 解码失败: %v", width, height, err)
			continue
		}

		if w != width || h != height {
			t.Errorf("%dx%d: 尺寸不匹配: 得到%dx%d", width, height, w, h)
			continue
		}

		diffs := 0
		firstDiff := -1
		for i := 0; i < len(pixelData); i++ {
			if byte(decoded[i]) != pixelData[i] {
				diffs++
				if firstDiff == -1 {
					firstDiff = i
					x := firstDiff % width
					y := firstDiff / width
					t.Logf("  第一个差异在索引%d (x=%d, y=%d): 期望%d, 得到%d",
						firstDiff, x, y, pixelData[firstDiff], decoded[firstDiff])
				}
			}
		}

		if diffs > 0 {
			t.Errorf("%dx%d: ❌ 发现%d个差异 (%.2f%%)", width, height, diffs, float64(diffs)*100/float64(len(pixelData)))
		} else {
			t.Logf("%dx%d: ✅ 完美", width, height)
		}
	}
}
