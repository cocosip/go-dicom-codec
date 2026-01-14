package t2

import "fmt"

var debugPrecinctStats = false

func EnablePrecinctStats() {
	debugPrecinctStats = true
}

func logPrecinctStats(precinct *Precinct) {
	if !debugPrecinctStats {
		return
	}
	fmt.Printf("Precinct: NumCB=%dx%d, TotalCB=%d\n", 
		precinct.NumCodeBlocksX, precinct.NumCodeBlocksY, len(precinct.CodeBlocks))
}
