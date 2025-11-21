package t1

import (
	"fmt"
	"testing"
)

// TestCompareBitstreams - Compare encoded bitstreams for 4x4 vs 5x5
func TestCompareBitstreams(t *testing.T) {
	testCases := []struct {
		size int
		name string
	}{
		{4, "4x4"},
		{5, "5x5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size
			numPixels := size * size
			input := make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				input[i] = int32(i%256) - 128
			}

			maxBitplane := CalculateMaxBitplane(input)
			numPasses := (maxBitplane + 1) * 3

			encoder := NewT1Encoder(size, size, 0)
			encoded, err := encoder.Encode(input, numPasses, 0)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			t.Logf("%s: maxBitplane=%d, numPasses=%d, encoded_bytes=%d",
				tc.name, maxBitplane, numPasses, len(encoded))

			// Print first 20 bytes of bitstream in hex
			printLen := 20
			if len(encoded) < printLen {
				printLen = len(encoded)
			}
			hex := ""
			for i := 0; i < printLen; i++ {
				hex += fmt.Sprintf("%02x ", encoded[i])
			}
			t.Logf("%s bitstream: %s", tc.name, hex)

			// Decode and check
			decoder := NewT1Decoder(size, size, 0)
			err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decoded := decoder.GetData()

			errorCount := 0
			for i := range input {
				if decoded[i] != input[i] {
					errorCount++
				}
			}

			if errorCount > 0 {
				t.Logf("%s: %d errors", tc.name, errorCount)
			} else {
				t.Logf("%s: PASS (0 errors)", tc.name)
			}
		})
	}
}
