package htj2k

import (
	"testing"
)

// TestVLCNoStuffing tests VLC encoding without bit-stuffing
func TestVLCNoStuffing(t *testing.T) {
	enc := NewVLCEncoder()

	// Use entry that won't trigger bit-stuffing
	// From VLC_tbl0: {0, 0x1, 0x0, 0x0, 0x0, 0x06, 4}
	// This encodes to codeword 0x06 (0110) with length 4
	// Codeword 0x06 = 0110 (4 bits), LSB-first: 0,1,1,0

	context := uint8(0)
	rho := uint8(1)
	uOff := uint8(0)
	ek := uint8(0)
	e1 := uint8(0)

	t.Logf("=== ENCODING ===")
	t.Logf("Params: context=%d, rho=%d, uOff=%d, ek=%d, e1=%d", context, rho, uOff, ek, e1)

	// Check table entry
	key := encodeKey{cq: context, rho: rho, uOff: uOff, ek: ek, e1: e1}
	entry, found := enc.encodeMap0[key]
	t.Logf("Table entry found: %v", found)
	if found {
		t.Logf("  Codeword: 0x%02X (%08b), Length: %d", entry.Codeword, entry.Codeword, entry.Length)
	}

	// Encode
	err := enc.EncodeCxtVLC(context, rho, uOff, ek, e1, true)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("\n=== BEFORE FLUSH ===")
	t.Logf("vlcBuf: %v", enc.vlcBuf)
	t.Logf("vlcTmp: %d (0x%02X, %08b)", enc.vlcTmp, enc.vlcTmp, enc.vlcTmp)
	t.Logf("vlcBits: %d", enc.vlcBits)

	vlcData := enc.Flush()

	t.Logf("\n=== ENCODED VLC DATA ===")
	for i, b := range vlcData {
		t.Logf("  [%d]: %d (0x%02X, %08b)", i, b, b, b)
	}

	// Decode
	t.Logf("\n=== DECODING ===")
	dec := NewVLCDecoder(vlcData)
	rho2, uOff2, ek2, e12, found2 := dec.DecodeInitialRow(context)
	t.Logf("Decoded: rho=%d, uOff=%d, ek=%d, e1=%d, found=%v", rho2, uOff2, ek2, e12, found2)

	// Check results
	if rho != rho2 {
		t.Errorf("Rho mismatch: encoded=%d, decoded=%d", rho, rho2)
	}
	if uOff != uOff2 {
		t.Errorf("UOff mismatch: encoded=%d, decoded=%d", uOff, uOff2)
	}

	t.Logf("\n=== SUCCESS ===")
}
