package t1

// Helper functions for testing

// CalculateMaxBitplane calculates the maximum bitplane for given data
// This simulates what T2 layer would do in real JPEG2000
func CalculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)

	// Find maximum absolute value
	for _, val := range data {
		abs := val
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}

	if maxAbs == 0 {
		return -1 // All zero
	}

	// Find MSB position
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}

	return bitplane - 1
}
