package htj2k

import (
	"testing"
)

// Test4x4Direct tests 4x4 encoding/decoding directly
func Test4x4Direct(t *testing.T) {
	width := 4
	height := 4
	size := width * height

	// Same data as TestHTBlockEncoderDecoder
	testCoeffs := make([]int32, size)
	for i := 0; i < size; i++ {
		testCoeffs[i] = int32(i - size/2) // [-8, -7, ..., 6, 7]
	}

	t.Logf("Input: %v", testCoeffs)

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes: %v", len(encoded), encoded)

	// Try to decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Logf("Decode error: %v", err)

		// Parse segments to debug
		if len(encoded) >= 2 {
			melLen := int(encoded[len(encoded)-2])
			vlcLen := int(encoded[len(encoded)-1])
			t.Logf("Segment lengths: MEL=%d, VLC=%d", melLen, vlcLen)

			totalDataLen := len(encoded) - 2
			magsgnLen := totalDataLen - melLen - vlcLen
			t.Logf("MagSgn length: %d", magsgnLen)

			if vlcLen > 0 && melLen + vlcLen + magsgnLen == totalDataLen {
				vlcStart := magsgnLen + melLen
				vlcEnd := vlcStart + vlcLen
				vlcData := encoded[vlcStart:vlcEnd]
				t.Logf("VLC data: %v", vlcData)
			}
		}

		t.Fatalf("Decode failed")
	}

	t.Logf("Decoded: %v", decoded)

	// Compare
	errors := 0
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Logf("Mismatch at [%d]: expected %d, got %d", i, testCoeffs[i], decoded[i])
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Found %d errors", errors)
	} else {
		t.Logf("✓ All values match!")
	}
}
