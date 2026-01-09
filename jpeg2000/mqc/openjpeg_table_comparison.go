package mqc

// OpenJPEG 状态表解析
// 从 OpenJPEG mqc.c 中提取的状态表
// 格式: {qeval, mps, nmps_index, nlps_index}

// 解析 OpenJPEG 的状态表
// OpenJPEG 使用 47*2=94 个状态（每个状态有 MPS=0 和 MPS=1 两个版本）
// 我们需要将其转换为 47 个独立状态，每个状态有 NMPS、NLPS、Switch 信息

type OpenJPEGState struct {
	Qe     uint32
	NMPS   uint8
	NLPS   uint8
	Switch uint8
}

// 从 OpenJPEG 的 mqc_states 数组提取的状态表
// 每一项对应一个基础状态（不考虑 MPS 值）
var openJPEGStates = [47]OpenJPEGState{
	// State 0: mqc_states[0] and mqc_states[1]
	// qe=0x5601, nmps->states[2] (state 1), nlps->states[3] (state 1 with opposite MPS)
	// Switch = 1 (因为 NLPS 指向相反的 MPS)
	{0x5601, 1, 1, 1},

	// State 1: mqc_states[2] and mqc_states[3]
	// qe=0x3401, nmps->states[4] (state 2), nlps->states[12] (state 6)
	{0x3401, 2, 6, 0},

	// State 2: mqc_states[4] and mqc_states[5]
	// qe=0x1801, nmps->states[6] (state 3), nlps->states[18] (state 9)
	{0x1801, 3, 9, 0},

	// State 3: mqc_states[6] and mqc_states[7]
	// qe=0x0ac1, nmps->states[8] (state 4), nlps->states[24] (state 12)
	{0x0ac1, 4, 12, 0},

	// State 4: mqc_states[8] and mqc_states[9]
	// qe=0x0521, nmps->states[10] (state 5), nlps->states[58] (state 29)
	{0x0521, 5, 29, 0},

	// State 5: mqc_states[10] and mqc_states[11]
	// qe=0x0221, nmps->states[76] (state 38), nlps->states[66] (state 33)
	{0x0221, 38, 33, 0},

	// State 6: mqc_states[12] and mqc_states[13]
	// qe=0x5601, nmps->states[14]/[15] (state 7), nlps->states[13]/[12] (state 6 with opposite)
	{0x5601, 7, 6, 1},

	// State 7: mqc_states[14] and mqc_states[15]
	// qe=0x5401, nmps->states[16]/[17] (state 8), nlps->states[28]/[29] (state 14)
	{0x5401, 8, 14, 0},

	// State 8: mqc_states[16] and mqc_states[17]
	// qe=0x4801, nmps->states[18]/[19] (state 9), nlps->states[28]/[29] (state 14)
	{0x4801, 9, 14, 0},

	// State 9: mqc_states[18] and mqc_states[19]
	// qe=0x3801, nmps->states[20]/[21] (state 10), nlps->states[28]/[29] (state 14)
	{0x3801, 10, 14, 0},

	// State 10: mqc_states[20] and mqc_states[21]
	// qe=0x3001, nmps->states[22]/[23] (state 11), nlps->states[34]/[35] (state 17)
	{0x3001, 11, 17, 0},

	// State 11: mqc_states[22] and mqc_states[23]
	// qe=0x2401, nmps->states[24]/[25] (state 12), nlps->states[36]/[37] (state 18)
	{0x2401, 12, 18, 0},

	// State 12: mqc_states[24] and mqc_states[25]
	// qe=0x1c01, nmps->states[26]/[27] (state 13), nlps->states[40]/[41] (state 20)
	{0x1c01, 13, 20, 0},

	// State 13: mqc_states[26] and mqc_states[27]
	// qe=0x1601, nmps->states[58]/[59] (state 29), nlps->states[42]/[43] (state 21)
	{0x1601, 29, 21, 0},

	// State 14: mqc_states[28] and mqc_states[29]
	// qe=0x5601, nmps->states[30]/[31], nlps->states[29]/[28] (opposite)
	{0x5601, 15, 14, 1},

	// State 15: mqc_states[30] and mqc_states[31]
	// qe=0x5401, nmps->states[32]/[33] (state 16), nlps->states[28]/[29] (state 14)
	{0x5401, 16, 14, 0},

	// State 16: mqc_states[32] and mqc_states[33]
	// qe=0x5101, nmps->states[34]/[35] (state 17), nlps->states[30]/[31] (state 15)
	{0x5101, 17, 15, 0},

	// State 17: mqc_states[34] and mqc_states[35]
	// qe=0x4801, nmps->states[36]/[37] (state 18), nlps->states[32]/[33] (state 16)
	{0x4801, 18, 16, 0},

	// State 18: mqc_states[36] and mqc_states[37]
	// qe=0x3801, nmps->states[38]/[39] (state 19), nlps->states[34]/[35] (state 17)
	{0x3801, 19, 17, 0},

	// State 19: mqc_states[38] and mqc_states[39]
	// qe=0x3401, nmps->states[40]/[41] (state 20), nlps->states[36]/[37] (state 18)
	{0x3401, 20, 18, 0},

	// State 20: mqc_states[40] and mqc_states[41]
	// qe=0x3001, nmps->states[42]/[43] (state 21), nlps->states[38]/[39] (state 19)
	{0x3001, 21, 19, 0},

	// State 21: mqc_states[42] and mqc_states[43]
	// qe=0x2801, nmps->states[44]/[45] (state 22), nlps->states[38]/[39] (state 19)
	{0x2801, 22, 19, 0},

	// State 22: mqc_states[44] and mqc_states[45]
	// qe=0x2401, nmps->states[46]/[47] (state 23), nlps->states[40]/[41] (state 20)
	{0x2401, 23, 20, 0},

	// State 23: mqc_states[46] and mqc_states[47]
	// qe=0x2201, nmps->states[48]/[49] (state 24), nlps->states[42]/[43] (state 21)
	{0x2201, 24, 21, 0},

	// State 24: mqc_states[48] and mqc_states[49]
	// qe=0x1c01, nmps->states[50]/[51] (state 25), nlps->states[44]/[45] (state 22)
	{0x1c01, 25, 22, 0},

	// State 25: mqc_states[50] and mqc_states[51]
	// qe=0x1801, nmps->states[52]/[53] (state 26), nlps->states[46]/[47] (state 23)
	{0x1801, 26, 23, 0},

	// State 26: mqc_states[52] and mqc_states[53]
	// qe=0x1601, nmps->states[54]/[55] (state 27), nlps->states[48]/[49] (state 24)
	{0x1601, 27, 24, 0},

	// State 27: mqc_states[54] and mqc_states[55]
	// qe=0x1401, nmps->states[56]/[57] (state 28), nlps->states[50]/[51] (state 25)
	{0x1401, 28, 25, 0},

	// State 28: mqc_states[56] and mqc_states[57]
	// qe=0x1201, nmps->states[58]/[59] (state 29), nlps->states[52]/[53] (state 26)
	{0x1201, 29, 26, 0},

	// State 29: mqc_states[58] and mqc_states[59]
	// qe=0x1101, nmps->states[60]/[61] (state 30), nlps->states[54]/[55] (state 27)
	{0x1101, 30, 27, 0},

	// State 30: mqc_states[60] and mqc_states[61]
	// qe=0x0ac1, nmps->states[62]/[63] (state 31), nlps->states[56]/[57] (state 28)
	{0x0ac1, 31, 28, 0},

	// State 31: mqc_states[62] and mqc_states[63]
	// qe=0x09c1, nmps->states[64]/[65] (state 32), nlps->states[58]/[59] (state 29)
	{0x09c1, 32, 29, 0},

	// State 32: mqc_states[64] and mqc_states[65]
	// qe=0x08a1, nmps->states[66]/[67] (state 33), nlps->states[60]/[61] (state 30)
	{0x08a1, 33, 30, 0},

	// State 33: mqc_states[66] and mqc_states[67]
	// qe=0x0521, nmps->states[68]/[69] (state 34), nlps->states[62]/[63] (state 31)
	{0x0521, 34, 31, 0},

	// State 34: mqc_states[68] and mqc_states[69]
	// qe=0x0441, nmps->states[70]/[71] (state 35), nlps->states[64]/[65] (state 32)
	{0x0441, 35, 32, 0},

	// State 35: mqc_states[70] and mqc_states[71]
	// qe=0x02a1, nmps->states[72]/[73] (state 36), nlps->states[66]/[67] (state 33)
	{0x02a1, 36, 33, 0},

	// State 36: mqc_states[72] and mqc_states[73]
	// qe=0x0221, nmps->states[74]/[75] (state 37), nlps->states[68]/[69] (state 34)
	{0x0221, 37, 34, 0},

	// State 37: mqc_states[74] and mqc_states[75]
	// qe=0x0141, nmps->states[76]/[77] (state 38), nlps->states[70]/[71] (state 35)
	{0x0141, 38, 35, 0},

	// State 38: mqc_states[76] and mqc_states[77]
	// qe=0x0111, nmps->states[78]/[79] (state 39), nlps->states[72]/[73] (state 36)
	{0x0111, 39, 36, 0},

	// State 39: mqc_states[78] and mqc_states[79]
	// qe=0x0085, nmps->states[80]/[81] (state 40), nlps->states[74]/[75] (state 37)
	{0x0085, 40, 37, 0},

	// State 40: mqc_states[80] and mqc_states[81]
	// qe=0x0049, nmps->states[82]/[83] (state 41), nlps->states[76]/[77] (state 38)
	{0x0049, 41, 38, 0},

	// State 41: mqc_states[82] and mqc_states[83]
	// qe=0x0025, nmps->states[84]/[85] (state 42), nlps->states[78]/[79] (state 39)
	{0x0025, 42, 39, 0},

	// State 42: mqc_states[84] and mqc_states[85]
	// qe=0x0015, nmps->states[86]/[87] (state 43), nlps->states[80]/[81] (state 40)
	{0x0015, 43, 40, 0},

	// State 43: mqc_states[86] and mqc_states[87]
	// qe=0x0009, nmps->states[88]/[89] (state 44), nlps->states[82]/[83] (state 41)
	{0x0009, 44, 41, 0},

	// State 44: mqc_states[88] and mqc_states[89]
	// qe=0x0005, nmps->states[90]/[91] (state 45), nlps->states[84]/[85] (state 42)
	{0x0005, 45, 42, 0},

	// State 45: mqc_states[90] and mqc_states[91]
	// qe=0x0001, nmps->states[90]/[91] (state 45, stay), nlps->states[86]/[87] (state 43)
	{0x0001, 45, 43, 0},

	// State 46: mqc_states[92] and mqc_states[93]
	// qe=0x5601, nmps->states[92]/[93] (state 46, stay), nlps->states[92]/[93] (state 46, stay)
	{0x5601, 46, 46, 0},
}
