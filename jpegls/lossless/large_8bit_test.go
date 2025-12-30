package lossless

import (
	"testing"
)

func TestLarge8BitImage(t *testing.T) {
	// 创建一个类似真实医学图像的测试：100x100, 8位，有变化的图像
	width, height := 100, 100
	bitDepth := 8

	pixelData := make([]byte, width*height)

	// 创建一个渐变图案with some variation
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			// 基础渐变
			base := (x + y) % 256
			// 添加一些随机变化
			variation := ((x * 7) ^ (y * 13)) % 20
			pixelData[idx] = byte((base + variation) % 256)
		}
	}

	t.Logf("测试图像: %dx%d, %d位", width, height, bitDepth)
	t.Logf("前20个像素: %v", pixelData[:20])

	// 编码
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	t.Logf("编码大小: %d bytes", len(encoded))
	t.Logf("压缩率: %.2f%%", float64(len(encoded))*100/float64(len(pixelData)))

	// 查找扫描数据
	scanStart := -1
	for i := 0; i < len(encoded)-1; i++ {
		if encoded[i] == 0xFF && encoded[i+1] == 0xDA {
			length := int(encoded[i+2])<<8 | int(encoded[i+3])
			scanStart = i + 2 + length
			break
		}
	}

	if scanStart > 0 {
		scanEnd := len(encoded) - 2
		scanData := encoded[scanStart:scanEnd]
		t.Logf("扫描数据大小: %d bytes", len(scanData))
		t.Logf("前100字节: %X", scanData[:min(100, len(scanData))])

		// 检查是否有重复pattern
		if len(scanData) > 10 {
			pattern := scanData[0:3]
			repeatCount := 0
			for i := 0; i < len(scanData)-3; i += 3 {
				if i+3 <= len(scanData) &&
					scanData[i] == pattern[0] &&
					scanData[i+1] == pattern[1] &&
					scanData[i+2] == pattern[2] {
					repeatCount++
				}
			}
			if repeatCount > len(scanData)/10 {
				t.Logf("⚠️  警告: 检测到重复pattern, 重复%d次", repeatCount)
			}
		}
	}

	// 解码
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	t.Logf("解码: %dx%d, %d组件, %d位", w, h, c, b)

	// 验证
	if w != width || h != height {
		t.Errorf("尺寸不匹配: 期望%dx%d, 得到%dx%d", width, height, w, h)
	}

	if len(decoded) != len(pixelData) {
		t.Fatalf("数据长度不匹配: %d vs %d", len(decoded), len(pixelData))
	}

	diffs := 0
	firstDiffIdx := -1
	for i := 0; i < len(pixelData); i++ {
		if byte(decoded[i]) != pixelData[i] {
			diffs++
			if firstDiffIdx == -1 {
				firstDiffIdx = i
			}
		}
	}

	if diffs > 0 {
		t.Errorf("发现%d个差异, 第一个在索引%d: 期望%d, 得到%d",
			diffs, firstDiffIdx, pixelData[firstDiffIdx], decoded[firstDiffIdx])
	} else {
		t.Log("✅ 完美重建!")
	}
}

func TestFlatRegion8Bit(t *testing.T) {
	// 测试一个有大量平坦区域的图像（测试RUN模式）
	width, height := 100, 100
	bitDepth := 8

	pixelData := make([]byte, width*height)

	// 前50行都是同一个值（测试长RUN）
	for i := 0; i < 50*width; i++ {
		pixelData[i] = 128
	}

	// 后50行是渐变
	for i := 50 * width; i < width*height; i++ {
		pixelData[i] = byte(i % 256)
	}

	t.Logf("测试图像: %dx%d (前50行平坦值128)", width, height)

	// 编码
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	t.Logf("编码大小: %d bytes (原始: %d bytes)", len(encoded), len(pixelData))

	// 解码
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	t.Logf("解码成功: %dx%d, %d组件, %d位", w, h, c, b)

	// 验证
	diffs := 0
	for i := 0; i < len(pixelData); i++ {
		if byte(decoded[i]) != pixelData[i] {
			diffs++
			if diffs <= 5 {
				t.Logf("差异 %d: 索引%d, 期望%d, 得到%d", diffs, i, pixelData[i], decoded[i])
			}
		}
	}

	if diffs > 0 {
		t.Errorf("发现%d个差异", diffs)
	} else {
		t.Log("✅ 完美重建!")
	}
}
