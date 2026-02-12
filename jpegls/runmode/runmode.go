// Package runmode provides shared utilities and tables for JPEG-LS modules,
// including run-mode constants and helpers used by encoder/decoder implementations.
package runmode

// J array for run mode encoding
// This is defined in lossless package and re-exported here for convenience
// J array (JPEG-LS A.2.1)
var J = [32]int{
	0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3,
	4, 4, 5, 5, 6, 6, 7, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

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
