package wavelet

func splitLengths(n int, even bool) (low, high int) {
	if even {
		low = (n + 1) / 2
	} else {
		low = n / 2
	}
	high = n - low
	return
}

func isEven(value int) bool {
	return value&1 == 0
}

func nextCoord(value int) int {
	return (value + 1) >> 1
}
