package colorspace

// RGBToYCbCr converts RGB to YCbCr color space
// This is used for RGB images in JPEG 2000
// Reference: ISO/IEC 15444-1:2019 Annex G.2
func RGBToYCbCr(r, g, b int32) (y, cb, cr int32) {
	// ITU-R BT.601 conversion (for JPEG 2000)
	// Y  = 0.299*R + 0.587*G + 0.114*B
	// Cb = -0.168736*R - 0.331264*G + 0.5*B + 128
	// Cr = 0.5*R - 0.418688*G - 0.081312*B + 128

	// Using fixed-point arithmetic (16-bit fractional part)
	// Multiply by 65536 for fixed point
	const (
		yR  = 19595  // 0.299 * 65536
		yG  = 38470  // 0.587 * 65536
		yB  = 7471   // 0.114 * 65536
		cbR = -11059 // -0.168736 * 65536
		cbG = -21709 // -0.331264 * 65536
		cbB = 32768  // 0.5 * 65536
		crR = 32768  // 0.5 * 65536
		crG = -27439 // -0.418688 * 65536
		crB = -5329  // -0.081312 * 65536
	)

	y = (yR*r + yG*g + yB*b + 32768) >> 16
	cb = ((cbR*r + cbG*g + cbB*b + 32768) >> 16) + 128
	cr = ((crR*r + crG*g + crB*b + 32768) >> 16) + 128

	return
}

// YCbCrToRGB converts YCbCr to RGB color space
// Reference: ISO/IEC 15444-1:2019 Annex G.2
func YCbCrToRGB(y, cb, cr int32) (r, g, b int32) {
	// Inverse ITU-R BT.601 conversion
	// R = Y + 1.402 * (Cr - 128)
	// G = Y - 0.344136 * (Cb - 128) - 0.714136 * (Cr - 128)
	// B = Y + 1.772 * (Cb - 128)

	// Using fixed-point arithmetic
	const (
		crR = 91881  // 1.402 * 65536
		cbG = -22553 // -0.344136 * 65536
		crG = -46802 // -0.714136 * 65536
		cbB = 116130 // 1.772 * 65536
	)

	cb -= 128
	cr -= 128

	r = y + ((crR*cr + 32768) >> 16)
	g = y + ((cbG*cb + crG*cr + 32768) >> 16)
	b = y + ((cbB*cb + 32768) >> 16)

	// Clamp to [0, 255]
	if r < 0 {
		r = 0
	} else if r > 255 {
		r = 255
	}

	if g < 0 {
		g = 0
	} else if g > 255 {
		g = 255
	}

	if b < 0 {
		b = 0
	} else if b > 255 {
		b = 255
	}

	return
}

// ConvertRGBToYCbCr converts an RGB image to YCbCr components
// Input: interleaved RGB data [R0,G0,B0,R1,G1,B1,...]
// Output: three separate component arrays [Y], [Cb], [Cr]
func ConvertRGBToYCbCr(rgb []int32, width, height int) (y, cb, cr []int32) {
	numPixels := width * height
	y = make([]int32, numPixels)
	cb = make([]int32, numPixels)
	cr = make([]int32, numPixels)

	for i := 0; i < numPixels; i++ {
		r := rgb[i*3]
		g := rgb[i*3+1]
		b := rgb[i*3+2]

		y[i], cb[i], cr[i] = RGBToYCbCr(r, g, b)
	}

	return
}

// ConvertYCbCrToRGB converts YCbCr components to RGB image
// Input: three separate component arrays [Y], [Cb], [Cr]
// Output: interleaved RGB data [R0,G0,B0,R1,G1,B1,...]
func ConvertYCbCrToRGB(y, cb, cr []int32, width, height int) []int32 {
	numPixels := width * height
	rgb := make([]int32, numPixels*3)

	for i := 0; i < numPixels; i++ {
		r, g, b := YCbCrToRGB(y[i], cb[i], cr[i])
		rgb[i*3] = r
		rgb[i*3+1] = g
		rgb[i*3+2] = b
	}

	return rgb
}

// InterleaveComponents interleaves separate component arrays
// Input: components [[C0_p0, C0_p1, ...], [C1_p0, C1_p1, ...], ...]
// Output: interleaved [C0_p0, C1_p0, ..., C0_p1, C1_p1, ...]
func InterleaveComponents(components [][]int32) []int32 {
	if len(components) == 0 {
		return nil
	}

	numComponents := len(components)
	numPixels := len(components[0])

	result := make([]int32, numPixels*numComponents)

	for p := 0; p < numPixels; p++ {
		for c := 0; c < numComponents; c++ {
			result[p*numComponents+c] = components[c][p]
		}
	}

	return result
}

// DeinterleaveComponents separates interleaved data into component arrays
// Input: interleaved [C0_p0, C1_p0, ..., C0_p1, C1_p1, ...]
// Output: components [[C0_p0, C0_p1, ...], [C1_p0, C1_p1, ...], ...]
func DeinterleaveComponents(data []int32, numComponents int) [][]int32 {
	if len(data) == 0 || numComponents == 0 {
		return nil
	}

	numPixels := len(data) / numComponents
	components := make([][]int32, numComponents)

	for c := 0; c < numComponents; c++ {
		components[c] = make([]int32, numPixels)
	}

	for p := 0; p < numPixels; p++ {
		for c := 0; c < numComponents; c++ {
			components[c][p] = data[p*numComponents+c]
		}
	}

	return components
}

func ConvertComponentsRGBToYCbCr(r, g, b []int32) (y, cb, cr []int32) {
    n := len(r)
    y = make([]int32, n)
    cb = make([]int32, n)
    cr = make([]int32, n)
    for i := 0; i < n; i++ {
        y[i], cb[i], cr[i] = RGBToYCbCr(r[i], g[i], b[i])
    }
    return
}

func ConvertComponentsYCbCrToRGB(y, cb, cr []int32) (r, g, b []int32) {
    n := len(y)
    r = make([]int32, n)
    g = make([]int32, n)
    b = make([]int32, n)
    for i := 0; i < n; i++ {
        r[i], g[i], b[i] = YCbCrToRGB(y[i], cb[i], cr[i])
    }
    return
}
