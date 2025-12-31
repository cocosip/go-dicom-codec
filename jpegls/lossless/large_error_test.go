package lossless

import (
	"fmt"
	"testing"
)

// TestLargeError 测试 lossless 如何处理大误差
func TestLargeError(t *testing.T) {
	// 测试与 nearlossless 相同的模式
	width, height := 3, 1
	pixelData := make([]byte, width*height)

	pixelData[0] = 245
	pixelData[1] = 252
	pixelData[2] = 3

	fmt.Printf("测试 lossless 处理大误差：\n")
	fmt.Printf("原始: ")
	for i := 0; i < len(pixelData); i++ {
		fmt.Printf("%3d ", pixelData[i])
	}
	fmt.Printf("\n")

	// 编码
	encoded, err := Encode(pixelData, width, height, 1, 8)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	fmt.Printf("编码大小: %d 字节\n", len(encoded))

	// 解码
	decoded, _, _, _, _, err := Decode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	fmt.Printf("解码: ")
	for i := 0; i < len(decoded); i++ {
		fmt.Printf("%3d ", decoded[i])
	}
	fmt.Printf("\n")

	// 检查
	for i := 0; i < len(pixelData); i++ {
		if decoded[i] != pixelData[i] {
			t.Errorf("像素%d: 原始=%d, 解码=%d", i, pixelData[i], decoded[i])
		}
	}
}
