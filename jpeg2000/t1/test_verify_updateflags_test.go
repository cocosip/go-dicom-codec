package t1

import (
	"testing"
)

// TestVerifyUpdateNeighborFlags - 详细验证updateNeighborFlags的正确性
func TestVerifyUpdateNeighborFlags(t *testing.T) {
	t.Run("center_coefficient_negative", func(t *testing.T) {
		// 创建3x3矩阵，中心系数为负数
		width, height := 3, 3
		encoder := NewT1Encoder(width, height, 0)
		paddedWidth := width + 2
		paddedSize := paddedWidth * (height + 2)
		encoder.data = make([]int32, paddedSize)
		encoder.flags = make([]uint32, paddedSize)

		// 设置中心系数(1,1)为负数significant
		centerX, centerY := 1, 1
		centerIdx := (centerY+1)*paddedWidth + (centerX + 1)
		encoder.data[centerIdx] = -100
		encoder.flags[centerIdx] = T1_SIG | T1_SIGN

		t.Logf("Center coefficient at (%d,%d) idx=%d: value=-100, SIG=1, SIGN=1",
			centerX, centerY, centerIdx)

		// 调用updateNeighborFlags
		encoder.updateNeighborFlags(centerX, centerY, centerIdx)

		// 验证8个邻居的标志
		tests := []struct {
			name      string
			dx, dy    int
			sigFlag   uint32
			signFlag  uint32
			shouldSet bool // 是否应该设置（考虑边界检查）
		}{
			{"North", 0, -1, T1_SIG_S, T1_SIGN_S, true},      // y > 0
			{"South", 0, 1, T1_SIG_N, T1_SIGN_N, true},       // y < height-1
			{"West", -1, 0, T1_SIG_E, T1_SIGN_E, true},       // x > 0
			{"East", 1, 0, T1_SIG_W, T1_SIGN_W, true},        // x < width-1
			{"NorthWest", -1, -1, T1_SIG_SE, 0, true},        // x > 0 && y > 0
			{"NorthEast", 1, -1, T1_SIG_SW, 0, true},         // x < width-1 && y > 0
			{"SouthWest", -1, 1, T1_SIG_NE, 0, true},         // x > 0 && y < height-1
			{"SouthEast", 1, 1, T1_SIG_NW, 0, true},          // x < width-1 && y < height-1
		}

		allCorrect := true
		for _, tt := range tests {
			nx, ny := centerX+tt.dx, centerY+tt.dy
			nIdx := (ny+1)*paddedWidth + (nx + 1)
			flags := encoder.flags[nIdx]

			hasSig := flags&tt.sigFlag != 0
			hasSign := tt.signFlag == 0 || flags&tt.signFlag != 0

			t.Logf("  %s neighbor (%d,%d) idx=%d:", tt.name, nx, ny, nIdx)
			t.Logf("    SIG flag (0x%x): %v", tt.sigFlag, hasSig)
			if tt.signFlag != 0 {
				t.Logf("    SIGN flag (0x%x): %v", tt.signFlag, hasSign)
			}

			if tt.shouldSet {
				if !hasSig {
					t.Errorf("    ❌ SIG flag NOT set (expected)")
					allCorrect = false
				}
				if tt.signFlag != 0 && !hasSign {
					t.Errorf("    ❌ SIGN flag NOT set (expected for negative center)")
					allCorrect = false
				}
			}
		}

		if allCorrect {
			t.Logf("\n✅ All neighbor flags set correctly")
		}
	})

	t.Run("edge_coefficient_boundary_checks", func(t *testing.T) {
		// 测试边界系数的updateNeighborFlags行为
		width, height := 3, 3
		encoder := NewT1Encoder(width, height, 0)
		paddedWidth := width + 2
		paddedSize := paddedWidth * (height + 2)
		encoder.data = make([]int32, paddedSize)
		encoder.flags = make([]uint32, paddedSize)

		// 测试左上角系数(0,0)
		cornerX, cornerY := 0, 0
		cornerIdx := (cornerY+1)*paddedWidth + (cornerX + 1)
		encoder.data[cornerIdx] = -50
		encoder.flags[cornerIdx] = T1_SIG | T1_SIGN

		t.Logf("Testing corner coefficient at (%d,%d)", cornerX, cornerY)
		encoder.updateNeighborFlags(cornerX, cornerY, cornerIdx)

		// North: y=0, y > 0 is FALSE, 不应该更新
		northIdx := (cornerY)*paddedWidth + (cornerX + 1)
		if encoder.flags[northIdx]&T1_SIG_S != 0 {
			t.Errorf("❌ North neighbor in padding should NOT be updated (y=0)")
		} else {
			t.Logf("✅ North neighbor correctly NOT updated (boundary check)")
		}

		// West: x=0, x > 0 is FALSE, 不应该更新
		westIdx := (cornerY+1)*paddedWidth + cornerX
		if encoder.flags[westIdx]&T1_SIG_E != 0 {
			t.Errorf("❌ West neighbor in padding should NOT be updated (x=0)")
		} else {
			t.Logf("✅ West neighbor correctly NOT updated (boundary check)")
		}

		// South: y=0, y < height-1=2 is TRUE, 应该更新
		southIdx := (cornerY+2)*paddedWidth + (cornerX + 1)
		if encoder.flags[southIdx]&T1_SIG_N == 0 {
			t.Errorf("❌ South neighbor should be updated")
		} else {
			t.Logf("✅ South neighbor correctly updated")
		}

		// East: x=0, x < width-1=2 is TRUE, 应该更新
		eastIdx := (cornerY+1)*paddedWidth + (cornerX + 2)
		if encoder.flags[eastIdx]&T1_SIG_W == 0 {
			t.Errorf("❌ East neighbor should be updated")
		} else {
			t.Logf("✅ East neighbor correctly updated")
		}
	})

	t.Run("last_row_coefficient", func(t *testing.T) {
		// 关键测试：最后一行的系数
		width, height := 5, 4
		encoder := NewT1Encoder(width, height, 0)
		paddedWidth := width + 2
		paddedSize := paddedWidth * (height + 2)
		encoder.data = make([]int32, paddedSize)
		encoder.flags = make([]uint32, paddedSize)

		// 测试最后一行(y=3)的第一个系数
		x, y := 0, 3
		idx := (y+1)*paddedWidth + (x + 1)
		encoder.data[idx] = -100
		encoder.flags[idx] = T1_SIG | T1_SIGN

		t.Logf("\nTesting LAST row coefficient at (%d,%d) in %dx%d", x, y, width, height)
		encoder.updateNeighborFlags(x, y, idx)

		// South neighbor: y=3, y < height-1=3 is FALSE
		// 所以South邻居（在padding中）不应该被更新
		southIdx := (y+2)*paddedWidth + (x + 1)
		if encoder.flags[southIdx]&T1_SIG_N != 0 {
			t.Errorf("❌ South neighbor in padding should NOT be updated (y=%d >= height-1=%d)",
				y, height-1)
			t.Logf("   This is the ROOT CAUSE of 5x4 vs 5x5 difference!")
		} else {
			t.Logf("✅ South neighbor correctly NOT updated (boundary check prevented)")
			t.Logf("   For 5x4: row 3 is last row, South not updated")
			t.Logf("   For 5x5: row 3 is NOT last row, South WILL be updated")
			t.Logf("   This causes different encoding!")
		}
	})
}
