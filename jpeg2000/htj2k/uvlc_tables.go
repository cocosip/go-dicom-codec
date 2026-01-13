package htj2k

// UVLC decoding tables，按 OpenJPH ojph_block_common.cpp::uvlc_init_tables 逻辑生成。
// 这些表可用于快速解析 U-VLC 前缀/后缀长度以及初始行对的 bias。

// UVLCDecodeEntry 携带双 quad 的总前缀/后缀长度和单个前缀值等信息。
// bits layout:
//
//	[0:2]  total prefix length (lp0+lp1)
//	[3:6]  total suffix length (ls0+ls1)
//	[7:9]  suffix length for quad0
//	[10:12] prefix value for quad0
//	[13:15] prefix value for quad1
type UVLCDecodeEntry uint16

func (e UVLCDecodeEntry) TotalPrefixLen() int { return int(e & 0x7) }
func (e UVLCDecodeEntry) TotalSuffixLen() int { return int((e >> 3) & 0xF) }
func (e UVLCDecodeEntry) U0SuffixLen() int    { return int((e >> 7) & 0x7) }
func (e UVLCDecodeEntry) U0Prefix() int       { return int((e >> 10) & 0x7) }
func (e UVLCDecodeEntry) U1Prefix() int       { return int((e >> 13) & 0x7) }

var (
	UVLCTbl0 [256 + 64]UVLCDecodeEntry // 初始行对 (含 MEL 事件)
	UVLCTbl1 [256]UVLCDecodeEntry      // 非初始行
	UVLCBias [256 + 64]uint8           // 初始行对的 u_bias
)

func init() {
	generateUVLCTables()
}

func generateUVLCTables() {
	// dec 表：索引用 3bit 头（xx1/x10/100/000），值包含 lp/ls 和 prefix 值
	dec := [8]uint8{
		3 | (5 << 2) | (5 << 5), // 000 -> lp=3, ls=5, u_pfx=5
		1 | (0 << 2) | (1 << 5), // 001 -> lp=1, ls=0, u_pfx=1
		2 | (0 << 2) | (2 << 5), // 010 -> lp=2, ls=0, u_pfx=2
		1 | (0 << 2) | (1 << 5), // 011 -> lp=1, ls=0, u_pfx=1
		3 | (1 << 2) | (3 << 5), // 100 -> lp=3, ls=1, u_pfx=3
		1 | (0 << 2) | (1 << 5), // 101 -> lp=1, ls=0, u_pfx=1
		2 | (0 << 2) | (2 << 5), // 110 -> lp=2, ls=0, u_pfx=2
		1 | (0 << 2) | (1 << 5), // 111 -> lp=1, ls=0, u_pfx=1
	}

	// 初始行对：mode=0 无 u_off，1/2 单个 u_off，3 双 u_off 且 mel=0，4 双 u_off 且 mel=1
	for i := 0; i < len(UVLCTbl0); i++ {
		mode := i >> 6
		vlc := i & 0x3F
		switch mode {
		case 0:
			UVLCTbl0[i] = 0
			UVLCBias[i] = 0
		case 1, 2:
			d := dec[vlc&0x7]
			lp := int(d & 0x3)
			ls := int((d >> 2) & 0x7)
			u0suf := ls
			u0 := int(d >> 5)
			u1 := 0
			if mode == 2 {
				u0suf = 0
				u0 = 0
				u1 = int(d >> 5)
			}
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
			UVLCBias[i] = 0
		case 3:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			var lp, u0suf, ls, u0, u1 int
			if (d0 & 0x3) == 3 {
				lp = int(d0&0x3) + 1
				u0suf = int((d0 >> 2) & 0x7)
				ls = u0suf
				u0 = int(d0 >> 5)
				u1 = (vlc & 1) + 1
				UVLCBias[i] = 4
			} else {
				lp = int(d0&0x3) + int(d1&0x3)
				u0suf = int((d0 >> 2) & 0x7)
				ls = u0suf + int((d1>>2)&0x7)
				u0 = int(d0 >> 5)
				u1 = int(d1 >> 5)
				UVLCBias[i] = 0
			}
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		case 4:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			lp := int((d0 & 0x3) + (d1 & 0x3))
			u0suf := int((d0 >> 2) & 0x7)
			ls := u0suf + int((d1>>2)&0x7)
			u0 := int((d0 >> 5) + 2)
			u1 := int((d1 >> 5) + 2)
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
			UVLCBias[i] = 10
		}
	}

	// 非初始行：mode=0/1/2/3（无 MEL）
	for i := 0; i < len(UVLCTbl1); i++ {
		mode := i >> 6
		vlc := i & 0x3F
		switch mode {
		case 0:
			UVLCTbl1[i] = 0
		case 1, 2:
			d := dec[vlc&0x7]
			lp := int(d & 0x3)
			ls := int((d >> 2) & 0x7)
			u0suf := ls
			u0 := int(d >> 5)
			u1 := 0
			if mode == 2 {
				u0suf = 0
				u0 = 0
				u1 = int(d >> 5)
			}
			UVLCTbl1[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		case 3:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			lp := int((d0 & 0x3) + (d1 & 0x3))
			u0suf := int((d0 >> 2) & 0x7)
			ls := u0suf + int((d1>>2)&0x7)
			u0 := int(d0 >> 5)
			u1 := int(d1 >> 5)
			UVLCTbl1[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		}
	}
}
