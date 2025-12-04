package htj2k

import (
	"fmt"
	"testing"
)

// Test2x2Simple tests a minimal 2x2 block (1 quad)
func Test2x2Simple(t *testing.T) {
	width := 2
	height := 2

	// Simple test data: all samples = 7
	testCoeffs := []int32{7, 7, 7, 7}

	t.Logf("Input: %v", testCoeffs)

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes: %v", len(encoded), encoded)

	// Decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %v", decoded)

	// Compare
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("Mismatch at [%d]: expected %d, got %d", i, testCoeffs[i], decoded[i])
		}
	}
}

// Test4x2TwoQuads tests 4x2 block (2 quads in same row)
func Test4x2TwoQuads(t *testing.T) {
	width := 4
	height := 2

	// Two quads: Q(0,0) and Q(1,0)
	// Q(0,0): [1,2,3,4]
	// Q(1,0): [5,6,7,8]
	testCoeffs := []int32{
		1, 2, // row 0
		3, 4, // row 1
		5, 6, // row 0 (quad 1)
		7, 8, // row 1 (quad 1)
	}

	t.Logf("Input: %v", testCoeffs)

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded: %d bytes: %v", len(encoded), encoded)

	// Parse segments
	if len(encoded) >= 2 {
		melLen := int(encoded[len(encoded)-2])
		vlcLen := int(encoded[len(encoded)-1])
		totalDataLen := len(encoded) - 2
		magsgnLen := totalDataLen - melLen - vlcLen

		t.Logf("Segments: MagSgn=%d, MEL=%d, VLC=%d", magsgnLen, melLen, vlcLen)

		if vlcLen > 0 && magsgnLen >= 0 && melLen >= 0 {
			vlcStart := magsgnLen + melLen
			vlcEnd := vlcStart + vlcLen
			vlcData := encoded[vlcStart:vlcEnd]
			t.Logf("VLC data: %v", vlcData)

			// Print VLC data in binary
			for i, b := range vlcData {
				t.Logf("  VLC[%d]: 0x%02X = %08b", i, b, b)
			}
		}
	}

	// Decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %v", decoded)

	// Compare
	errors := 0
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("Mismatch at [%d]: expected %d, got %d", i, testCoeffs[i], decoded[i])
			errors++
		}
	}

	if errors == 0 {
		t.Logf("✓ All values match!")
	} else {
		t.Logf("Found %d errors", errors)
	}
}

// Test4x2TwoQuadsDebug adds detailed debug output
func Test4x2TwoQuadsDebug(t *testing.T) {
	width := 4
	height := 2

	testCoeffs := []int32{1, 2, 3, 4, 5, 6, 7, 8}

	// Calculate expected encoding
	t.Logf("=== Expected Encoding ===")
	t.Logf("Q(0,0): samples=[1,2,3,4], all significant, rho=0xF")
	t.Logf("  maxMag=4, Uq=3 bits")
	t.Logf("  Kq=0 (first quad), u=Uq-Kq=3")
	t.Logf("  ULF=1 (u>0)")
	t.Logf("  U-VLC: u=3, InitialPair mode: encode u-2=1")
	t.Logf("    u-2=1 → pfx=1, 1 bit: '1'")

	t.Logf("Q(1,0): samples=[5,6,7,8], all significant, rho=0xF")
	t.Logf("  maxMag=8, Uq=4 bits")
	t.Logf("  Kq≈0-1, assume u=3-4")
	t.Logf("  ULF=1")
	t.Logf("  Since Q(0,0).Uq=3 ≤ 2? No, so NOT simplified")
	t.Logf("  Or if Q(0,0).Uq=3 > 2? Yes, so SIMPLIFIED")
	t.Logf("  U-VLC simplified: encode u-1 as 1 bit")

	// Encode
	encoder := NewHTEncoder(width, height)
	encoded, err := encoder.Encode(testCoeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("\n=== Actual Encoding ===")
	t.Logf("Encoded: %d bytes", len(encoded))
	fmt.Printf("Raw bytes: ")
	for _, b := range encoded {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()

	// Decode
	decoder := NewHTDecoder(width, height)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Compare
	match := true
	for i := 0; i < len(testCoeffs); i++ {
		if testCoeffs[i] != decoded[i] {
			t.Errorf("[%d] expected %d, got %d", i, testCoeffs[i], decoded[i])
			match = false
		}
	}

	if match {
		t.Logf("✓ Perfect match!")
	}
}
