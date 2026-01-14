package wavelet

// DWT97 implements the 9/7 irreversible wavelet transform
// Used for lossy JPEG 2000 compression
// Reference: ISO/IEC 15444-1:2019 Annex F

// 9/7 filter coefficients (Cohen-Daubechies-Feauveau)
// These are the lifting scheme coefficients for the CDF 9/7 wavelet
const (
	// Analysis (forward transform)
	alpha97 = -1.586134342059924  // Predict 1
	beta97  = -0.052980118572961  // Update 1
	gamma97 = 0.882911075530934   // Predict 2
	delta97 = 0.443506852043971   // Update 2

	// Normalization factors
	K97  = 1.230174104914001  // Low-pass gain
	K97i = 1.0 / K97           // High-pass gain (1/K)
)

// Forward97_1D performs the forward 9/7 wavelet transform on a 1D signal
// Separates the signal into low-pass (L) and high-pass (H) subbands
// Input: data (will be modified in-place)
// Output: first half = L (approximation), second half = H (detail)
//
// Note: Uses floating-point arithmetic (irreversible/lossy)
func Forward97_1D(data []float64) {
	n := len(data)
	if n <= 1 {
		return
	}

	nL := (n + 1) / 2 // Low-pass (even indices)
	nH := n / 2       // High-pass (odd indices)

	temp := make([]float64, n)

	// Step 1: Predict 1 (alpha)
	// H[i] = X[2i+1] + alpha * (X[2i] + X[2i+2])
	for i := 0; i < nH; i++ {
		left := data[2*i]
		var right float64
		if 2*i+2 < n {
			right = data[2*i+2]
		} else {
			right = data[2*i] // Mirror boundary
		}
		temp[nL+i] = data[2*i+1] + alpha97*(left+right)
	}

	// Step 2: Update 1 (beta)
	// L[i] = X[2i] + beta * (H[i-1] + H[i])
	for i := 0; i < nL; i++ {
		var left, right float64
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
		temp[i] = data[2*i] + beta97*(left+right)
	}

	// Copy temp back to data for next steps
	copy(data, temp)

	// Step 3: Predict 2 (gamma)
	// H[i] = H[i] + gamma * (L[i] + L[i+1])
	for i := 0; i < nH; i++ {
		left := data[i]
		var right float64
		if i+1 < nL {
			right = data[i+1]
		} else {
			right = data[nL-1]
		}
		temp[nL+i] = data[nL+i] + gamma97*(left+right)
	}

	// Step 4: Update 2 (delta)
	// L[i] = L[i] + delta * (H[i-1] + H[i])
	for i := 0; i < nL; i++ {
		var left, right float64
		if i > 0 {
			left = temp[nL+i-1]
		} else {
			left = temp[nL]
		}
		if i < nH {
			right = temp[nL+i]
		} else {
			right = temp[nL+nH-1]
		}
		temp[i] = data[i] + delta97*(left+right)
	}

	// Step 5: Normalization
	for i := 0; i < nL; i++ {
		temp[i] *= K97
	}
	for i := 0; i < nH; i++ {
		temp[nL+i] *= K97i
	}

	copy(data, temp)
}

// Inverse97_1D performs the inverse 9/7 wavelet transform on a 1D signal
// Reconstructs the original signal from L and H subbands
// Input: data with first half = L, second half = H (will be modified in-place)
// Output: reconstructed signal
func Inverse97_1D(data []float64) {
	n := len(data)
	if n <= 1 {
		return
	}

	nL := (n + 1) / 2
	nH := n / 2

	temp := make([]float64, n)
	copy(temp, data)

	// Step 1: Inverse normalization
	for i := 0; i < nL; i++ {
		temp[i] /= K97
	}
	for i := 0; i < nH; i++ {
		temp[nL+i] /= K97i
	}

	// Step 2: Inverse update 2 (delta)
	// L[i] = L[i] - delta * (H[i-1] + H[i])
	for i := 0; i < nL; i++ {
		var left, right float64
		if i > 0 {
			left = temp[nL+i-1]
		} else {
			left = temp[nL]
		}
		if i < nH {
			right = temp[nL+i]
		} else {
			right = temp[nL+nH-1]
		}
		temp[i] = temp[i] - delta97*(left+right)
	}

	// Copy to data for next step
	copy(data, temp)

	// Step 3: Inverse predict 2 (gamma)
	// H[i] = H[i] - gamma * (L[i] + L[i+1])
	for i := 0; i < nH; i++ {
		left := data[i]
		var right float64
		if i+1 < nL {
			right = data[i+1]
		} else {
			right = data[nL-1]
		}
		temp[nL+i] = data[nL+i] - gamma97*(left+right)
	}

	// Step 4: Inverse update 1 (beta)
	// X[2i] = L[i] - beta * (H[i-1] + H[i])
	for i := 0; i < nL; i++ {
		var left, right float64
		if i > 0 {
			left = temp[nL+i-1]
		} else {
			left = temp[nL]
		}
		if i < nH {
			right = temp[nL+i]
		} else {
			right = temp[nL+nH-1]
		}
		temp[i] = data[i] - beta97*(left+right)
	}

	// Copy to data for final step
	copy(data, temp)

	// Step 5: Inverse predict 1 (alpha)
	// X[2i+1] = H[i] - alpha * (X[2i] + X[2i+2])
	for i := 0; i < nH; i++ {
		left := data[i]
		var right float64
		if i+1 < nL {
			right = data[i+1]
		} else {
			right = data[nL-1]
		}
		temp[2*i+1] = data[nL+i] - alpha97*(left+right)
	}

	// Copy even samples
	for i := 0; i < nL; i++ {
		temp[2*i] = data[i]
	}

	copy(data, temp)
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
			Forward97_1D(row)

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
			Forward97_1D(col)

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
			Inverse97_1D(col)

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
			Inverse97_1D(row)

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
	// Keep the original stride (full width) throughout all levels
	// This is critical: after level 1, the LL subband is stored in the top-left,
	// but the row stride remains the original full width
	originalStride := width

	// Current resolution dimensions
	curWidth := width
	curHeight := height

	for level := 0; level < levels; level++ {
		if curWidth <= 1 && curHeight <= 1 {
			break
		}

		// Transform current level in-place with original stride
		// For level 0: curWidth == originalStride
		// For level 1+: curWidth < originalStride (only process LL subband)
		Forward97_2D(data, curWidth, curHeight, originalStride)

		// Next level will work only on LL subband (top-left region)
		// Update dimensions for next iteration
		curWidth = (curWidth + 1) / 2
		curHeight = (curHeight + 1) / 2
	}
}

// InverseMultilevel97 performs multilevel 9/7 wavelet reconstruction
// levels: number of decomposition levels to inverse
// Reconstructs from coarsest to finest
func InverseMultilevel97(data []float64, width, height, levels int) {
	// Keep the original stride (full width) throughout all levels
	originalStride := width

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

		// Inverse transform this level in-place with original stride
		Inverse97_2D(data, curWidth, curHeight, originalStride)
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
