package t1

import (
	"testing"
)

// TestVerifySignLogic - verify the sign flag logic is correct
func TestVerifySignLogic(t *testing.T) {
	// Test scenario: set up flags manually and verify getSignCodingContext behavior

	t.Run("east_negative", func(t *testing.T) {
		// Scenario: East neighbor is negative (significant + sign flag set)
		var flags uint32
		flags |= T1_SIG_E  // East is significant
		flags |= T1_SIGN_E // East is negative

		ctx := getSignCodingContext(flags)
		t.Logf("East negative: ctx=%d", ctx)

		// According to initSignContextLUT:
		// idx should have bit 2 set (East negative → h--)
		// We can't easily verify the exact context, but we can test consistency
	})

	t.Run("east_positive", func(t *testing.T) {
		// Scenario: East neighbor is positive (significant but no sign flag)
		var flags uint32
		flags |= T1_SIG_E  // East is significant
		// T1_SIGN_E NOT set → East is positive

		ctx := getSignCodingContext(flags)
		t.Logf("East positive: ctx=%d", ctx)

		// According to initSignContextLUT:
		// idx should have bit 1 set (East positive → h++)
	})

	t.Run("verify_current_logic", func(t *testing.T) {
		// Let's trace through the current logic manually

		// Case 1: East negative (T1_SIG_E=1, T1_SIGN_E=1)
		var flags1 uint32 = T1_SIG_E | T1_SIGN_E

		// Current code does:
		// if flags&T1_SIGN_E != 0 { // TRUE
		//     if flags&T1_SIG_E != 0 { // TRUE
		//         idx |= 1  // Sets bit 1 (positive???)

		t.Logf("Case 1 (East negative): T1_SIG_E=%v T1_SIGN_E=%v",
			flags1&T1_SIG_E != 0, flags1&T1_SIGN_E != 0)

		// The current code would set idx |= 1 (bit 1 = positive)
		// But we expect bit 2 (negative) to be set!
		// This confirms the bug!

		// Case 2: East positive (T1_SIG_E=1, T1_SIGN_E=0)
		var flags2 uint32 = T1_SIG_E

		// Current code does:
		// if flags&T1_SIGN_E != 0 { // FALSE
		//     // Not executed
		// So idx remains 0, no bits set!

		t.Logf("Case 2 (East positive): T1_SIG_E=%v T1_SIGN_E=%v",
			flags2&T1_SIG_E != 0, flags2&T1_SIGN_E != 0)

		// The current code would NOT set any bits
		// But we expect bit 1 (positive) to be set!
		// This also confirms the bug!

		t.Logf("\nConclusion: The current logic is INVERTED!")
		t.Logf("It should check T1_SIG_X first, then check T1_SIGN_X for the sign")
	})
}
