package jpeg2000

import (
	"testing"
)

func TestCompareMultiLayerVsSingleLayer(t *testing.T) {
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	// Encode with single layer
	paramsSingle := DefaultEncodeParams(width, height, 1, 8, false)
	paramsSingle.NumLayers = 1
	paramsSingle.Lossless = false
	paramsSingle.NumLevels = 5

	encoderSingle := NewEncoder(paramsSingle)
	encodedSingle, err := encoderSingle.Encode(pixelData)
	if err != nil {
		t.Fatalf("Single-layer encoding failed: %v", err)
	}

	// Encode with 2 layers
	paramsMulti := DefaultEncodeParams(width, height, 1, 8, false)
	paramsMulti.NumLayers = 2
	paramsMulti.Lossless = false
	paramsMulti.NumLevels = 5

	encoderMulti := NewEncoder(paramsMulti)
	encodedMulti, err := encoderMulti.Encode(pixelData)
	if err != nil {
		t.Fatalf("Multi-layer encoding failed: %v", err)
	}

	t.Logf("Single-layer size: %d bytes", len(encodedSingle))
	t.Logf("Multi-layer size: %d bytes", len(encodedMulti))

	// Decode both
	decoderSingle := NewDecoder()
	if err := decoderSingle.Decode(encodedSingle); err != nil {
		t.Fatalf("Single-layer decoding failed: %v", err)
	}
	decodedSingle := decoderSingle.GetPixelData()

	decoderMulti := NewDecoder()
	if err := decoderMulti.Decode(encodedMulti); err != nil {
		t.Fatalf("Multi-layer decoding failed: %v", err)
	}
	decodedMulti := decoderMulti.GetPixelData()

	// Compare results
	singleError := 0
	multiError := 0
	for i := 0; i < len(pixelData); i++ {
		diffSingle := int(pixelData[i]) - int(decodedSingle[i])
		if diffSingle < 0 {
			diffSingle = -diffSingle
		}
		if diffSingle > singleError {
			singleError = diffSingle
		}

		diffMulti := int(pixelData[i]) - int(decodedMulti[i])
		if diffMulti < 0 {
			diffMulti = -diffMulti
		}
		if diffMulti > multiError {
			multiError = diffMulti
		}
	}

	t.Logf("Single-layer max error: %d", singleError)
	t.Logf("Multi-layer max error: %d", multiError)
	t.Logf("Single-layer first 10 decoded: %v", decodedSingle[:10])
	t.Logf("Multi-layer first 10 decoded: %v", decodedMulti[:10])
	t.Logf("Original first 10: %v", pixelData[:10])

	if singleError > 100 {
		t.Errorf("Single-layer decoding broken")
	}
	if multiError > 100 {
		t.Errorf("Multi-layer decoding broken")
	}
}
