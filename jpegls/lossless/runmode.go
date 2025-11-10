package lossless

// Run mode is used when encoding flat regions (many pixels with same value)
// This provides better compression for smooth areas

// RunModeEncoder handles run mode encoding
type RunModeEncoder struct {
	runIndex  int // Index of current run (J[0] to J[31])
	runLength int // Length of current run
}

// NewRunModeEncoder creates a new run mode encoder
func NewRunModeEncoder() *RunModeEncoder {
	return &RunModeEncoder{
		runIndex:  0,
		runLength: 0,
	}
}

// EncodeRun encodes a run of identical values
// runLength: number of pixels in the run
// endOfLine: true if run ends at end of line
// Returns encoded run representation
func (rme *RunModeEncoder) EncodeRun(runLength int, endOfLine bool) []int {
	// JPEG-LS uses a variable-length encoding for runs
	// Based on adaptive J[runIndex] parameter

	runs := []int{}

	// Compute J[runIndex] - the expected run length
	j := rme.computeJ()

	for runLength >= j {
		// Encode a full run of length j
		runs = append(runs, 1) // Signal: continue run
		runLength -= j
		rme.updateRunIndex(true) // Increase run index
		j = rme.computeJ()
	}

	// Encode remaining partial run
	if runLength > 0 || !endOfLine {
		runs = append(runs, 0)           // Signal: end run
		runs = append(runs, runLength)   // Actual remainder
		rme.updateRunIndex(false)        // Reset or decrease run index
	}

	return runs
}

// computeJ computes the expected run length J[runIndex]
// J[0] = 4, J[i+1] = min(2*J[i], maxRun)
func (rme *RunModeEncoder) computeJ() int {
	if rme.runIndex >= 32 {
		rme.runIndex = 31 // Clamp
	}

	// Standard J values from JPEG-LS
	j := 4 // J[0] = 4
	for i := 0; i < rme.runIndex; i++ {
		j *= 2
		if j > 65536 { // Practical limit
			j = 65536
			break
		}
	}
	return j
}

// updateRunIndex updates the run index based on success/failure
func (rme *RunModeEncoder) updateRunIndex(success bool) {
	if success {
		// Run was at least J[runIndex], increase index
		if rme.runIndex < 31 {
			rme.runIndex++
		}
	} else {
		// Run was less than J[runIndex], decrease index
		if rme.runIndex > 0 {
			rme.runIndex--
		}
	}
}

// Reset resets the run mode encoder (e.g., at start of new line)
func (rme *RunModeEncoder) Reset() {
	// Don't reset runIndex - it adapts across the image
	rme.runLength = 0
}

// RunModeDecoder handles run mode decoding
type RunModeDecoder struct {
	runIndex int // Index of current run (J[0] to J[31])
}

// NewRunModeDecoder creates a new run mode decoder
func NewRunModeDecoder() *RunModeDecoder {
	return &RunModeDecoder{
		runIndex: 0,
	}
}

// DecodeRun decodes run length from encoded representation
func (rmd *RunModeDecoder) DecodeRun(bits []int) int {
	runLength := 0
	idx := 0

	for idx < len(bits) {
		signal := bits[idx]
		idx++

		if signal == 1 {
			// Continue run: add J[runIndex]
			j := rmd.computeJ()
			runLength += j
			rmd.updateRunIndex(true)
		} else {
			// End run: read remainder
			if idx < len(bits) {
				remainder := bits[idx]
				idx++
				runLength += remainder
			}
			rmd.updateRunIndex(false)
			break
		}
	}

	return runLength
}

// computeJ computes the expected run length J[runIndex]
func (rmd *RunModeDecoder) computeJ() int {
	if rmd.runIndex >= 32 {
		rmd.runIndex = 31
	}

	j := 4
	for i := 0; i < rmd.runIndex; i++ {
		j *= 2
		if j > 65536 {
			j = 65536
			break
		}
	}
	return j
}

// updateRunIndex updates the run index
func (rmd *RunModeDecoder) updateRunIndex(success bool) {
	if success {
		if rmd.runIndex < 31 {
			rmd.runIndex++
		}
	} else {
		if rmd.runIndex > 0 {
			rmd.runIndex--
		}
	}
}

// Reset resets the run mode decoder
func (rmd *RunModeDecoder) Reset() {
	// Keep runIndex for adaptation
}

// DetectRun checks if current position starts a run
// Returns run length if run detected, 0 otherwise
func DetectRun(data []int, pos, width, maxVal int, threshold int) int {
	if pos >= len(data) {
		return 0
	}

	runValue := data[pos]
	runLength := 1

	// Count consecutive pixels with same (or very close) value
	for i := pos + 1; i < len(data) && i < pos+width; i++ {
		if abs(data[i]-runValue) <= threshold {
			runLength++
		} else {
			break
		}
	}

	// Only consider it a run if length is significant
	if runLength >= 4 { // Minimum run length
		return runLength
	}

	return 0
}
