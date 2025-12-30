package lossless

import (
	"fmt"
	"testing"
)

func TestDecodeWithPrint(t *testing.T) {
	// Test [10, 20]
	width, height := 2, 1
	bitDepth := 8
	pixelData := []byte{10, 20}

	fmt.Println("\n=== 编码 ===")
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	fmt.Printf("编码大小: %d bytes\n", len(encoded))

	fmt.Println("\n=== 解码 ===")
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	fmt.Printf("解码结果: %dx%d, %d components, %d bits\n", w, h, c, b)
	fmt.Printf("解码像素: %v\n", decoded)
	fmt.Printf("期望像素: %v\n", pixelData)

	if len(decoded) != len(pixelData) {
		t.Errorf("长度不匹配: %d vs %d", len(decoded), len(pixelData))
		return
	}

	for i := 0; i < len(pixelData); i++ {
		if byte(decoded[i]) != pixelData[i] {
			t.Errorf("像素 %d: 期望 %d, 得到 %d", i, pixelData[i], decoded[i])
		}
	}
}
