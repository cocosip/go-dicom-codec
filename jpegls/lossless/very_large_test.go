package lossless

import (
	"testing"
)

func TestVeryLarge8BitImage(t *testing.T) {
	// 测试接近真实DICOM大小的图像：850x1100
	width, height := 850, 1100
	bitDepth := 8

	t.Logf("创建测试图像: %dx%d, %d位 (%d像素)", width, height, bitDepth, width*height)

	pixelData := make([]byte, width*height)

	// 创建一个类似医学图像的图案
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			// 渐变 + 纹理
			base := ((x + y) * 127 / (width + height)) % 256
			texture := ((x*3)^(y*5) + (x*y)%100) % 20
			pixelData[idx] = byte((base + texture) % 256)
		}
	}

	t.Logf("图像创建完成")

	// 编码
	t.Logf("开始编码...")
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	t.Logf("编码成功: %d bytes", len(encoded))
	t.Logf("原始大小: %d bytes", len(pixelData))
	t.Logf("压缩率: %.2f%%", float64(len(encoded))*100/float64(len(pixelData)))

	// 解码
	t.Logf("开始解码...")
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	t.Logf("解码成功: %dx%d, %d组件, %d位", w, h, c, b)

	// 验证
	if len(decoded) != len(pixelData) {
		t.Fatalf("数据长度不匹配: %d vs %d", len(decoded), len(pixelData))
	}

	diffs := 0
	firstDiff := -1
	for i := 0; i < len(pixelData); i++ {
		if byte(decoded[i]) != pixelData[i] {
			diffs++
			if firstDiff == -1 {
				firstDiff = i
			}
		}
	}

	if diffs > 0 {
		t.Errorf("发现%d个差异, 第一个在%d", diffs, firstDiff)
	} else {
		t.Log("✅ 完美重建大图像!")
	}
}
