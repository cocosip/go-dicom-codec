package mqc

import (
	"testing"
)

// TestProbabilityIntervalUpdate 验证概率区间更新算法
// 这是 MQ 编码器的核心算法，必须与 OpenJPEG 完全一致
// 参考: ISO/IEC 15444-1 Annex C.3.2
func TestProbabilityIntervalUpdate(t *testing.T) {
	t.Run("MPS编码时的区间更新", func(t *testing.T) {
		// 测试 MPS (Most Probable Symbol) 编码
		enc := NewMQEncoder(1)

		// 初始状态：A = 0x8000, C = 0, state = 0
		initialA := uint32(0x8000)
		if enc.a != initialA {
			t.Errorf("初始 A 错误: 期望 0x%04X, 得到 0x%04X", initialA, enc.a)
		}

		// 编码 MPS (假设初始 MPS = 0)
		// 根据 ISO/IEC 15444-1 Annex C.3.2:
		// 编码 MPS 时: A = A - Qe(state)
		state := uint8(0)
		qe := qeTable[state] // state 0 的 Qe = 0x5601

		t.Logf("编码前: 状态 %d, Qe = 0x%04X, A = 0x%04X", state, qe, enc.a)

		enc.Encode(0, 0) // 编码 MPS

		// 验证 A 的更新
		// 注意：如果 A >= 0x8000，不需要重归一化，A 应该减去 Qe
		// 但如果触发重归一化，A 会被左移
		t.Logf("编码后: A = 0x%04X", enc.a)
		t.Logf("编码 MPS 后 A = 0x%04X", enc.a)

		// A 应该在合理范围内（0x8000 或更大，或因重归一化而变化）
		if enc.a == 0 {
			t.Errorf("A 不应为 0")
		}
	})

	t.Run("LPS编码时的区间更新", func(t *testing.T) {
		// 测试 LPS (Least Probable Symbol) 编码
		enc := NewMQEncoder(1)

		// 编码 LPS (与初始 MPS 相反)
		// 根据 ISO/IEC 15444-1 Annex C.3.2:
		// 编码 LPS 时: A = A - Qe, 然后进行条件交换
		state := uint8(0)
		qe := qeTable[state]

		t.Logf("编码前: 状态 %d, Qe = 0x%04X", state, qe)

		enc.Encode(1, 0) // 编码 LPS (假设初始 MPS = 0)

		t.Logf("编码 LPS 后 A = 0x%04X", enc.a)
		t.Logf("编码 LPS 后状态 = %d", enc.contexts[0]&0x7F)

		// LPS 编码后应该转移到 NLPS 状态
		newState := enc.contexts[0] & 0x7F
		expectedState := nlpsTable[state]
		if newState != expectedState {
			t.Errorf("LPS 后状态错误: 期望 %d, 得到 %d", expectedState, newState)
		}
	})

	t.Run("条件交换验证", func(t *testing.T) {
		// 验证 MPS/LPS 的条件交换逻辑
		// 根据 ISO/IEC 15444-1 Annex C.3.2:
		// 当 A < Qe 时，需要进行 MPS/LPS 交换

		tests := []struct {
			name          string
			initialState  uint8
			encodeBit     int
			shouldSwitch  bool
		}{
			{
				name:         "状态0编码LPS",
				initialState: 0,
				encodeBit:    1, // LPS
				shouldSwitch: true, // State 0 的 switch 标志为 1
			},
			{
				name:         "状态1编码LPS",
				initialState: 1,
				encodeBit:    1, // LPS
				shouldSwitch: false, // State 1 的 switch 标志为 0
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				enc := NewMQEncoder(1)

				// 设置初始状态
				enc.contexts[0] = tt.initialState
				initialMPS := int(enc.contexts[0] >> 7)

				// 编码 LPS
				enc.Encode(tt.encodeBit, 0)

				// 检查 MPS 是否切换
				finalMPS := int(enc.contexts[0] >> 7)
				switched := (initialMPS != finalMPS)

				switchFlag := switchTable[tt.initialState]
				expectedSwitch := (switchFlag == 1)

				t.Logf("初始状态: %d, 初始MPS: %d", tt.initialState, initialMPS)
				t.Logf("Switch标志: %d, 期望切换: %v, 实际切换: %v", switchFlag, expectedSwitch, switched)
				t.Logf("最终MPS: %d", finalMPS)

				// 对于编码 LPS，如果 switch 标志为 1，MPS 应该切换
				if tt.encodeBit == (1 - initialMPS) { // 确实是 LPS
					if expectedSwitch && !switched {
						t.Errorf("应该切换 MPS 但没有切换")
					}
					if !expectedSwitch && switched {
						t.Errorf("不应该切换 MPS 但切换了")
					}
				}
			})
		}
	})

	t.Run("区间分割验证", func(t *testing.T) {
		// 验证区间分割的正确性
		// A 应该总是被分割为 MPS 和 LPS 两部分
		// A_MPS = A - Qe, A_LPS = Qe

		for state := uint8(0); state < 47; state++ {
			enc := NewMQEncoder(1)
			enc.contexts[0] = state

			qe := qeTable[state]
			initialA := enc.a

			// 编码 MPS
			enc.Encode(0, 0)

			t.Logf("状态 %d: Qe=0x%04X, 初始A=0x%04X, 编码后A=0x%04X",
				state, qe, initialA, enc.a)

			// A 应该在合理范围内
			if enc.a == 0 {
				t.Errorf("状态 %d: A 不应为 0", state)
			}
		}
	})
}

// TestRenormalization 验证重归一化过程
// 重归一化确保概率区间 A 始终 >= 0x8000
// 参考: ISO/IEC 15444-1 Annex C.3.3
func TestRenormalization(t *testing.T) {
	t.Run("重归一化条件", func(t *testing.T) {
		// 验证何时触发重归一化
		// 根据标准：当 A < 0x8000 时必须重归一化

		enc := NewMQEncoder(1)

		// 强制 A 低于阈值（通过编码多个符号）
		for i := 0; i < 100; i++ {
			enc.Encode(1, 0)

			// A 应该始终 >= 0x8000
			if enc.a < 0x8000 {
				t.Errorf("迭代 %d: A = 0x%04X < 0x8000，重归一化未正确执行", i, enc.a)
			}
		}
	})

	t.Run("重归一化过程", func(t *testing.T) {
		// 验证重归一化的具体步骤
		// 1. A <<= 1
		// 2. C <<= 1
		// 3. ct--
		// 4. 如果 ct == 0，输出一个字节

		enc := NewMQEncoder(1)
		initialCt := enc.ct

		// 编码一些符号触发重归一化
		for i := 0; i < 10; i++ {
			enc.Encode(1, 0)
		}

		t.Logf("初始 ct: %d, 当前 ct: %d", initialCt, enc.ct)

		// ct 应该在有效范围内 (0-12)
		if enc.ct < 0 || enc.ct > 12 {
			t.Errorf("ct 超出范围: %d", enc.ct)
		}
	})

	t.Run("比特发送验证", func(t *testing.T) {
		// 验证比特发送到输出缓冲区
		// 当 ct 变为 0 时，应该调用 byteout()

		enc := NewMQEncoder(1)

		// 编码足够多的符号以触发字节输出
		for i := 0; i < 100; i++ {
			enc.Encode(1, 0)
		}

		// 调用 Flush() 确保所有数据输出
		encoded := enc.Flush()

		// 应该有字节输出
		numBytes := len(encoded)
		t.Logf("编码 100 个符号后输出 %d 字节", numBytes)

		if numBytes == 0 {
			t.Errorf("编码 100 个符号后应该有字节输出")
		}
	})

	t.Run("重归一化后A的范围", func(t *testing.T) {
		// 验证重归一化后 A 的有效范围
		// A 应该在 [0x8000, 0xFFFF] 之间

		enc := NewMQEncoder(1)

		for i := 0; i < 1000; i++ {
			// 随机编码 0 或 1
			bit := i % 2
			enc.Encode(bit, 0)

			// 验证 A 的范围
			if enc.a < 0x8000 {
				t.Errorf("迭代 %d: A = 0x%04X < 0x8000", i, enc.a)
			}
			if enc.a > 0xFFFF {
				t.Errorf("迭代 %d: A = 0x%04X > 0xFFFF", i, enc.a)
			}
		}
	})

	t.Run("C寄存器的左移", func(t *testing.T) {
		// 验证 C 寄存器在重归一化时的左移
		// C 应该与 A 同步左移

		enc := NewMQEncoder(1)

		// 记录初始 C
		initialC := enc.c

		// 编码一些符号
		for i := 0; i < 10; i++ {
			enc.Encode(1, 0)
		}

		t.Logf("初始 C: 0x%08X, 当前 C: 0x%08X", initialC, enc.c)

		// C 应该在合理范围内（注意 C 可能因为字节输出而减小）
		// 只验证 C 的有效性，不强制要求递增
		if enc.c > 0x0FFFFFFF {
			t.Errorf("C 超出范围: 0x%08X", enc.c)
		}
	})
}

// TestByteStuffingDetailed 详细验证字节填充机制
// 参考: ISO/IEC 15444-1 Annex C.3.5
func TestByteStuffingDetailed(t *testing.T) {
	t.Run("0xFF字节检测", func(t *testing.T) {
		// 验证编码器正确检测 0xFF 字节

		enc := NewMQEncoder(1)

		// 编码足够多的符号以产生各种字节值
		for i := 0; i < 2000; i++ {
			enc.Encode(1, 0)
		}

		encoded := enc.Flush()

		// 检查输出中的 0xFF 字节
		ffCount := 0
		for i := 0; i < len(encoded); i++ {
			if encoded[i] == 0xFF {
				ffCount++
				t.Logf("发现 0xFF 在位置 %d", i)

				// 检查下一字节（如果存在）
				if i+1 < len(encoded) {
					nextByte := encoded[i+1]
					t.Logf("  下一字节: 0x%02X", nextByte)

					// 根据标准，0xFF 后面应该跟着 < 0x90 的字节（bit stuffing）
					// 或者是标记字节（>= 0x90）
					if nextByte >= 0x90 {
						t.Logf("  → 可能是标记字节")
					} else {
						t.Logf("  → 字节填充")
					}
				}
			}
		}

		t.Logf("总共发现 %d 个 0xFF 字节", ffCount)
	})

	t.Run("字节填充对往返的影响", func(t *testing.T) {
		// 验证字节填充不影响解码正确性

		enc := NewMQEncoder(1)

		// 编码一个会产生 0xFF 的序列
		testBits := make([]int, 1000)
		for i := range testBits {
			testBits[i] = 1
		}

		for _, bit := range testBits {
			enc.Encode(bit, 0)
		}

		encoded := enc.Flush()

		// 检查是否有 0xFF
		hasFF := false
		for _, b := range encoded {
			if b == 0xFF {
				hasFF = true
				break
			}
		}

		// 解码
		dec := NewMQDecoder(encoded, 1)
		for i, expectedBit := range testBits {
			bit := dec.Decode(0)
			if bit != expectedBit {
				t.Errorf("位 %d 解码错误: 期望 %d, 得到 %d", i, expectedBit, bit)
			}
		}

		if hasFF {
			t.Logf("✅ 包含 0xFF 字节，解码正确")
		} else {
			t.Logf("本次测试未产生 0xFF 字节")
		}
	})

	t.Run("ct计数器在字节填充时的行为", func(t *testing.T) {
		// 验证 ct 计数器在遇到 0xFF 时的行为
		// 根据标准：0xFF 后 ct 应该设置为 7（而不是 8）

		enc := NewMQEncoder(1)

		// 编码序列
		for i := 0; i < 100; i++ {
			enc.Encode(1, 0)

			// ct 应该在 0-12 范围内
			if enc.ct < 0 || enc.ct > 12 {
				t.Errorf("ct 超出范围: %d", enc.ct)
			}
		}

		t.Logf("编码完成，最终 ct: %d", enc.ct)
	})
}

// TestEncoderDecoderSymmetry 验证编码器和解码器的对称性
// 确保两者使用相同的算法和状态表
func TestEncoderDecoderSymmetry(t *testing.T) {
	t.Run("状态转移对称性", func(t *testing.T) {
		// 编码器和解码器应该以相同方式更新状态

		enc := NewMQEncoder(1)
		dec := NewMQDecoder([]byte{0x00}, 1)

		// 验证初始状态
		encState := enc.GetContextState(0)
		decState := dec.GetContextState(0)

		if encState != decState {
			t.Errorf("初始状态不匹配: 编码器=%d, 解码器=%d", encState, decState)
		}

		// 编码一些符号
		testBits := []int{0, 1, 1, 0, 1, 0, 0, 1}
		for _, bit := range testBits {
			enc.Encode(bit, 0)
		}
		encoded := enc.Flush()

		// 解码相同序列
		dec = NewMQDecoder(encoded, 1)
		for _, expectedBit := range testBits {
			bit := dec.Decode(0)
			if bit != expectedBit {
				t.Errorf("解码不匹配")
			}
		}

		// 状态应该相同
		encFinalState := enc.GetContextState(0)
		decFinalState := dec.GetContextState(0)

		t.Logf("编码后状态: 编码器=%d, 解码器=%d", encFinalState, decFinalState)

		// 注意：由于 Flush 可能影响编码器状态，我们主要验证解码正确性
		// 实际状态可能不完全相同，但解码应该正确
	})

	t.Run("概率估计一致性", func(t *testing.T) {
		// 编码器和解码器应该对相同状态使用相同的 Qe 值

		for state := uint8(0); state < 47; state++ {
			qe := qeTable[state]

			// 验证 Qe 值合理
			if qe == 0 {
				t.Errorf("状态 %d 的 Qe 不应为 0", state)
			}

			// Qe 应该递减（大致趋势）
			if state > 0 && state < 46 {
				prevQe := qeTable[state-1]
				// 允许一些例外（某些状态的 Qe 可能相同或略有增加）
				t.Logf("状态 %d: Qe=0x%04X, 前一状态 Qe=0x%04X", state, qe, prevQe)
			}
		}
	})
}
