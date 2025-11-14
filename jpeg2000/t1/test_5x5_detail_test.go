package t1

import (
	"testing"
)

func Test5x5Detail(t *testing.T) {
	width, height := 5, 5
	numPixels := width * height
	input := make([]int32, numPixels)

	// Generate pattern (same as TestRLBoundaryConditions)
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	maxBitplane := 7
	numPasses := (maxBitplane + 1) * 3

	encoder := NewT1Encoder(width, height, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	decoder := NewT1Decoder(width, height, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	t.Logf("\n5x5 Input vs Output:")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			marker := "✓"
			if decoded[i] != input[i] {
				marker = "✗"
			}
			t.Logf("  [%d,%d] input=%4d output=%4d %s", x, y, input[i], decoded[i], marker)
		}
	}
}
