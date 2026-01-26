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

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a >= 0 {
		return (a + b - 1) / b
	}
	return a / b
}

func floorDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a >= 0 {
		return a / b
	}
	return -(((-a) + b - 1) / b)
}

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

type bandInfo struct {
	band             int
	width, height    int
	offsetX, offsetY int
}

func resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res int) (resW, resH, resX0, resY0 int) {
	levelNo := numLevels - res
	if levelNo < 0 {
		levelNo = 0
	}
	resW = width
	resH = height
	resX0 = x0
	resY0 = y0
	for i := 0; i < levelNo; i++ {
		lowW, _ := splitLengths(resW, isEven(resX0))
		lowH, _ := splitLengths(resH, isEven(resY0))
		resW = lowW
		resH = lowH
		resX0 = nextCoord(resX0)
		resY0 = nextCoord(resY0)
	}
	return
}

func bandInfosForResolution(width, height, x0, y0, numLevels, res int) (resW, resH, resX0, resY0 int, bands []bandInfo) {
	resW, resH, resX0, resY0 = resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res)
	if res == 0 {
		bands = []bandInfo{{
			band:   0,
			width:  resW,
			height: resH,
		}}
		return
	}
	lowW, lowH, _, _ := resolutionDimsWithOrigin(width, height, x0, y0, numLevels, res-1)
	highW := resW - lowW
	highH := resH - lowH
	bands = []bandInfo{
		{band: 1, width: highW, height: lowH, offsetX: lowW, offsetY: 0},
		{band: 2, width: lowW, height: highH, offsetX: 0, offsetY: lowH},
		{band: 3, width: highW, height: highH, offsetX: lowW, offsetY: lowH},
	}
	return
}
