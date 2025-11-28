package jpeg2000

import (
	"testing"
)

// TestDebugLossyMultiLayerDetail - 详细调试lossy多层编码
func TestDebugLossyMultiLayerDetail(t *testing.T) {
	// 使用非常小的图像便于调试
	width, height := 8, 8
	pixelData := make([]byte, width*height)
	
	// 简单的测试数据
	for i := 0; i < width*height; i++ {
		pixelData[i] = byte(i % 16) // 0-15循环
	}
	
	t.Logf("Original pixel data (8x8): %v", pixelData)
	
	// 2层lossy编码
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 2
	params.Lossless = false
	params.NumLevels = 2 // 减少层级便于调试
	
	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}
	t.Logf("Encoded size: %d bytes", len(encoded))
	
	// 解码
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}
	
	decoded := decoder.GetPixelData()
	t.Logf("Decoded pixel data (8x8): %v", decoded)
	
	// 详细比较
	maxError := 0
	errorPositions := []int{}
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		if diff > 10 {
			errorPositions = append(errorPositions, i)
		}
	}
	
	t.Logf("Max error: %d", maxError)
	t.Logf("Large error positions (>10): %v", errorPositions)
	
	if maxError > 50 {
		t.Errorf("Lossy multi-layer broken: max error=%d", maxError)
	}
}
