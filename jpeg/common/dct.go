package common

// DCT performs Discrete Cosine Transform on an 8x8 block
// Input: 64 spatial domain values (range 0-255)
// Output: 64 DCT coefficients
func DCT(input []byte, stride int, coef []int32) {
	// Intermediate buffer
	var tmp [64]int32

	// Constants for fast DCT (scaled by 2048)
	const (
		w1 = 2841 // 2048*sqrt(2)*cos(1*pi/16)
		w2 = 2676 // 2048*sqrt(2)*cos(2*pi/16)
		w3 = 2408 // 2048*sqrt(2)*cos(3*pi/16)
		w5 = 1609 // 2048*sqrt(2)*cos(5*pi/16)
		w6 = 1108 // 2048*sqrt(2)*cos(6*pi/16)
		w7 = 565  // 2048*sqrt(2)*cos(7*pi/16)

		r2 = 181 // 256/sqrt(2)
	)

	// 1D DCT on rows
	for y := 0; y < 8; y++ {
		row := y * 8

		// Level shift and load
		x0 := int32(input[y*stride+0]) - 128
		x1 := int32(input[y*stride+1]) - 128
		x2 := int32(input[y*stride+2]) - 128
		x3 := int32(input[y*stride+3]) - 128
		x4 := int32(input[y*stride+4]) - 128
		x5 := int32(input[y*stride+5]) - 128
		x6 := int32(input[y*stride+6]) - 128
		x7 := int32(input[y*stride+7]) - 128

		// First stage
		x8 := x0 + x7
		x0 -= x7
		x7 = x1 + x6
		x1 -= x6
		x6 = x2 + x5
		x2 -= x5
		x5 = x3 + x4
		x3 -= x4

		// Second stage
		x4 = x8 + x5
		x8 -= x5
		x5 = x7 + x6
		x7 -= x6
		x6 = ((x0 + x3) * r2) >> 8
		x0 = ((x0 - x3) * r2) >> 8
		x3 = x1 + x2
		x1 -= x2

		// Third stage
		x2 = x4 + x5
		x4 -= x5
		x5 = ((x7 + x8) * r2) >> 8
		x7 = ((x7 - x8) * r2) >> 8

		x8 = x1 + x6
		x1 -= x6
		x6 = x0 + x3
		x0 -= x3

		// Output
		tmp[row+0] = x2
		tmp[row+1] = ((w1*x8 - w7*x6) >> 11)
		tmp[row+2] = x5
		tmp[row+3] = ((w3*x1 - w5*x0) >> 11)
		tmp[row+4] = x4
		tmp[row+5] = ((w5*x1 + w3*x0) >> 11)
		tmp[row+6] = x7
		tmp[row+7] = ((w7*x8 + w1*x6) >> 11)
	}

	// 1D DCT on columns
	for x := 0; x < 8; x++ {
		x0 := tmp[0+x]
		x1 := tmp[8+x]
		x2 := tmp[16+x]
		x3 := tmp[24+x]
		x4 := tmp[32+x]
		x5 := tmp[40+x]
		x6 := tmp[48+x]
		x7 := tmp[56+x]

		// First stage
		x8 := x0 + x7
		x0 -= x7
		x7 = x1 + x6
		x1 -= x6
		x6 = x2 + x5
		x2 -= x5
		x5 = x3 + x4
		x3 -= x4

		// Second stage
		x4 = x8 + x5
		x8 -= x5
		x5 = x7 + x6
		x7 -= x6
		x6 = ((x0 + x3) * r2) >> 8
		x0 = ((x0 - x3) * r2) >> 8
		x3 = x1 + x2
		x1 -= x2

		// Third stage
		x2 = x4 + x5
		x4 -= x5
		x5 = ((x7 + x8) * r2) >> 8
		x7 = ((x7 - x8) * r2) >> 8

		x8 = x1 + x6
		x1 -= x6
		x6 = x0 + x3
		x0 -= x3

		// Output with scaling
		coef[0+x] = (x2 + 4) >> 3
		coef[8+x] = ((w1*x8 - w7*x6) >> 14)
		coef[16+x] = (x5 + 2) >> 2
		coef[24+x] = ((w3*x1 - w5*x0) >> 14)
		coef[32+x] = (x4 + 2) >> 2
		coef[40+x] = ((w5*x1 + w3*x0) >> 14)
		coef[48+x] = (x7 + 2) >> 2
		coef[56+x] = ((w7*x8 + w1*x6) >> 14)
	}
}
