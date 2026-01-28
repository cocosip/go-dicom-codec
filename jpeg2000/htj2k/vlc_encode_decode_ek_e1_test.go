package htj2k

import (
	"testing"
)

// TestVLCEncodeDecodeEKE1 tests VLC encoding/decoding for ek=0xF, e1=0xF
func TestVLCEncodeDecodeEKE1(t *testing.T) {
	// Our case: ctx=0, rho=0xF, uOff=1, ek=0xF, e1=0xF, isFirstRow=true
	context := uint8(0)
	rho := uint8(0xF)
	uOff := uint8(1)
	ek := uint8(0xF)
	e1 := uint8(0xF)
	isFirstRow := true

	t.Logf("=== Input ===")
	t.Logf("ctx=%d, rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X, isFirstRow=%v",
		context, rho, uOff, ek, e1, isFirstRow)

	// Test encoding
	encoder := NewVLCEncoder()
	length, err := encoder.EncodeCxtVLCWithLen(context, rho, uOff, ek, e1, isFirstRow)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded: %d bits", length)

	vlcData := encoder.Flush()
	t.Logf("VLC data: %d bytes: %X", len(vlcData), vlcData)

	// Test decoding
	decoder := NewVLCDecoder(vlcData)
	decodedRho, decodedUOff, decodedEK, decodedE1, found := decoder.DecodeQuadWithContext(context, isFirstRow)
	if !found {
		t.Fatalf("Decode failed: no match found")
	}

	t.Logf("\n=== Output ===")
	t.Logf("Decoded: rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", decodedRho, decodedUOff, decodedEK, decodedE1)

	// Check results
	pass := true
	if decodedRho != rho {
		t.Errorf("rho mismatch: expected 0x%X, got 0x%X", rho, decodedRho)
		pass = false
	}
	if decodedUOff != uOff {
		t.Errorf("uOff mismatch: expected %d, got %d", uOff, decodedUOff)
		pass = false
	}
	if decodedEK != ek {
		t.Errorf("ek mismatch: expected 0x%X, got 0x%X", ek, decodedEK)
		pass = false
	}
	if decodedE1 != e1 {
		t.Errorf("e1 mismatch: expected 0x%X, got 0x%X", e1, decodedE1)
		pass = false
	}

	if pass {
		t.Logf("✓ VLC roundtrip successful")
	} else {
		t.Logf("✗ VLC roundtrip FAILED")
	}
}

// TestVLCVariousEKE1 tests various ek/e1 combinations
func TestVLCVariousEKE1(t *testing.T) {
	cases := []struct {
		name string
		ek   uint8
		e1   uint8
	}{
		{"ek=0xE,e1=0x6", 0xE, 0x6},
		{"ek=0xF,e1=0xC", 0xF, 0xC},
		{"ek=0xF,e1=0xF", 0xF, 0xF},
		{"ek=0x0,e1=0x0", 0x0, 0x0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			context := uint8(0)
			rho := uint8(0xF)
			uOff := uint8(1)
			isFirstRow := true

			encoder := NewVLCEncoder()
			_, err := encoder.EncodeCxtVLCWithLen(context, rho, uOff, tc.ek, tc.e1, isFirstRow)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			vlcData := encoder.Flush()
			decoder := NewVLCDecoder(vlcData)
			decodedRho, decodedUOff, decodedEK, decodedE1, found := decoder.DecodeQuadWithContext(context, isFirstRow)
			if !found {
				t.Fatalf("Decode failed")
			}

			t.Logf("Input:  ek=0x%X, e1=0x%X", tc.ek, tc.e1)
			t.Logf("Output: ek=0x%X, e1=0x%X", decodedEK, decodedE1)

			if decodedRho != rho || decodedUOff != uOff || decodedEK != tc.ek || decodedE1 != tc.e1 {
				t.Errorf("Mismatch")
			}
		})
	}
}
