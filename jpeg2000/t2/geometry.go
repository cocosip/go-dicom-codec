package t2

// ceilDivPow2 computes ceil(n / 2^pow) for pow >= 0.
func ceilDivPow2(n, pow int) int {
	if pow <= 0 {
		return n
	}
	divisor := 1 << pow
	return (n + divisor - 1) / divisor
}

// resolutionDim returns the resolution-level dimension using ceil division.
func resolutionDim(n, numLevels, res int) int {
	pow := numLevels - res
	if pow < 0 {
		pow = 0
	}
	val := ceilDivPow2(n, pow)
	if val < 1 {
		val = 1
	}
	return val
}

// subbandDim returns the subband dimension for a resolution using OpenJPEG's formula.
func subbandDim(n, numLevels, res int) int {
	if res == 0 {
		val := ceilDivPow2(n, numLevels)
		if val < 1 {
			return 1
		}
		return val
	}
	pow := numLevels - res + 1
	if pow < 0 {
		pow = 0
	}
	val := ceilDivPow2(n, pow)
	if val < 1 {
		val = 1
	}
	return val
}
