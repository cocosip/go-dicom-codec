package common

// IDCT performs Inverse Discrete Cosine Transform on an 8x8 block
// Input: 64 DCT coefficients in natural order
// Output: 64 spatial domain values
func IDCT(coef []int32, out []byte, stride int) {
	// Intermediate buffer
	var tmp [64]int32

	// Constants for fast IDCT (scaled by 2048)
	const (
		w1 = 2841 // 2048*sqrt(2)*cos(1*pi/16)
		w2 = 2676 // 2048*sqrt(2)*cos(2*pi/16)
		w3 = 2408 // 2048*sqrt(2)*cos(3*pi/16)
		w5 = 1609 // 2048*sqrt(2)*cos(5*pi/16)
		w6 = 1108 // 2048*sqrt(2)*cos(6*pi/16)
		w7 = 565  // 2048*sqrt(2)*cos(7*pi/16)

		r2 = 181 // 256/sqrt(2)
	)

	// 1D IDCT on rows
	for y := 0; y < 8; y++ {
		row := y * 8

		// Check if AC coefficients are all zero (optimization)
		if coef[row+1] == 0 && coef[row+2] == 0 && coef[row+3] == 0 &&
			coef[row+4] == 0 && coef[row+5] == 0 && coef[row+6] == 0 && coef[row+7] == 0 {
			dc := coef[row] << 3
			tmp[row+0] = dc
			tmp[row+1] = dc
			tmp[row+2] = dc
			tmp[row+3] = dc
			tmp[row+4] = dc
			tmp[row+5] = dc
			tmp[row+6] = dc
			tmp[row+7] = dc
			continue
		}

		x0 := (coef[row+0] << 11) + 128
		x1 := coef[row+4] << 11
		x2 := coef[row+6]
		x3 := coef[row+2]
		x4 := coef[row+1]
		x5 := coef[row+7]
		x6 := coef[row+5]
		x7 := coef[row+3]

		// First stage
		x8 := w7 * (x4 + x5)
		x4 = x8 + w1*x4
		x5 = x8 - w5*x5
		x8 = w3 * (x6 + x7)
		x6 = x8 - w3*x6
		x7 = x8 - w7*x7

		// Second stage
		x8 = x0 + x1
		x0 -= x1
		x1 = w6 * (x3 + x2)
		x2 = x1 - w2*x2
		x3 = x1 + w6*x3
		x1 = x4 + x6
		x4 -= x6
		x6 = x5 + x7
		x5 -= x7

		// Third stage
		x7 = x8 + x3
		x8 -= x3
		x3 = x0 + x2
		x0 -= x2
		x2 = (r2 * (x4 + x5)) >> 8
		x4 = (r2 * (x4 - x5)) >> 8

		// Output
		tmp[row+0] = (x7 + x1) >> 8
		tmp[row+1] = (x3 + x2) >> 8
		tmp[row+2] = (x0 + x4) >> 8
		tmp[row+3] = (x8 + x6) >> 8
		tmp[row+4] = (x8 - x6) >> 8
		tmp[row+5] = (x0 - x4) >> 8
		tmp[row+6] = (x3 - x2) >> 8
		tmp[row+7] = (x7 - x1) >> 8
	}

	// 1D IDCT on columns
	for x := 0; x < 8; x++ {
		// Check if AC coefficients are all zero
		if tmp[8+x] == 0 && tmp[16+x] == 0 && tmp[24+x] == 0 &&
			tmp[32+x] == 0 && tmp[40+x] == 0 && tmp[48+x] == 0 && tmp[56+x] == 0 {
			dc := Clamp(int(((tmp[x]+32)>>6)+128), 0, 255)
			out[0*stride+x] = byte(dc)
			out[1*stride+x] = byte(dc)
			out[2*stride+x] = byte(dc)
			out[3*stride+x] = byte(dc)
			out[4*stride+x] = byte(dc)
			out[5*stride+x] = byte(dc)
			out[6*stride+x] = byte(dc)
			out[7*stride+x] = byte(dc)
			continue
		}

		x0 := (tmp[0+x] << 8) + 8192
		x1 := tmp[32+x] << 8
		x2 := tmp[48+x]
		x3 := tmp[16+x]
		x4 := tmp[8+x]
		x5 := tmp[56+x]
		x6 := tmp[40+x]
		x7 := tmp[24+x]

		// First stage
		x8 := w7 * (x4 + x5)
		x4 = x8 + w1*x4
		x5 = x8 - w5*x5
		x8 = w3 * (x6 + x7)
		x6 = x8 - w3*x6
		x7 = x8 - w7*x7

		// Second stage
		x8 = x0 + x1
		x0 -= x1
		x1 = w6 * (x3 + x2)
		x2 = x1 - w2*x2
		x3 = x1 + w6*x3
		x1 = x4 + x6
		x4 -= x6
		x6 = x5 + x7
		x5 -= x7

		// Third stage
		x7 = x8 + x3
		x8 -= x3
		x3 = x0 + x2
		x0 -= x2
		x2 = (r2 * (x4 + x5)) >> 8
		x4 = (r2 * (x4 - x5)) >> 8

		// Output with range limiting and level shift
		out[0*stride+x] = byte(Clamp(int(((x7+x1)>>14)+128), 0, 255))
		out[1*stride+x] = byte(Clamp(int(((x3+x2)>>14)+128), 0, 255))
		out[2*stride+x] = byte(Clamp(int(((x0+x4)>>14)+128), 0, 255))
		out[3*stride+x] = byte(Clamp(int(((x8+x6)>>14)+128), 0, 255))
		out[4*stride+x] = byte(Clamp(int(((x8-x6)>>14)+128), 0, 255))
		out[5*stride+x] = byte(Clamp(int(((x0-x4)>>14)+128), 0, 255))
		out[6*stride+x] = byte(Clamp(int(((x3-x2)>>14)+128), 0, 255))
		out[7*stride+x] = byte(Clamp(int(((x7-x1)>>14)+128), 0, 255))
	}
}
