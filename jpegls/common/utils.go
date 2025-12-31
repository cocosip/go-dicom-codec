package common

// Abs returns the absolute value of an integer
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Sign returns 1 if n >= 0, -1 if n < 0
func Sign(n int) int {
	if n >= 0 {
		return 1
	}
	return -1
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
