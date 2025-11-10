package wavelet

// DWT53 implements the 5/3 reversible wavelet transform
// Used for lossless JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F

// Forward53_1D performs the forward 5/3 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
func Forward53_1D(data []int32) {
	n := len(data)
	if n <= 1 {
		return
	}

	// Number of low-pass and high-pass coefficients
	nL := (n + 1) / 2 // Low-pass (even indices)
	nH := n / 2       // High-pass (odd indices)

	// Temporary buffers
	temp := make([]int32, n)

	// Step 1: Predict (high-pass filter)
	// H[i] = X[2i+1] - floor((X[2i] + X[2i+2]) / 2)
	for i := 0; i < nH; i++ {
		// Get neighboring even samples
		left := data[2*i]
		var right int32
		if 2*i+2 < n {
			right = data[2*i+2]
		} else {
			right = data[2*i] // Mirror boundary
		}
		// Predict odd sample from neighboring even samples
		temp[nL+i] = data[2*i+1] - ((left + right) >> 1)
	}

	// Step 2: Update (low-pass filter)
	// L[i] = X[2i] + floor((H[i-1] + H[i] + 2) / 4)
	for i := 0; i < nL; i++ {
		// Get neighboring high-pass coefficients
		var left, right int32
		if i > 0 {
			left = temp[nL+i-1]
		} else {
			left = temp[nL] // First high-pass coefficient
		}
		if i < nH {
			right = temp[nL+i]
		} else {
			right = temp[nL+nH-1] // Last high-pass coefficient
		}
		// Update even sample
		temp[i] = data[2*i] + ((left + right + 2) >> 2)
	}

	// Copy result back
	copy(data, temp)
}

// Inverse53_1D performs the inverse 5/3 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
func Inverse53_1D(data []int32) {
	n := len(data)
	if n <= 1 {
		return
	}

	nL := (n + 1) / 2
	nH := n / 2

	temp := make([]int32, n)

	// Step 1: Reverse update
	// X[2i] = L[i] - floor((H[i-1] + H[i] + 2) / 4)
	for i := 0; i < nL; i++ {
		var left, right int32
		if i > 0 {
			left = data[nL+i-1]
		} else {
			left = data[nL]
		}
		if i < nH {
			right = data[nL+i]
		} else {
			right = data[nL+nH-1]
		}
		temp[2*i] = data[i] - ((left + right + 2) >> 2)
	}

	// Step 2: Reverse predict
	// X[2i+1] = H[i] + floor((X[2i] + X[2i+2]) / 2)
	for i := 0; i < nH; i++ {
		left := temp[2*i]
		var right int32
		if 2*i+2 < n {
			right = temp[2*i+2]
		} else {
			right = temp[2*i]
		}
		temp[2*i+1] = data[nL+i] + ((left + right) >> 1)
	}

	copy(data, temp)
}

// Forward53_2D performs the forward 5/3 wavelet transform on a 2D image
// Applies 1D transform to rows, then to columns (separable transform)
// Input: data (row-major order), width, height
// Output: LL (top-left), HL (top-right), LH (bottom-left), HH (bottom-right)
func Forward53_2D(data []int32, width, height int) {
	if width <= 1 && height <= 1 {
		return
	}

	// Allocate row buffer
	row := make([]int32, width)

	// Step 1: Transform rows
	if width > 1 {
		for y := 0; y < height; y++ {
			// Extract row
			for x := 0; x < width; x++ {
				row[x] = data[y*width+x]
			}

			// Transform row
			Forward53_1D(row)

			// Write back
			for x := 0; x < width; x++ {
				data[y*width+x] = row[x]
			}
		}
	}

	// Step 2: Transform columns
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column
			for y := 0; y < height; y++ {
				col[y] = data[y*width+x]
			}

			// Transform column
			Forward53_1D(col)

			// Write back
			for y := 0; y < height; y++ {
				data[y*width+x] = col[y]
			}
		}
	}
}

// Inverse53_2D performs the inverse 5/3 wavelet transform on a 2D image
// Applies inverse 1D transform to columns, then to rows
// Input: data with subbands (LL, HL, LH, HH), width, height
// Output: reconstructed image
func Inverse53_2D(data []int32, width, height int) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Inverse transform columns
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column
			for y := 0; y < height; y++ {
				col[y] = data[y*width+x]
			}

			// Inverse transform column
			Inverse53_1D(col)

			// Write back
			for y := 0; y < height; y++ {
				data[y*width+x] = col[y]
			}
		}
	}

	// Step 2: Inverse transform rows
	if width > 1 {
		row := make([]int32, width)
		for y := 0; y < height; y++ {
			// Extract row
			for x := 0; x < width; x++ {
				row[x] = data[y*width+x]
			}

			// Inverse transform row
			Inverse53_1D(row)

			// Write back
			for x := 0; x < width; x++ {
				data[y*width+x] = row[x]
			}
		}
	}
}

// ForwardMultilevel performs multilevel 5/3 wavelet decomposition
// levels: number of decomposition levels (1 = one level, 2 = two levels, etc.)
// After each level, only the LL subband is further decomposed
func ForwardMultilevel(data []int32, width, height, levels int) {
	curWidth := width
	curHeight := height

	for level := 0; level < levels; level++ {
		if curWidth <= 1 && curHeight <= 1 {
			break
		}

		// Create temporary buffer for this level
		temp := make([]int32, curWidth*curHeight)
		for i := 0; i < curWidth*curHeight; i++ {
			temp[i] = data[i]
		}

		// Transform this level
		Forward53_2D(temp, curWidth, curHeight)

		// Copy result back to data
		for i := 0; i < curWidth*curHeight; i++ {
			data[i] = temp[i]
		}

		// Next level works only on LL subband (top-left quarter)
		curWidth = (curWidth + 1) / 2
		curHeight = (curHeight + 1) / 2
	}
}

// InverseMultilevel performs multilevel 5/3 wavelet reconstruction
// levels: number of decomposition levels to inverse
// Reconstructs from finest to coarsest level
func InverseMultilevel(data []int32, width, height, levels int) {
	// Calculate dimensions at each level
	levelWidths := make([]int, levels+1)
	levelHeights := make([]int, levels+1)
	levelWidths[0] = width
	levelHeights[0] = height

	for i := 1; i <= levels; i++ {
		levelWidths[i] = (levelWidths[i-1] + 1) / 2
		levelHeights[i] = (levelHeights[i-1] + 1) / 2
	}

	// Inverse transform from coarsest to finest
	for level := levels - 1; level >= 0; level-- {
		curWidth := levelWidths[level]
		curHeight := levelHeights[level]

		// Create temporary buffer
		temp := make([]int32, curWidth*curHeight)
		for i := 0; i < curWidth*curHeight; i++ {
			temp[i] = data[i]
		}

		// Inverse transform this level
		Inverse53_2D(temp, curWidth, curHeight)

		// Copy result back
		for i := 0; i < curWidth*curHeight; i++ {
			data[i] = temp[i]
		}
	}
}
