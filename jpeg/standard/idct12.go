package standard

// IDCT12 performs Inverse Discrete Cosine Transform for 12-bit data
// Input: 64 DCT coefficients
// Output: 64 spatial domain values written to output (range 0-4095)
func IDCT12(coef []int32, output []uint16, stride int) {
	// Intermediate buffer
	var tmp [64]int32

	// Constants for fast IDCT (scaled by 256)
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

		x0 := coef[row+0] << 11
		x1 := coef[row+4] << 11
		x2 := coef[row+6]
		x3 := coef[row+2]
		x4 := coef[row+1]
		x5 := coef[row+7]
		x6 := coef[row+5]
		x7 := coef[row+3]

		// Check for shortcut (all AC coefficients are zero)
		if x1 == 0 && x2 == 0 && x3 == 0 && x4 == 0 && x5 == 0 && x6 == 0 && x7 == 0 {
			dc := x0 >> 11
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

		// First stage
		x8 := w7*(x4+x5) + 4
		x4 = (x8 + (w1-w7)*x4) >> 3
		x5 = (x8 - (w1+w7)*x5) >> 3
		x8 = w3*(x6+x7) + 4
		x6 = (x8 - (w3-w5)*x6) >> 3
		x7 = (x8 - (w3+w5)*x7) >> 3

		// Second stage
		x8 = x0 + x1
		x0 -= x1
		x1 = w6*(x3+x2) + 4
		x2 = (x1 - (w2+w6)*x2) >> 3
		x3 = (x1 + (w2-w6)*x3) >> 3
		x1 = x4 + x6
		x4 -= x6
		x6 = x5 + x7
		x5 -= x7

		// Third stage
		x7 = x8 + x3
		x8 -= x3
		x3 = x0 + x2
		x0 -= x2
		x2 = (((x4 + x5) * r2) + 128) >> 8
		x4 = (((x4 - x5) * r2) + 128) >> 8

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
		x0 := (tmp[0+x] << 8) + 8192
		x1 := tmp[32+x] << 8
		x2 := tmp[48+x]
		x3 := tmp[16+x]
		x4 := tmp[8+x]
		x5 := tmp[56+x]
		x6 := tmp[40+x]
		x7 := tmp[24+x]

		// First stage
		x8 := w7*(x4+x5) + 4
		x4 = (x8 + (w1-w7)*x4) >> 3
		x5 = (x8 - (w1+w7)*x5) >> 3
		x8 = w3*(x6+x7) + 4
		x6 = (x8 - (w3-w5)*x6) >> 3
		x7 = (x8 - (w3+w5)*x7) >> 3

		// Second stage
		x8 = x0 + x1
		x0 -= x1
		x1 = w6*(x3+x2) + 4
		x2 = (x1 - (w2+w6)*x2) >> 3
		x3 = (x1 + (w2-w6)*x3) >> 3
		x1 = x4 + x6
		x4 -= x6
		x6 = x5 + x7
		x5 -= x7

		// Third stage
		x7 = x8 + x3
		x8 -= x3
		x3 = x0 + x2
		x0 -= x2
		x2 = (((x4 + x5) * r2) + 128) >> 8
		x4 = (((x4 - x5) * r2) + 128) >> 8

		// Output with level shift and clamping to 12-bit range (0-4095)
		const levelShift = 2048
		output[0*stride+x] = Clamp16((x7+x1)>>14+levelShift, 0, 4095)
		output[1*stride+x] = Clamp16((x3+x2)>>14+levelShift, 0, 4095)
		output[2*stride+x] = Clamp16((x0+x4)>>14+levelShift, 0, 4095)
		output[3*stride+x] = Clamp16((x8+x6)>>14+levelShift, 0, 4095)
		output[4*stride+x] = Clamp16((x8-x6)>>14+levelShift, 0, 4095)
		output[5*stride+x] = Clamp16((x0-x4)>>14+levelShift, 0, 4095)
		output[6*stride+x] = Clamp16((x3-x2)>>14+levelShift, 0, 4095)
		output[7*stride+x] = Clamp16((x7-x1)>>14+levelShift, 0, 4095)
	}
}

// Clamp16 clamps a value to a range for 16-bit values
func Clamp16(val, minVal, maxVal int32) uint16 {
	if val < minVal {
		return uint16(minVal)
	}
	if val > maxVal {
		return uint16(maxVal)
	}
	return uint16(val)
}
