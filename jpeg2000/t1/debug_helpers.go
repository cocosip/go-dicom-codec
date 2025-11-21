package t1

import "fmt"

// debugPrintFlags prints detailed flag information for a coefficient
func debugPrintFlags(prefix string, flags uint32) {
	fmt.Printf("%s flags=0x%x: ", prefix, flags)

	if flags&T1_SIG != 0 {
		fmt.Printf("SIG ")
	}
	if flags&T1_REFINE != 0 {
		fmt.Printf("REFINE ")
	}
	if flags&T1_VISIT != 0 {
		fmt.Printf("VISIT ")
	}
	if flags&T1_SIGN != 0 {
		fmt.Printf("SIGN(neg) ")
	}

	// Neighbor significance
	neighbors := ""
	if flags&T1_SIG_N != 0 {
		neighbors += "N "
	}
	if flags&T1_SIG_S != 0 {
		neighbors += "S "
	}
	if flags&T1_SIG_W != 0 {
		neighbors += "W "
	}
	if flags&T1_SIG_E != 0 {
		neighbors += "E "
	}
	if flags&T1_SIG_NW != 0 {
		neighbors += "NW "
	}
	if flags&T1_SIG_NE != 0 {
		neighbors += "NE "
	}
	if flags&T1_SIG_SW != 0 {
		neighbors += "SW "
	}
	if flags&T1_SIG_SE != 0 {
		neighbors += "SE "
	}

	if neighbors != "" {
		fmt.Printf("| Neighbors: %s", neighbors)
	}

	// Neighbor signs
	signs := ""
	if flags&T1_SIGN_N != 0 {
		signs += "N- "
	}
	if flags&T1_SIGN_S != 0 {
		signs += "S- "
	}
	if flags&T1_SIGN_W != 0 {
		signs += "W- "
	}
	if flags&T1_SIGN_E != 0 {
		signs += "E- "
	}

	if signs != "" {
		fmt.Printf("| Signs: %s", signs)
	}

	fmt.Println()
}

// compareFlags compares encoder and decoder flags for a position
func compareFlags(x, y int, encFlags, decFlags uint32) bool {
	if encFlags != decFlags {
		fmt.Printf("FLAGS MISMATCH at (%d,%d):\n", x, y)
		debugPrintFlags("  ENC", encFlags)
		debugPrintFlags("  DEC", decFlags)
		return false
	}
	return true
}
