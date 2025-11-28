package jpeg2000

import (
	"testing"
)

// TestCompareSingleVsMultiEncoding - 比较单层和多层编码的T1输出
func TestCompareSingleVsMultiEncoding(t *testing.T) {
	// 非常小的数据
	width, height := 4, 4
	pixelData := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		pixelData[i] = byte(i)
	}
	
	t.Logf("Test data: %v", pixelData)
	
	// 单层编码
	paramsSingle := DefaultEncodeParams(width, height, 1, 8, false)
	paramsSingle.NumLayers = 1
	paramsSingle.Lossless = false
	paramsSingle.NumLevels = 1
	
	encoderSingle := NewEncoder(paramsSingle)
	encodedSingle, err := encoderSingle.Encode(pixelData)
	if err != nil {
		t.Fatalf("Single-layer failed: %v", err)
	}
	
	// 多层编码
	paramsMulti := DefaultEncodeParams(width, height, 1, 8, false)
	paramsMulti.NumLayers = 2
	paramsMulti.Lossless = false
	paramsMulti.NumLevels = 1
	
	encoderMulti := NewEncoder(paramsMulti)
	encodedMulti, err := encoderMulti.Encode(pixelData)
	if err != nil {
		t.Fatalf("Multi-layer failed: %v", err)
	}
	
	t.Logf("Single-layer: %d bytes", len(encodedSingle))
	t.Logf("Multi-layer: %d bytes", len(encodedMulti))
	
	// 解码并比较
	decoderSingle := NewDecoder()
	if err := decoderSingle.Decode(encodedSingle); err != nil {
		t.Fatalf("Single decode failed: %v", err)
	}
	
	decoderMulti := NewDecoder()
	if err := decoderMulti.Decode(encodedMulti); err != nil {
		t.Fatalf("Multi decode failed: %v", err)
	}
	
	decodedSingle := decoderSingle.GetPixelData()
	decodedMulti := decoderMulti.GetPixelData()
	
	t.Logf("Original:  %v", pixelData)
	t.Logf("Single decoded: %v", decodedSingle)
	t.Logf("Multi decoded:  %v", decodedMulti)
	
	// 计算误差
	singleError := 0
	multiError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedSingle[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > singleError {
			singleError = diff
		}
		
		diff = int(pixelData[i]) - int(decodedMulti[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > multiError {
			multiError = diff
		}
	}
	
	t.Logf("Single-layer max error: %d", singleError)
	t.Logf("Multi-layer max error: %d", multiError)
	
	if singleError > 50 {
		t.Errorf("Single-layer broken")
	}
	if multiError > 50 {
		t.Errorf("Multi-layer broken")
	}
}
