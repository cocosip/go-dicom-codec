package t1

import (
	"testing"
)

// TestCheckSignSemantics - verify what T1_SIGN actually means
func TestCheckSignSemantics(t *testing.T) {
	// Create a simple 3x1 array: [positive, negative, positive]
	width, height := 3, 1
	input := []int32{10, -20, 30}

	encoder := NewT1Encoder(width, height, 0)
	paddedWidth := width + 2
	paddedSize := paddedWidth * (height + 2)

	// Manually set up the encoder with data
	encoder.data = make([]int32, paddedSize)
	encoder.flags = make([]uint32, paddedSize)
	for i, val := range input {
		idx := (0+1)*paddedWidth + (i + 1) // row 0, columns 0,1,2
		encoder.data[idx] = val
	}

	// Now manually set the flags as if coefficients became significant
	for i, val := range input {
		idx := (0+1)*paddedWidth + (i + 1)
		encoder.flags[idx] |= T1_SIG
		if val < 0 {
			encoder.flags[idx] |= T1_SIGN
			t.Logf("Coeff[%d]=%d: T1_SIGN SET (negative)", i, val)
		} else {
			t.Logf("Coeff[%d]=%d: T1_SIGN NOT set (positive)", i, val)
		}
	}

	// Now call updateNeighborFlags for the middle coefficient (negative)
	x, y := 1, 0
	idx := (y+1)*paddedWidth + (x + 1)
	t.Logf("\nCalling updateNeighborFlags for coeff[1]=%d at idx=%d", input[1], idx)

	encoder.updateNeighborFlags(x, y, idx)

	// Check West neighbor (coeff 0, positive)
	wIdx := (y+1)*paddedWidth + (x)
	t.Logf("\nWest neighbor (coeff[0]=%d) flags:", input[0])
	t.Logf("  T1_SIG_E=%v (East neighbor [1] is significant)",
		encoder.flags[wIdx]&T1_SIG_E != 0)
	t.Logf("  T1_SIGN_E=%v (East neighbor [1] is negative?)",
		encoder.flags[wIdx]&T1_SIGN_E != 0)

	// Check East neighbor (coeff 2, positive)
	eIdx := (y+1)*paddedWidth + (x + 2)
	t.Logf("\nEast neighbor (coeff[2]=%d) flags:", input[2])
	t.Logf("  T1_SIG_W=%v (West neighbor [1] is significant)",
		encoder.flags[eIdx]&T1_SIG_W != 0)
	t.Logf("  T1_SIGN_W=%v (West neighbor [1] is negative?)",
		encoder.flags[eIdx]&T1_SIGN_W != 0)

	// Verification
	if encoder.flags[wIdx]&T1_SIGN_E != 0 {
		t.Logf("\n✓ Confirmed: T1_SIGN_E is SET when East neighbor is NEGATIVE")
	}
	if encoder.flags[eIdx]&T1_SIGN_W != 0 {
		t.Logf("✓ Confirmed: T1_SIGN_W is SET when West neighbor is NEGATIVE")
	}
}
