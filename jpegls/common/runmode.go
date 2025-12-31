package common

import "github.com/cocosip/go-dicom-codec/jpegls/lossless"

// J array for run mode encoding
// This is defined in lossless package and re-exported here for convenience
var J = lossless.J

// IncrementRunIndex increments the run index with bounds checking
func IncrementRunIndex(runIndex int) int {
	if runIndex < 31 {
		return runIndex + 1
	}
	return runIndex
}

// DecrementRunIndex decrements the run index with bounds checking
func DecrementRunIndex(runIndex int) int {
	if runIndex > 0 {
		return runIndex - 1
	}
	return runIndex
}
