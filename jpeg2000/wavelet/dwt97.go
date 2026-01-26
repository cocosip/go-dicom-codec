package wavelet

// DWT97 implements the 9/7 irreversible wavelet transform
// Used for lossy JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F

// 9/7 filter coefficients (Cohen-Daubechies-Feauveau)
// These are the lifting scheme coefficients for the CDF 9/7 wavelet
const (
	// Analysis (forward transform)
	alpha97 = -1.586134342059924 // Predict 1
	beta97  = -0.052980118572961 // Update 1
	gamma97 = 0.882911075530934  // Predict 2
	delta97 = 0.443506852043971  // Update 2

	// Normalization factors
	K97  = 1.230174104914001 // Low-pass gain
	K97i = 1.0 / K97         // High-pass gain (1/K)
)

// Forward97_1D performs the forward 9/7 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
//
// Note: Uses floating-point arithmetic (irreversible/lossy)
func Forward97_1D(data []float64) {
	Forward97_1DWithParity(data, true)
}

// Forward97_1DWithParity performs the forward 9/7 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
func Forward97_1DWithParity(data []float64, even bool) {
	n := len(data)
	if n <= 1 {
		return
	}

	nL, nH := splitLengths(n, even)
	if nL == 0 || nH == 0 {
		return
	}

	low := make([]float64, nL)
	high := make([]float64, nH)

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

	// Predict 1
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
		high[i] += alpha97 * (left + right)
	}

	// Update 1
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] += beta97 * (left + right)
	}

	// Predict 2
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
		high[i] += gamma97 * (left + right)
	}

	// Update 2
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] += delta97 * (left + right)
	}

	// Normalization
	for i := 0; i < nL; i++ {
		low[i] *= K97
	}
	for i := 0; i < nH; i++ {
		high[i] *= K97i
	}

	// Deinterleave back to [L | H]
	copy(data[:nL], low)
	copy(data[nL:], high)
}

// Inverse97_1D performs the inverse 9/7 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
func Inverse97_1D(data []float64) {
	Inverse97_1DWithParity(data, true)
}

// Inverse97_1DWithParity performs the inverse 9/7 wavelet transform on a 1D signal.
// even=true means low-pass starts at even indices (cas=0). even=false means odd (cas=1).
func Inverse97_1DWithParity(data []float64, even bool) {
	n := len(data)
	if n <= 1 {
		return
	}

	nL, nH := splitLengths(n, even)
	if nL == 0 || nH == 0 {
		return
	}

	low := make([]float64, nL)
	high := make([]float64, nH)

	copy(low, data[:nL])
	copy(high, data[nL:])

	// Inverse normalization
	for i := 0; i < nL; i++ {
		low[i] /= K97
	}
	for i := 0; i < nH; i++ {
		high[i] /= K97i
	}

	// Inverse update 2
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] -= delta97 * (left + right)
	}

	// Inverse predict 2
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
		high[i] -= gamma97 * (left + right)
	}

	// Inverse update 1
	for i := 0; i < nL; i++ {
		left := high[0]
		if i > 0 {
			left = high[i-1]
		}
		right := high[nH-1]
		if i < nH {
			right = high[i]
		}
		low[i] -= beta97 * (left + right)
	}

	// Inverse predict 1
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
		high[i] -= alpha97 * (left + right)
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

// Forward97_2D performs the forward 9/7 wavelet transform on a 2D image
// Applies 1D transform to rows, then to columns (separable transform)
// Input: data (row-major order), width, height, stride
// Output: LL (top-left), HL (top-right), LH (bottom-left), HH (bottom-right)
//
// The stride parameter is the full width of the data array (important for multi-level transforms)
// For the first level, stride == width
// For subsequent levels, stride remains the original width while width shrinks
func Forward97_2D(data []float64, width, height, stride int) {
	Forward97_2DWithParity(data, width, height, stride, true, true)
}

// Forward97_2DWithParity performs the forward 9/7 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
func Forward97_2DWithParity(data []float64, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Transform rows
	if width > 1 {
		row := make([]float64, width)
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Transform row
			Forward97_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}

	// Step 2: Transform columns
	if height > 1 {
		col := make([]float64, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Transform column
			Forward97_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}
}

// Inverse97_2D performs the inverse 9/7 wavelet transform on a 2D image
// Applies inverse 1D transform to columns, then to rows
// Input: data with subbands (LL, HL, LH, HH), width, height, stride
// Output: reconstructed image
func Inverse97_2D(data []float64, width, height, stride int) {
	Inverse97_2DWithParity(data, width, height, stride, true, true)
}

// Inverse97_2DWithParity performs the inverse 9/7 wavelet transform on a 2D image
// evenRow/evenCol control parity for horizontal/vertical passes (cas=0 when true).
func Inverse97_2DWithParity(data []float64, width, height, stride int, evenRow, evenCol bool) {
	if width <= 1 && height <= 1 {
		return
	}

	// Step 1: Inverse transform columns
	if height > 1 {
		col := make([]float64, height)
		for x := 0; x < width; x++ {
			// Extract column (use stride for row indexing)
			for y := 0; y < height; y++ {
				col[y] = data[y*stride+x]
			}

			// Inverse transform column
			Inverse97_1DWithParity(col, evenCol)

			// Write back (use stride for row indexing)
			for y := 0; y < height; y++ {
				data[y*stride+x] = col[y]
			}
		}
	}

	// Step 2: Inverse transform rows
	if width > 1 {
		row := make([]float64, width)
		for y := 0; y < height; y++ {
			// Extract row (use stride for indexing)
			for x := 0; x < width; x++ {
				row[x] = data[y*stride+x]
			}

			// Inverse transform row
			Inverse97_1DWithParity(row, evenRow)

			// Write back (use stride for indexing)
			for x := 0; x < width; x++ {
				data[y*stride+x] = row[x]
			}
		}
	}
}

// ForwardMultilevel97 performs multilevel 9/7 wavelet decomposition
// levels: number of decomposition levels (1 = one level, 2 = two levels, etc.)
// After each level, only the LL subband is further decomposed
func ForwardMultilevel97(data []float64, width, height, levels int) {
	ForwardMultilevel97WithParity(data, width, height, levels, 0, 0)
}

// ForwardMultilevel97WithParity performs multilevel 9/7 wavelet decomposition with origin parity.
func ForwardMultilevel97WithParity(data []float64, width, height, levels int, x0, y0 int) {
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
		Forward97_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)

		// Next level will work only on LL subband (top-left region)
		// Update dimensions for next iteration
		lowW, _ := splitLengths(curWidth, evenRow)
		lowH, _ := splitLengths(curHeight, evenCol)
		curWidth = lowW
		curHeight = lowH
		curX0 = nextCoord(curX0)
		curY0 = nextCoord(curY0)
	}
}

// InverseMultilevel97 performs multilevel 9/7 wavelet reconstruction
// levels: number of decomposition levels to inverse
// Reconstructs from coarsest to finest
func InverseMultilevel97(data []float64, width, height, levels int) {
	InverseMultilevel97WithParity(data, width, height, levels, 0, 0)
}

// InverseMultilevel97WithParity performs multilevel 9/7 wavelet reconstruction with origin parity.
func InverseMultilevel97WithParity(data []float64, width, height, levels int, x0, y0 int) {
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
		evenRow := isEven(levelX0[i-1])
		evenCol := isEven(levelY0[i-1])
		lowW, _ := splitLengths(levelWidths[i-1], evenRow)
		lowH, _ := splitLengths(levelHeights[i-1], evenCol)
		levelWidths[i] = lowW
		levelHeights[i] = lowH
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
		Inverse97_2DWithParity(data, curWidth, curHeight, originalStride, evenRow, evenCol)
	}
}

// ConvertInt32ToFloat64 converts int32 array to float64 for 9/7 transform
func ConvertInt32ToFloat64(data []int32) []float64 {
	result := make([]float64, len(data))
	for i, v := range data {
		result[i] = float64(v)
	}
	return result
}

// ConvertFloat64ToInt32 converts float64 array back to int32 after inverse transform
func ConvertFloat64ToInt32(data []float64) []int32 {
	result := make([]int32, len(data))
	for i, v := range data {
		// Round to nearest integer
		if v >= 0 {
			result[i] = int32(v + 0.5)
		} else {
			result[i] = int32(v - 0.5)
		}
	}
	return result
}
