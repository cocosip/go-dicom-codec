package t1

import (
	"testing"
)

// TestVerifySignCodingContext - 验证getSignCodingContext的逻辑正确性
func TestVerifySignCodingContext(t *testing.T) {
	t.Run("no_significant_neighbors", func(t *testing.T) {
		// 没有任何significant邻居
		var flags uint32 = 0
		ctx := getSignCodingContext(flags)
		t.Logf("No significant neighbors: ctx=%d", ctx)

		// 验证idx应该是0（没有任何位被设置）
		// ctx应该是lut_ctxno_sc[0] + CTX_SC_START
		// 从initSignContextLUT可知，idx=0时h=0,v=0
		// 对应context应该是2（相对CTX_SC_START）
		expectedCtx := uint8(2 + CTX_SC_START) // CTX_SC_START=9, so 11
		if ctx != expectedCtx {
			t.Errorf("Expected ctx=%d, got %d", expectedCtx, ctx)
		} else {
			t.Logf("✅ Context correct for no neighbors")
		}
	})

	t.Run("east_negative_only", func(t *testing.T) {
		// 只有East邻居significant且为负数
		var flags uint32 = T1_SIG_E | T1_SIGN_E

		t.Logf("East negative: T1_SIG_E=%v, T1_SIGN_E=%v",
			flags&T1_SIG_E != 0, flags&T1_SIGN_E != 0)

		ctx := getSignCodingContext(flags)
		t.Logf("Context: %d", ctx)

		// 根据当前代码逻辑（虽然看起来反了）：
		// if flags&T1_SIGN_E != 0 { // TRUE
		//     if flags&T1_SIG_E != 0 { // TRUE
		//         idx |= 1  // 设置bit 1
		// 所以idx = 1

		// 从initSignContextLUT: bit 1 → East positive → h++
		// h=1, v=0 → context = 1 (CTX_SC_START=9, so 10)

		// 但是逻辑上，East是negative，应该是h--，期望context不同
		// 这里我们只验证当前代码是否一致工作
		t.Logf("  Current code sets idx bit 1 (comments say 'East positive')")
		t.Logf("  But East is actually negative!")
	})

	t.Run("east_positive_only", func(t *testing.T) {
		// 只有East邻居significant且为正数（没有SIGN标志）
		var flags uint32 = T1_SIG_E
		// T1_SIGN_E NOT set

		t.Logf("East positive: T1_SIG_E=%v, T1_SIGN_E=%v",
			flags&T1_SIG_E != 0, flags&T1_SIGN_E != 0)

		ctx := getSignCodingContext(flags)
		t.Logf("Context: %d", ctx)

		// 根据当前代码逻辑：
		// if flags&T1_SIGN_E != 0 { // FALSE
		//     // Not executed
		// 所以idx = 0（没有位被设置）

		// 从initSignContextLUT: idx=0 → h=0, v=0
		// 这意味着当前代码认为"没有significant邻居"
		// 但实际上East是positive且significant！

		t.Logf("  Current code sets idx = 0 (no bits)")
		t.Logf("  But East is positive and significant!")
		t.Logf("  ❌ This is the BUG!")
	})

	t.Run("trace_current_logic_bug", func(t *testing.T) {
		t.Logf("\n=== ANALYSIS OF CURRENT LOGIC ===")
		t.Logf("Current code structure:")
		t.Logf("  if flags&T1_SIGN_E != 0 {")
		t.Logf("      if flags&T1_SIG_E != 0 {")
		t.Logf("          idx |= 1  // comment: East positive")
		t.Logf("      } else {")
		t.Logf("          idx |= 2  // comment: East negative")
		t.Logf("      }")
		t.Logf("  }")
		t.Logf("")
		t.Logf("Problem 1: Checks T1_SIGN_E first, not T1_SIG_E")
		t.Logf("  → If neighbor not significant, shouldn't contribute to context")
		t.Logf("  → But code might set bits even if not significant")
		t.Logf("")
		t.Logf("Problem 2: Logic appears inverted")
		t.Logf("  → T1_SIGN_E means East is negative")
		t.Logf("  → But when T1_SIGN_E=1 AND T1_SIG_E=1, sets bit 1 (positive)")
		t.Logf("")
		t.Logf("Problem 3: When T1_SIG_E=1 but T1_SIGN_E=0 (East positive)")
		t.Logf("  → Outer if is false, no bits set")
		t.Logf("  → Positive significant neighbor is ignored!")
		t.Logf("")
		t.Logf("HOWEVER: Tests pass with current logic for simple cases")
		t.Logf("  → Maybe there's a compensating error elsewhere?")
		t.Logf("  → Or maybe my understanding of T1_SIGN semantics is still wrong?")
	})

	t.Run("verify_lut_initialization", func(t *testing.T) {
		// 验证LUT初始化的逻辑
		t.Logf("\n=== LUT INITIALIZATION LOGIC ===")
		t.Logf("From initSignContextLUT:")
		t.Logf("  if i&1 != 0: h++    // bit 1 = East positive")
		t.Logf("  if i&2 != 0: h--    // bit 2 = East negative")
		t.Logf("")

		// 测试几个idx值
		testCases := []struct {
			idx int
			desc string
		}{
			{0, "no neighbors"},
			{1, "East positive only (bit 1)"},
			{2, "East negative only (bit 2)"},
			{4, "West positive only"},
			{8, "West negative only"},
		}

		for _, tc := range testCases {
			ctx := lut_ctxno_sc[tc.idx] + CTX_SC_START
			t.Logf("  idx=%d (%s): context=%d", tc.idx, tc.desc, ctx)
		}
	})
}
