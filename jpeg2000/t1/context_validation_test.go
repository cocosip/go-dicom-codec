package t1

import (
	"testing"
)

// TestSignContextLUTAlignment 验证 Sign Context LUT 与 OpenJPEG 的对齐
// 参考: OpenJPEG t1_luts.h lut_ctxno_sc[256]
func TestSignContextLUTAlignment(t *testing.T) {
	// 从 OpenJPEG t1_luts.h 提取的 lut_ctxno_sc[256]
	openjpeg_lut_ctxno_sc := [256]uint8{
		0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb,
		0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd,
		0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0x9, 0xd, 0xa, 0x9, 0xc, 0xa, 0xb,
		0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0x9, 0xb, 0xa, 0x9, 0xc, 0xa, 0xd,
		0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb,
		0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd,
		0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0x9, 0xd, 0xa, 0x9, 0xc, 0xa, 0xb,
		0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0x9, 0xb, 0xa, 0x9, 0xc, 0xa, 0xd,
		0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb,
		0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xd, 0xb, 0xc, 0xc, 0xd, 0xb, 0xc, 0xc,
		0xd, 0xd, 0xd, 0xd, 0xb, 0xb, 0xb, 0xb, 0xd, 0xa, 0xd, 0xa, 0xa, 0xb, 0xa, 0xb,
		0xd, 0xd, 0xc, 0xc, 0xb, 0xb, 0xc, 0xc, 0xd, 0xa, 0xc, 0x9, 0xa, 0xb, 0x9, 0xc,
		0xa, 0xa, 0x9, 0x9, 0xa, 0xa, 0x9, 0x9, 0xb, 0xd, 0xc, 0xc, 0xb, 0xd, 0xc, 0xc,
		0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd, 0xb, 0xd,
		0xb, 0xb, 0xc, 0xc, 0xd, 0xd, 0xc, 0xc, 0xb, 0xa, 0xc, 0x9, 0xa, 0xd, 0x9, 0xc,
		0xb, 0xb, 0xb, 0xb, 0xd, 0xd, 0xd, 0xd, 0xb, 0xa, 0xb, 0xa, 0xa, 0xd, 0xa, 0xd,
	}

	// 验证每个值
	errorCount := 0
	for i := 0; i < 256; i++ {
		expected := openjpeg_lut_ctxno_sc[i]
		actual := lut_ctxno_sc[i] + CTX_SC_START // 注意：go实现返回相对值，需要加上CTX_SC_START(9)

		if actual != expected {
			t.Errorf("lut_ctxno_sc[%d]: 期望 0x%x, 得到 0x%x", i, expected, actual)
			errorCount++
			if errorCount >= 10 {
				t.Fatalf("发现太多错误，停止检查")
			}
		}
	}

	if errorCount == 0 {
		t.Logf("✅ 所有 256 个 Sign Context LUT 条目与 OpenJPEG 完全一致")
	}
}

// TestSignPredictionLUTAlignment 验证 Sign Prediction LUT 与 OpenJPEG 的对齐
// 参考: OpenJPEG t1_luts.h lut_spb[256]
func TestSignPredictionLUTAlignment(t *testing.T) {
	// 从 OpenJPEG t1_luts.h 提取的 lut_spb[256]
	openjpeg_lut_spb := [256]int{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 1,
		1, 1, 0, 0, 1, 1, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 0, 1, 1, 0, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1,
	}

	// 验证每个值
	errorCount := 0
	for i := 0; i < 256; i++ {
		expected := openjpeg_lut_spb[i]
		actual := lut_spb[i]

		if actual != expected {
			t.Errorf("lut_spb[%d]: 期望 %d, 得到 %d", i, expected, actual)
			errorCount++
			if errorCount >= 10 {
				t.Fatalf("发现太多错误，停止检查")
			}
		}
	}

	if errorCount == 0 {
		t.Logf("✅ 所有 256 个 Sign Prediction LUT 条目与 OpenJPEG 完全一致")
	}
}

// TestZeroCodingContextLogic 验证 Zero Coding 上下文计算逻辑
// OpenJPEG 使用预计算的 2048 项 LUT，我们动态计算
// 需要验证动态计算的结果与 OpenJPEG LUT 一致
func TestZeroCodingContextLogic(t *testing.T) {
	// 从 OpenJPEG t1_luts.h 提取的部分 lut_ctxno_zc 值
	// 完整表有 2048 项，我们测试关键样本
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{
			name:     "无邻居显著",
			flags:    0,
			expected: 0, // 上下文 0
		},
		{
			name:     "仅左邻居",
			flags:    T1_SIG_W,
			expected: 2, // 水平贡献
		},
		{
			name:     "仅上邻居",
			flags:    T1_SIG_N,
			expected: 1, // 垂直贡献
		},
		{
			name:     "左右邻居",
			flags:    T1_SIG_W | T1_SIG_E,
			expected: 3, // h=2, v=0, d=0 → sum=4
		},
		{
			name:     "上下邻居",
			flags:    T1_SIG_N | T1_SIG_S,
			expected: 3, // h=0, v=2, d=0 → sum=4
		},
		{
			name:     "四周邻居",
			flags:    T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E,
			expected: 7, // h=2, v=2, d=0 → sum=8
		},
		{
			name:     "所有8个邻居",
			flags:    T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E | T1_SIG_NW | T1_SIG_NE | T1_SIG_SW | T1_SIG_SE,
			expected: 8, // h=2, v=2, d=4 → sum=12 → context 8
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getZeroCodingContext(tc.flags)
			if actual != tc.expected {
				t.Errorf("期望上下文 %d, 得到 %d", tc.expected, actual)
			}
		})
	}
}

// TestMagnitudeRefinementContext 验证幅度精化上下文计算
func TestMagnitudeRefinementContext(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{
			name:     "无邻居显著",
			flags:    0,
			expected: 14, // 上下文 14 (CTX_MR_START + 0)
		},
		{
			name:     "1个邻居",
			flags:    T1_SIG_W,
			expected: 15, // 上下文 15 (CTX_MR_START + 1)
		},
		{
			name:     "2个邻居",
			flags:    T1_SIG_W | T1_SIG_E,
			expected: 15, // 上下文 15 (CTX_MR_START + 1)
		},
		{
			name:     "3个邻居",
			flags:    T1_SIG_W | T1_SIG_E | T1_SIG_N,
			expected: 16, // 上下文 16 (CTX_MR_START + 2)
		},
		{
			name:     "4个邻居",
			flags:    T1_SIG_W | T1_SIG_E | T1_SIG_N | T1_SIG_S,
			expected: 16, // 上下文 16 (CTX_MR_START + 2)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getMagRefinementContext(tc.flags)
			if actual != tc.expected {
				t.Errorf("期望上下文 %d, 得到 %d", tc.expected, actual)
			}
		})
	}
}

// TestSignCodingContextExtraction 验证符号编码上下文提取
func TestSignCodingContextExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{
			name:     "全正邻居 h=2,v=2",
			flags:    T1_SIG_E | T1_SIG_W | T1_SIG_N | T1_SIG_S | T1_SIGN_E | T1_SIGN_W | T1_SIGN_N | T1_SIGN_S,
			expected: 9, // h=2, v=2 → context 0 (absolute 9)
		},
		{
			name:     "东负西负 h=-2",
			flags:    T1_SIG_E | T1_SIG_W,
			expected: 12, // h=-2, v=0 → context 3 (absolute 12)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getSignCodingContext(tc.flags)
			if actual != tc.expected {
				t.Errorf("期望上下文 %d, 得到 %d", tc.expected, actual)
			}
		})
	}
}

// TestContextConstants 验证上下文常量定义
func TestContextConstants(t *testing.T) {
	// 验证上下文范围定义
	tests := []struct {
		name     string
		start    int
		end      int
		count    int
		constant string
	}{
		{"Zero Coding", CTX_ZC_START, CTX_ZC_END, 9, "CTX_ZC"},
		{"Sign Coding", CTX_SC_START, CTX_SC_END, 5, "CTX_SC"},
		{"Magnitude Refinement", CTX_MR_START, CTX_MR_END, 3, "CTX_MR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualCount := tt.end - tt.start + 1
			if actualCount != tt.count {
				t.Errorf("%s: 期望 %d 个上下文, 得到 %d", tt.constant, tt.count, actualCount)
			}
		})
	}

	// 验证总上下文数
	if NUM_CONTEXTS != 19 {
		t.Errorf("NUM_CONTEXTS 应为 19, 得到 %d", NUM_CONTEXTS)
	}

	// 验证 Run-Length 和 Uniform 上下文
	if CTX_RL != 17 {
		t.Errorf("CTX_RL 应为 17, 得到 %d", CTX_RL)
	}
	if CTX_UNI != 18 {
		t.Errorf("CTX_UNI 应为 18, 得到 %d", CTX_UNI)
	}
}

// TestStateFlagDefinitions 验证状态标志定义
func TestStateFlagDefinitions(t *testing.T) {
	// 验证基本标志
	if T1_SIG != 0x0001 {
		t.Errorf("T1_SIG 应为 0x0001")
	}
	if T1_REFINE != 0x0002 {
		t.Errorf("T1_REFINE 应为 0x0002")
	}
	if T1_VISIT != 0x0004 {
		t.Errorf("T1_VISIT 应为 0x0004")
	}

	// 验证邻居显著性标志
	expectedNeighbors := []struct {
		flag uint32
		name string
	}{
		{T1_SIG_N, "T1_SIG_N"},
		{T1_SIG_S, "T1_SIG_S"},
		{T1_SIG_W, "T1_SIG_W"},
		{T1_SIG_E, "T1_SIG_E"},
		{T1_SIG_NW, "T1_SIG_NW"},
		{T1_SIG_NE, "T1_SIG_NE"},
		{T1_SIG_SW, "T1_SIG_SW"},
		{T1_SIG_SE, "T1_SIG_SE"},
	}

	allNeighbors := uint32(0)
	for _, n := range expectedNeighbors {
		if n.flag == 0 {
			t.Errorf("%s 不应为 0", n.name)
		}
		// 检查标志是否唯一（单个比特）
		if n.flag&(n.flag-1) != 0 {
			t.Errorf("%s 应该是单个比特: 0x%x", n.name, n.flag)
		}
		allNeighbors |= n.flag
	}

	// 验证邻居掩码
	if T1_SIG_NEIGHBORS != allNeighbors {
		t.Errorf("T1_SIG_NEIGHBORS 不匹配: 期望 0x%x, 得到 0x%x", allNeighbors, T1_SIG_NEIGHBORS)
	}
}

// BenchmarkSignContextLUT 基准测试符号上下文查找
func BenchmarkSignContextLUT(b *testing.B) {
	flags := uint32(T1_SIG_E | T1_SIG_W | T1_SIGN_E | T1_SIGN_W)
	for i := 0; i < b.N; i++ {
		_ = getSignCodingContext(flags)
	}
}

// BenchmarkZeroCodingContext 基准测试零编码上下文计算
func BenchmarkZeroCodingContext(b *testing.B) {
	flags := uint32(T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E)
	for i := 0; i < b.N; i++ {
		_ = getZeroCodingContext(flags)
	}
}

// BenchmarkMagRefinementContext 基准测试幅度精化上下文
func BenchmarkMagRefinementContext(b *testing.B) {
	flags := uint32(T1_SIG_N | T1_SIG_S | T1_SIG_W)
	for i := 0; i < b.N; i++ {
		_ = getMagRefinementContext(flags)
	}
}
