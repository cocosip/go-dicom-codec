package wavelet

// DWT53 implements the 5/3 reversible wavelet transform
// Used for lossless JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F

// Forward53_1D performs the forward 5/3 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
//
// Performance notes:
// - Uses integer arithmetic only (no floating point)
// - Bit shifts (>>) are faster than division for powers of 2
// - In-place processing minimizes memory allocations
// - Typical performance: ~100ns for 64 samples, ~1.5µs for 1024 samples
func Forward53_1D(data []int32) {
	Forward53_1DWithParity(data, true)
}

// Forward53_1DWithParity performs the forward 5/3 wavelet transform on a 1D signal
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
func Forward53_1DWithParity(data []int32, even bool) {
	n := len(data)
	if n <= 1 {
		if !even && n == 1 {
			data[0] *= 2
		}
		return
	}

	nL, nH := splitLengths(n, even)
	if nL == 0 || nH == 0 {
		return
	}

	low := make([]int32, nL)
	high := make([]int32, nH)

	if even {
		for i := 0; i < nL; i++ {
			low[i] = data[2*i]
		}
		for i := 0; i < nH; i++ {
			high[i] = data[2*i+1]
		}
	} else {
		for i := 0; i < nL; i++ {
			low[i] = data[2*i+1]
		}
		for i := 0; i < nH; i++ {
			high[i] = data[2*i]
		}
	}

	// Predict
	for i := 0; i < nH; i++ {
		leftIdx := i
		if leftIdx >= nL {
			leftIdx = nL - 1
		}
		rightIdx := leftIdx
		if i+1 < nL {
			rightIdx = i + 1
		}
		left := low[leftIdx]
		right := low[rightIdx]
		high[i] = high[i] - ((left + right) >> 1)
	}

	// Update
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] = low[i] + ((left + right + 2) >> 2)
	}

	// Deinterleave back to [L | H]
	copy(data[:nL], low)
	copy(data[nL:], high)
}

// Inverse53_1D performs the inverse 5/3 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
//
// Performance notes:
// - Perfect reconstruction guaranteed (lossless)
// - Integer-only arithmetic ensures bit-exact inverse
// - Performance similar to Forward53_1D
// - Optimization: could benefit from SIMD for large signals
func Inverse53_1D(data []int32) {
	Inverse53_1DWithParity(data, true)
}

// Inverse53_1DWithParity performs the inverse 5/3 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
func Inverse53_1DWithParity(data []int32, even bool) {
	n := len(data)
	if n <= 1 {
		if !even && n == 1 {
			data[0] /= 2
		}
		return
	}

	nL, nH := splitLengths(n, even)
	if nL == 0 || nH == 0 {
		return
	}

	low := make([]int32, nL)
	high := make([]int32, nH)

	copy(low, data[:nL])
	copy(high, data[nL:])

	// Reverse update
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] = low[i] - ((left + right + 2) >> 2)
	}

	// Reverse predict
	for i := 0; i < nH; i++ {
		leftIdx := i
		if leftIdx >= nL {
			leftIdx = nL - 1
		}
		rightIdx := leftIdx
		if i+1 < nL {
			rightIdx = i + 1
		}
		left := low[leftIdx]
		right := low[rightIdx]
		high[i] = high[i] + ((left + right) >> 1)
	}

	// Interleave back to samples
	if even {
		for i := 0; i < nL; i++ {
			data[2*i] = low[i]
		}
		for i := 0; i < nH; i++ {
			data[2*i+1] = high[i]
		}
	} else {
		for i := 0; i < nH; i++ {
			data[2*i] = high[i]
		}
		for i := 0; i < nL; i++ {
			data[2*i+1] = low[i]
		}
	}
}

// Forward53_2D performs the forward 5/3 wavelet transform on a 2D image
// Applies 1D transform to rows, then to columns (separable transform)
// Input: data (row-major order), width, height, stride
// Output: LL (top-left), HL (top-right), LH (bottom-left), HH (bottom-right)
//
// The stride parameter is the full width of the data array (important for multi-level transforms)
// For the first level, stride == width
// For subsequent levels, stride remains the original width while width shrinks
//
// Performance notes:
// - Separable transform: rows first, then columns
// - Row/column buffers reused to minimize allocations
// - Cache-friendly row processing (contiguous memory access)
// - Column processing less cache-friendly (strided access)
// - Typical performance: ~13µs for 64x64, ~102µs for 256x256
// - Potential optimization: transpose for better column access pattern
func Forward53_2D(data []int32, width, height, stride int) {
	Forward53_2DWithParity(data, width, height, stride, true, true)
}

// Forward53_2DWithParity performs the forward 5/3 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
func Forward53_2DWithParity(data []int32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Allocate row buffer
	row := make([]int32, width)

	// Step 1: Transform rows
	if width > 1 {
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Transform row
			Forward53_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}

	// Step 2: Transform columns
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Transform column
			Forward53_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}
}

// Inverse53_2D performs the inverse 5/3 wavelet transform on a 2D image
// Applies inverse 1D transform to columns, then to rows
// Input: data with subbands (LL, HL, LH, HH), width, height, stride
// Output: reconstructed image
func Inverse53_2D(data []int32, width, height, stride int) {
	Inverse53_2DWithParity(data, width, height, stride, true, true)
}

// Inverse53_2DWithParity performs the inverse 5/3 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
func Inverse53_2DWithParity(data []int32, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Inverse transform columns
	if height > 1 {
		col := make([]int32, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Inverse transform column
			Inverse53_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}

	// Step 2: Inverse transform rows
	if width > 1 {
		row := make([]int32, width)
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Inverse transform row
			Inverse53_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}
}

// ForwardMultilevel performs multilevel 5/3 wavelet decomposition
// levels: number of decomposition levels (1 = one level, 2 = two levels, etc.)
// After each level, only the LL subband is further decomposed
func ForwardMultilevel(data []int32, width, height, levels int) {
	ForwardMultilevelWithParity(data, width, height, levels, 0, 0)
}

// ForwardMultilevelWithParity performs multilevel 5/3 wavelet decomposition with origin parity.
func ForwardMultilevelWithParity(data []int32, width, height, levels int, x0, y0 int) {
	// Keep the original stride (full width) throughout all levels
	// This is critical: after level 1, the LL subband is stored in the top-left,
	// but the row stride remains the original full width
	originalStride := width

	// Current resolution dimensions
	curWidth := width
	curHeight := height

	curX0 := x0
	curY0 := y0

	for level := 0; level < levels; level++ {
		if curWidth <= 1 && curHeight <= 1 {
			break
		}

		evenRow := isEven(curX0)
		evenCol := isEven(curY0)

		// Transform current level in-place with original stride
		// For level 0: curWidth == originalStride
		// For level 1+: curWidth < originalStride (only process LL subband)
		Forward53_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)

		// Next level will work only on LL subband (top-left region)
		// Update dimensions for next iteration
		curWidth = (curWidth + 1) / 2
		curHeight = (curHeight + 1) / 2
		curX0 = nextCoord(curX0)
		curY0 = nextCoord(curY0)
	}
}

// InverseMultilevel performs multilevel 5/3 wavelet reconstruction
// levels: number of decomposition levels to inverse
// Reconstructs from coarsest to finest
func InverseMultilevel(data []int32, width, height, levels int) {
	InverseMultilevelWithParity(data, width, height, levels, 0, 0)
}

// InverseMultilevelWithParity performs multilevel 5/3 wavelet reconstruction with origin parity.
func InverseMultilevelWithParity(data []int32, width, height, levels int, x0, y0 int) {
	// Keep the original stride (full width) throughout all levels
	originalStride := width

	// Calculate dimensions at each level
	levelWidths := make([]int, levels+1)
	levelHeights := make([]int, levels+1)
	levelX0 := make([]int, levels+1)
	levelY0 := make([]int, levels+1)
	levelWidths[0] = width
	levelHeights[0] = height
	levelX0[0] = x0
	levelY0[0] = y0

	for i := 1; i <= levels; i++ {
		levelWidths[i] = (levelWidths[i-1] + 1) / 2
		levelHeights[i] = (levelHeights[i-1] + 1) / 2
		levelX0[i] = nextCoord(levelX0[i-1])
		levelY0[i] = nextCoord(levelY0[i-1])
	}

	// Inverse transform from coarsest to finest
	for level := levels - 1; level >= 0; level-- {
		curWidth := levelWidths[level]
		curHeight := levelHeights[level]
		evenRow := isEven(levelX0[level])
		evenCol := isEven(levelY0[level])

		// Inverse transform this level in-place with original stride
		Inverse53_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)
	}
}
