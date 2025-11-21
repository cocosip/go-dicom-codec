package t1

import (
	"testing"
)

// TestCheckDiagonalFlags - 检查对角邻居标志是否正确设置
func TestCheckDiagonalFlags(t *testing.T) {
	// 创建一个简单的测试:中心系数变significant,检查8个邻居的标志
	size := 3
	encoder := NewT1Encoder(size, size, 0)
	paddedWidth := size + 2
	paddedSize := paddedWidth * (size + 2)
	encoder.data = make([]int32, paddedSize)
	encoder.flags = make([]uint32, paddedSize)

	// 设置中心系数(1,1)为significant和negative
	centerX, centerY := 1, 1
	centerIdx := (centerY+1)*paddedWidth + (centerX + 1)
	encoder.data[centerIdx] = -100
	encoder.flags[centerIdx] = T1_SIG | T1_SIGN

	t.Logf("Center (%d,%d) idx=%d: SIG=1, SIGN=1 (negative)", centerX, centerY, centerIdx)

	// 调用updateNeighborFlags
	encoder.updateNeighborFlags(centerX, centerY, centerIdx)

	// 检查所有8个邻居
	checks := []struct {
		name      string
		dx, dy    int
		expectSig uint32
		expectSign uint32
	}{
		{"North", 0, -1, T1_SIG_S, T1_SIGN_S},
		{"South", 0, 1, T1_SIG_N, T1_SIGN_N},
		{"West", -1, 0, T1_SIG_E, T1_SIGN_E},
		{"East", 1, 0, T1_SIG_W, T1_SIGN_W},
		{"NW", -1, -1, T1_SIG_SE, 0}, // 对角:只有SIG,没有SIGN
		{"NE", 1, -1, T1_SIG_SW, 0},
		{"SW", -1, 1, T1_SIG_NE, 0},
		{"SE", 1, 1, T1_SIG_NW, 0},
	}

	allCorrect := true
	for _, chk := range checks {
		nx, ny := centerX+chk.dx, centerY+chk.dy
		nIdx := (ny+1)*paddedWidth + (nx + 1)
		flags := encoder.flags[nIdx]

		hasSig := (flags & chk.expectSig) != 0
		hasSign := chk.expectSign == 0 || (flags & chk.expectSign) != 0

		t.Logf("%s (%d,%d) idx=%d:", chk.name, nx, ny, nIdx)
		t.Logf("  Expected SIG flag 0x%x: %v", chk.expectSig, hasSig)
		if chk.expectSign != 0 {
			t.Logf("  Expected SIGN flag 0x%x: %v", chk.expectSign, hasSign)
		}

		if !hasSig {
			t.Errorf("  ❌ SIG flag NOT set!")
			allCorrect = false
		}
		if chk.expectSign != 0 && !hasSign {
			t.Errorf("  ❌ SIGN flag NOT set!")
			allCorrect = false
		}
	}

	if allCorrect {
		t.Log("\n✅ All neighbor flags set correctly")
	}
}
