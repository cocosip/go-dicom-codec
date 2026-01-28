package htj2k

// Context computation for HTJ2K VLC decoding.
// The context follows OpenJPH's implementation of ITU-T T.814 equations.

type ContextComputer struct {
	width  int
	height int
	numQX  int
	numQY  int

	// rho[qy][qx] stores the 4-bit significance pattern for each quad.
	rho [][]uint8
}

// NewContextComputer creates a new context computer for a block.
func NewContextComputer(width, height int) *ContextComputer {
	numQX := (width + 1) / 2
	numQY := (height + 1) / 2

	rho := make([][]uint8, numQY)
	for i := range rho {
		rho[i] = make([]uint8, numQX)
	}

	return &ContextComputer{
		width:  width,
		height: height,
		numQX:  numQX,
		numQY:  numQY,
		rho:    rho,
	}
}

// SetSignificant marks a sample as significant or not.
func (c *ContextComputer) SetSignificant(x, y int, significant bool) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	qx := x / 2
	qy := y / 2
	bit := uint8(1 << ((x%2)*2 + (y % 2)))
	if significant {
		c.rho[qy][qx] |= bit
		return
	}
	c.rho[qy][qx] &^= bit
}

// IsSignificant checks if a sample is significant.
func (c *ContextComputer) IsSignificant(x, y int) bool {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return false
	}
	qx := x / 2
	qy := y / 2
	bit := uint8(1 << ((x%2)*2 + (y % 2)))
	return (c.rho[qy][qx] & bit) != 0
}

// ComputeContext computes the VLC context for a quad at (qx, qy).
// Context is a 3-bit value in [0..7].
func (c *ContextComputer) ComputeContext(qx, qy int, isFirstRow bool) uint8 {
	if qx < 0 || qx >= c.numQX || qy < 0 || qy >= c.numQY {
		return 0
	}
	if isFirstRow || qy == 0 {
		if qx == 0 {
			return 0
		}
		left := c.rho[qy][qx-1]
		return ((left >> 1) & 0x7) | (left & 0x1)
	}

	var left uint8
	var aboveLeft uint8
	var above uint8
	var aboveRight uint8

	if qx > 0 {
		left = c.rho[qy][qx-1]
		aboveLeft = c.rho[qy-1][qx-1]
	}
	above = c.rho[qy-1][qx]
	if qx+1 < c.numQX {
		aboveRight = c.rho[qy-1][qx+1]
	}

	c0 := ((aboveLeft >> 3) & 0x1) | ((above >> 1) & 0x1)
	c1 := ((left >> 2) & 0x1) | ((left >> 3) & 0x1)
	c2 := ((above >> 3) & 0x1) | ((aboveRight >> 1) & 0x1)

	return c0 | (c1 << 1) | (c2 << 2)
}

// ComputeContextDetailed is kept for API compatibility.
func (c *ContextComputer) ComputeContextDetailed(qx, qy int, isFirstRow bool) uint8 {
	return c.ComputeContext(qx, qy, isFirstRow)
}

// UpdateQuadSignificance stores the rho pattern for a quad.
func (c *ContextComputer) UpdateQuadSignificance(qx, qy int, rho uint8) {
	if qx < 0 || qx >= c.numQX || qy < 0 || qy >= c.numQY {
		return
	}
	c.rho[qy][qx] = rho & 0xF
}
