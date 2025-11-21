package t1

import "fmt"

// CalculateMaxBitplane calculates the maximum bitplane for a set of coefficients
func CalculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)
	for _, v := range data {
		abs := v
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}

	if maxAbs == 0 {
		return 0
	}

	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}
	return bitplane - 1
}

// debugPrintFlags prints flag details for debugging
func debugPrintFlags(label string, flags uint32) {
	fmt.Printf("%s flags=0x%x:", label, flags)

	if flags&T1_SIG != 0 {
		fmt.Printf(" SIG")
	}
	if flags&T1_VISIT != 0 {
		fmt.Printf(" VISIT")
	}
	if flags&T1_REFINE != 0 {
		fmt.Printf(" REFINE")
	}
	if flags&T1_SIGN != 0 {
		fmt.Printf(" SIGN(%s)", map[bool]string{true: "neg", false: "pos"}[flags&T1_SIGN != 0])
	}

	// Neighbor significance
	neighbors := []string{}
	if flags&T1_SIG_N != 0 {
		neighbors = append(neighbors, "N")
	}
	if flags&T1_SIG_S != 0 {
		neighbors = append(neighbors, "S")
	}
	if flags&T1_SIG_W != 0 {
		neighbors = append(neighbors, "W")
	}
	if flags&T1_SIG_E != 0 {
		neighbors = append(neighbors, "E")
	}
	if flags&T1_SIG_NW != 0 {
		neighbors = append(neighbors, "NW")
	}
	if flags&T1_SIG_NE != 0 {
		neighbors = append(neighbors, "NE")
	}
	if flags&T1_SIG_SW != 0 {
		neighbors = append(neighbors, "SW")
	}
	if flags&T1_SIG_SE != 0 {
		neighbors = append(neighbors, "SE")
	}
	if len(neighbors) > 0 {
		fmt.Printf(" | Neighbors:")
		for _, n := range neighbors {
			fmt.Printf(" %s", n)
		}
	}

	// Neighbor signs
	signs := []string{}
	if flags&T1_SIGN_N != 0 {
		signs = append(signs, "N-")
	}
	if flags&T1_SIGN_S != 0 {
		signs = append(signs, "S-")
	}
	if flags&T1_SIGN_W != 0 {
		signs = append(signs, "W-")
	}
	if flags&T1_SIGN_E != 0 {
		signs = append(signs, "E-")
	}
	if len(signs) > 0 {
		fmt.Printf(" | Signs:")
		for _, s := range signs {
			fmt.Printf(" %s", s)
		}
	}

	fmt.Println()
}
