package htj2k

// MEL (Magnitude Exponent Likelihood) encoder/decoder.
// 完整实现 13 状态自适应游程编码，与 OpenJPH 的 mel_encode/mel_decode 对齐。
// 参考：ISO/IEC 15444-15:2019 Clause 7.3.3 & OpenJPH ojph_block_encoder.cpp。

var melExp = [...]int{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 4, 5}

// MELEncoder implements the MELCODE encoder for HTJ2K block coding.
// 字节填充规则：若输出字节为 0xFF，则下一字节最多写 7 bit。
type MELEncoder struct {
	buf           []byte
	tmp           uint8 // 累积的待输出比特（MSB 未必满）
	remainingBits int   // 当前 tmp 中可再写入的空位数
	run           int
	k             int
	threshold     int
}

// NewMELEncoder creates a new MEL encoder.
func NewMELEncoder() *MELEncoder {
	return &MELEncoder{
		buf:           make([]byte, 0, 192), // MEL 最多约 1bit/quad，预分配足够
		remainingBits: 8,
		threshold:     1,
	}
}

// EncodeBit encodes one MEL symbol (0 = 继续游程, 1 = 终止游程并输出 run).
func (m *MELEncoder) EncodeBit(bit int) {
	if bit == 0 {
		m.run++
		if m.run >= m.threshold {
			m.emitBit(1)
			m.run = 0
			if m.k < 12 {
				m.k++
			}
			m.threshold = 1 << melExp[m.k]
		}
		return
	}

	// bit == 1：输出 0 + run 的二进制（长度 melExp[k]）
	m.emitBit(0)
	t := melExp[m.k]
	for t > 0 {
		t--
		m.emitBit(uint8((m.run >> t) & 1))
	}
	m.run = 0
	if m.k > 0 {
		m.k--
	}
	m.threshold = 1 << melExp[m.k]
}

// emitBit writes a single bit to the output buffer with byte stuffing.
func (m *MELEncoder) emitBit(b uint8) {
	m.tmp = (m.tmp << 1) | (b & 1)
	m.remainingBits--
	if m.remainingBits == 0 {
		m.buf = append(m.buf, m.tmp)
		// 如果输出了 0xFF，下个字节只能写 7 位
		if m.tmp == 0xFF {
			m.remainingBits = 7
		} else {
			m.remainingBits = 8
		}
		m.tmp = 0
	}
}

// Flush finalizes the MEL stream and returns the encoded bytes.
// 若还有未写出的 run，则强制写出终止符（对应 mel_encode 中 run>0 时的收尾）。
func (m *MELEncoder) Flush() []byte {
	if m.run > 0 {
		m.EncodeBit(1)
	}
	if m.remainingBits != 8 {
		// 左移补齐剩余位
		m.tmp <<= m.remainingBits
		m.buf = append(m.buf, m.tmp)
		m.tmp = 0
		m.remainingBits = 8
	}
	return m.buf
}

// GetBytes returns current encoded bytes (without final padding).
func (m *MELEncoder) GetBytes() []byte {
	return m.buf
}

// Length returns number of bytes already emitted (without padding).
func (m *MELEncoder) Length() int {
	return len(m.buf)
}

// MELDecoder mirrors OpenJPH dec_mel_st/mel_decode 逻辑的精简版本。
// 直接按符号顺序解码（逐个 MEL 事件，而非批量 run 队列）。
type MELDecoder struct {
	data []byte
	pos  int

	// bit read state with un-stuffing (last 0xFF => next byte 7 bits)
	lastByte uint8
	bits     uint8 // cached bits in buffer
	buf      uint8 // buffer storing bits MSB-first

	// MEL state
	k         int
	threshold int

	// Pending symbols expanded from last decoded run
	pendingZeros int
	pendingOne   bool
}

// NewMELDecoder creates a new MEL decoder.
func NewMELDecoder(data []byte) *MELDecoder {
	return &MELDecoder{
		data:      data,
		lastByte:  0x00,
		threshold: 1,
	}
}

// readBit reads one bit applying un-stuffing rule.
func (m *MELDecoder) readBit() (uint8, bool) {
	if m.bits == 0 {
		if m.pos >= len(m.data) {
			return 0, false
		}
		b := m.data[m.pos]
		m.pos++

		if m.lastByte == 0xFF {
			// 下一个字节只取低 7 位
			m.buf = b & 0x7F
			m.bits = 7
		} else {
			m.buf = b
			m.bits = 8
		}
		m.lastByte = b
	}

	bit := (m.buf >> 7) & 1
	m.buf <<= 1
	m.bits--
	return bit, true
}

// DecodeBit decodes one MEL symbol (0 或 1)。第二返回值表示是否还有数据。
func (m *MELDecoder) DecodeBit() (int, bool) {
	// 如果有展开的待输出符号，优先返回
	if m.pendingZeros > 0 {
		m.pendingZeros--
		return 0, true
	}
	if m.pendingOne {
		m.pendingOne = false
		return 1, true
	}

	// 解码一个 run（参照 OpenJPH mel_decode）
	lead, ok := m.readBit()
	if !ok {
		return 0, false
	}

	eval := melExp[m.k]
	if lead == 1 {
		// run 溢出，无终止 1：zeros = threshold
		m.pendingZeros = 1 << eval
		if m.k < 12 {
			m.k++
		}
	} else {
		// 读取 eval 位作为 run 值，末尾有一个终止 1
		runVal := 0
		if eval > 0 {
			for i := 0; i < eval; i++ {
				b, ok := m.readBit()
				if !ok {
					return 0, false
				}
				runVal = (runVal << 1) | int(b)
			}
		}
		m.pendingZeros = runVal
		m.pendingOne = true
		if m.k > 0 {
			m.k--
		}
	}
	m.threshold = 1 << melExp[m.k]

	return m.DecodeBit()
}
