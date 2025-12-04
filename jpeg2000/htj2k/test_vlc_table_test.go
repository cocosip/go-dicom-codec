package htj2k

import (
	"testing"
)

// TestVLCTableBuild tests VLC table construction
func TestVLCTableBuild(t *testing.T) {
	dec := NewVLCDecoder([]byte{0})

	// Check specific entries
	testCases := []struct {
		context uint8
		codeword uint8
		expectedRho uint8
	}{
		{0, 0x3F, 1},  // Should match {0, 0x1, 0x1, 0x1, 0x1, 0x3F, 7}
		{0, 0x06, 1},  // Should match {0, 0x1, 0x0, 0x0, 0x0, 0x06, 4}
	}

	for _, tc := range testCases {
		key := (uint32(tc.context) << 7) | uint32(tc.codeword)
		if key >= 1024 {
			t.Errorf("Key %d out of range", key)
			continue
		}

		packed := dec.tbl0[key]
		rho := uint8((packed >> 4) & 0xF)
		uOff := uint8((packed >> 3) & 0x1)
		e1 := uint8((packed >> 8) & 0xF)
		ek := uint8((packed >> 12) & 0xF)
		cwdLen := uint8(packed & 0x7)

		t.Logf("Key %d (context=%d, cwd=0x%02X):", key, tc.context, tc.codeword)
		t.Logf("  Packed: 0x%04X", packed)
		t.Logf("  rho=%d, uOff=%d, ek=%d, e1=%d, cwdLen=%d", rho, uOff, ek, e1, cwdLen)

		if rho != tc.expectedRho {
			t.Errorf("  Expected rho=%d, got %d", tc.expectedRho, rho)
		}
	}

	// Also check what happens during actual decode
	t.Logf("\n=== Testing actual VLC decode ===")

	// Encode with codeword 0x3F
	enc := NewVLCEncoder()
	err := enc.EncodeCxtVLC(0, 1, 1, 1, 1, true) // Should use cwd=0x3F
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	vlcData := enc.Flush()
	t.Logf("Encoded VLC data: %v", vlcData)

	// Decode
	dec2 := NewVLCDecoder(vlcData)
	rho, uOff, ek, e1, found := dec2.DecodeInitialRow(0)
	t.Logf("Decoded: rho=%d, uOff=%d, ek=%d, e1=%d, found=%v", rho, uOff, ek, e1, found)

	if rho != 1 {
		t.Errorf("Expected rho=1, got %d", rho)
	}
}
