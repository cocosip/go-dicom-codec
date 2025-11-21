package t1

import (
	"testing"
	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// TestEnableMQDebug - 启用MQ debug来找出不同步的地方
func TestEnableMQDebug(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Testing 5x5 gradient with MQ debug enabled")
	t.Logf("This will show the first 12 MQ operations for both encoder and decoder\n")

	// Enable MQ debug
	mqc.EnableEncoderDebug()

	// Encode
	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	t.Logf("\n=== Bitstream (%d bytes) ===", len(encoded))
	for i, b := range encoded {
		if i%16 == 0 {
			t.Logf("")
		}
		t.Logf("%02x ", b)
	}
	t.Logf("\n\n=== DECODER ===\n")

	// Enable decoder debug
	mqc.EnableDecoderDebug()

	// Decode
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetData()

	// Check result
	errorCount := 0
	for i := range input {
		if decoded[i] != input[i] {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Logf("\nResult: %d errors (%.1f%%)", errorCount, float64(errorCount)/float64(numPixels)*100)
	}
}
