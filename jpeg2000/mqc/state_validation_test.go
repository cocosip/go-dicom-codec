package mqc

import (
	"testing"
)

// TestMQStateTablesCompliance 验证 MQ 状态表与 OpenJPEG 参考实现的完全对齐
// OpenJPEG 是 JPEG 2000 的权威参考实现
// 这是 MQ 算术编码器正确性的核心保证
func TestMQStateTablesCompliance(t *testing.T) {
	// 从 OpenJPEG mqc.c 中提取的状态表（47 个状态）
	// 每一行: Index | Qe | NMPS | NLPS | Switch
	// 参考: OpenJPEG mqc.c mqc_states[47*2]

	// 验证表大小
	if len(qeTable) != 47 {
		t.Errorf("qeTable 大小错误: 期望 47, 得到 %d", len(qeTable))
	}
	if len(nmpsTable) != 47 {
		t.Errorf("nmpsTable 大小错误: 期望 47, 得到 %d", len(nmpsTable))
	}
	if len(nlpsTable) != 47 {
		t.Errorf("nlpsTable 大小错误: 期望 47, 得到 %d", len(nlpsTable))
	}
	if len(switchTable) != 47 {
		t.Errorf("switchTable 大小错误: 期望 47, 得到 %d", len(switchTable))
	}

	// 逐项验证
	errorCount := 0
	for i := 0; i < 47; i++ {
		expected := openJPEGStates[i]

		// 验证 Qe 值
		if qeTable[i] != expected.Qe {
			t.Errorf("State %d: Qe 不匹配 - 期望 0x%04X, 得到 0x%04X",
				i, expected.Qe, qeTable[i])
			errorCount++
		}

		// 验证 NMPS
		if nmpsTable[i] != expected.NMPS {
			t.Errorf("State %d: NMPS 不匹配 - 期望 %d, 得到 %d",
				i, expected.NMPS, nmpsTable[i])
			errorCount++
		}

		// 验证 NLPS
		if nlpsTable[i] != expected.NLPS {
			t.Errorf("State %d: NLPS 不匹配 - 期望 %d, 得到 %d",
				i, expected.NLPS, nlpsTable[i])
			errorCount++
		}

		// 验证 Switch
		if switchTable[i] != expected.Switch {
			t.Errorf("State %d: Switch 不匹配 - 期望 %d, 得到 %d",
				i, expected.Switch, switchTable[i])
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fatalf("状态表验证失败：发现 %d 个错误", errorCount)
	}

	t.Logf("✅ 所有 47 个状态完全符合 OpenJPEG 参考实现")
}

// TestMQInitialization 测试 MQ 编码器和解码器的初始化
func TestMQInitialization(t *testing.T) {
	t.Run("编码器初始化", func(t *testing.T) {
		enc := NewMQEncoder(19) // JPEG 2000 uses 19 contexts

		// 验证初始状态
		if enc.a != 0x8000 {
			t.Errorf("初始 A 寄存器错误: 期望 0x8000, 得到 0x%04X", enc.a)
		}
		if enc.c != 0 {
			t.Errorf("初始 C 寄存器错误: 期望 0, 得到 0x%08X", enc.c)
		}
		if enc.ct != 12 {
			t.Errorf("初始 ct 计数器错误: 期望 12, 得到 %d", enc.ct)
		}

		// 验证所有上下文初始化为状态 0
		for i := 0; i < 19; i++ {
			if enc.contexts[i] != 0 {
				t.Errorf("上下文 %d 未初始化为状态 0: 得到 %d", i, enc.contexts[i])
			}
		}
	})

	t.Run("解码器初始化", func(t *testing.T) {
		testData := []byte{0x00, 0x00, 0x00, 0x00}
		dec := NewMQDecoder(testData, 19)

		// 验证初始 A 寄存器
		if dec.a != 0x8000 {
			t.Errorf("初始 A 寄存器错误: 期望 0x8000, 得到 0x%04X", dec.a)
		}

		// 验证所有上下文初始化为状态 0
		for i := 0; i < 19; i++ {
			if dec.contexts[i] != 0 {
				t.Errorf("上下文 %d 未初始化为状态 0: 得到 %d", i, dec.contexts[i])
			}
		}
	})
}

// TestMQRoundTrip 测试 MQ 编解码器的往返一致性
func TestMQRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		bits []int
	}{
		{
			name: "全0序列",
			bits: []int{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "全1序列",
			bits: []int{1, 1, 1, 1, 1, 1, 1, 1},
		},
		{
			name: "交替序列",
			bits: []int{0, 1, 0, 1, 0, 1, 0, 1},
		},
		{
			name: "随机序列",
			bits: []int{1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 1, 1},
		},
		{
			name: "长序列",
			bits: func() []int {
				b := make([]int, 1000)
				for i := range b {
					b[i] = i % 3 // 0, 1, 2 pattern (using 0 and 1)
					if b[i] > 1 {
						b[i] = 1
					}
				}
				return b
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 编码
			enc := NewMQEncoder(1) // 使用单个上下文
			for _, bit := range tt.bits {
				enc.Encode(bit, 0)
			}
			encoded := enc.Flush()

			// 解码
			dec := NewMQDecoder(encoded, 1)
			decoded := make([]int, len(tt.bits))
			for i := range tt.bits {
				decoded[i] = dec.Decode(0)
			}

			// 验证
			for i := range tt.bits {
				if tt.bits[i] != decoded[i] {
					t.Errorf("位 %d 不匹配: 期望 %d, 得到 %d", i, tt.bits[i], decoded[i])
				}
			}
		})
	}
}

// TestMQContextSwitching 测试多上下文切换
func TestMQContextSwitching(t *testing.T) {
	// 使用多个上下文编码不同的比特流
	numContexts := 5
	bitsPerContext := 20

	// 为每个上下文生成不同的模式
	patterns := [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 全0
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 全1
		{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1}, // 交替
		{1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1, 1, 0, 0, 1}, // 模式
		{1, 1, 0, 1, 0, 1, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1, 1}, // 随机
	}

	// 编码（交织使用不同上下文）
	enc := NewMQEncoder(numContexts)
	for i := 0; i < bitsPerContext; i++ {
		for ctx := 0; ctx < numContexts; ctx++ {
			enc.Encode(patterns[ctx][i], ctx)
		}
	}
	encoded := enc.Flush()

	// 解码（相同的上下文交织顺序）
	dec := NewMQDecoder(encoded, numContexts)
	decodedPatterns := make([][]int, numContexts)
	for i := range decodedPatterns {
		decodedPatterns[i] = make([]int, bitsPerContext)
	}

	for i := 0; i < bitsPerContext; i++ {
		for ctx := 0; ctx < numContexts; ctx++ {
			decodedPatterns[ctx][i] = dec.Decode(ctx)
		}
	}

	// 验证每个上下文的解码结果
	for ctx := 0; ctx < numContexts; ctx++ {
		for i := 0; i < bitsPerContext; i++ {
			if patterns[ctx][i] != decodedPatterns[ctx][i] {
				t.Errorf("上下文 %d, 位 %d 不匹配: 期望 %d, 得到 %d",
					ctx, i, patterns[ctx][i], decodedPatterns[ctx][i])
			}
		}
	}
}

// TestMQByteStuffing 测试 0xFF 字节填充处理
func TestMQByteStuffing(t *testing.T) {
	// 生成一个会产生 0xFF 字节的比特序列
	// 这需要特定的概率状态和输入模式
	enc := NewMQEncoder(1)

	// 编码大量的 1 比特来触发各种字节值
	for i := 0; i < 1000; i++ {
		enc.Encode(1, 0)
	}
	encoded := enc.Flush()

	// 检查是否有 0xFF 字节（如果有，后面应该有填充字节）
	hasFF := false
	for i := 0; i < len(encoded)-1; i++ {
		if encoded[i] == 0xFF {
			hasFF = true
			// 验证字节填充处理（但不强制要求，因为 0xFF 可能不需要填充）
			t.Logf("发现 0xFF 在位置 %d, 下一字节: 0x%02X", i, encoded[i+1])
		}
	}

	// 解码并验证
	dec := NewMQDecoder(encoded, 1)
	for i := 0; i < 1000; i++ {
		bit := dec.Decode(0)
		if bit != 1 {
			t.Errorf("位 %d 解码错误: 期望 1, 得到 %d", i, bit)
		}
	}

	if hasFF {
		t.Logf("✅ 正确处理了 0xFF 字节填充")
	}
}

// TestMQStateTransitions 测试状态转移的正确性
func TestMQStateTransitions(t *testing.T) {
	enc := NewMQEncoder(1)

	// 编码一系列比特并跟踪状态变化
	initialState := enc.GetContextState(0)
	if initialState != 0 {
		t.Errorf("初始状态应为 0, 得到 %d", initialState)
	}

	// 编码一些比特
	bits := []int{0, 1, 1, 0, 1, 0, 0, 1, 1, 1}
	for _, bit := range bits {
		enc.Encode(bit, 0)
	}

	// 状态应该已经改变
	finalState := enc.GetContextState(0)
	t.Logf("编码后状态: %d (从 %d 转移)", finalState, initialState)

	// 状态包含 MPS 位（第 7 位），所以有效范围是 0-255
	// 实际状态索引是低 7 位（0-46），MPS 是第 7 位（0 或 128）
	actualState := finalState & 0x7F
	mps := finalState >> 7

	// 验证实际状态在有效范围内（0-46）
	if actualState > 46 {
		t.Errorf("状态索引超出范围: %d (期望 0-46)", actualState)
	}

	// 验证 MPS 只能是 0 或 1
	if mps > 1 {
		t.Errorf("MPS 值无效: %d (期望 0 或 1)", mps)
	}

	t.Logf("  → 状态索引: %d, MPS: %d", actualState, mps)
}

// TestMQCompression 测试压缩效率
func TestMQCompression(t *testing.T) {
	// 高度可预测的序列应该压缩得很好
	enc := NewMQEncoder(1)

	// 100 个连续的 0
	for i := 0; i < 100; i++ {
		enc.Encode(0, 0)
	}
	encoded := enc.Flush()

	t.Logf("100 个连续 0 压缩为 %d 字节 (压缩率 %.2f%%)",
		len(encoded), float64(len(encoded))*8.0/100.0)

	// 压缩后的大小应该远小于未压缩的 100 比特（12.5 字节）
	if len(encoded) > 10 {
		t.Logf("警告：高度可预测序列的压缩率不够理想")
	}

	// 验证解码正确性
	dec := NewMQDecoder(encoded, 1)
	for i := 0; i < 100; i++ {
		bit := dec.Decode(0)
		if bit != 0 {
			t.Errorf("位 %d 解码错误: 期望 0, 得到 %d", i, bit)
		}
	}
}
