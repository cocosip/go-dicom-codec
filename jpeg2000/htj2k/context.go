package htj2k

// Context computation for HTJ2K VLC decoding
// Based on ISO/IEC 15444-15:2019 Clause 7.3.5

// ContextComputer computes VLC contexts based on neighbor significance patterns
type ContextComputer struct {
	width  int
	height int

	// Significance maps for current and neighboring samples
	// sigma[y][x] = 1 if sample at (x,y) is significant
	sigma [][]bool
}

// NewContextComputer creates a new context computer
func NewContextComputer(width, height int) *ContextComputer {
	// Initialize significance map
	sigma := make([][]bool, height)
	for i := range sigma {
		sigma[i] = make([]bool, width)
	}

	return &ContextComputer{
		width:  width,
		height: height,
		sigma:  sigma,
	}
}

// SetSignificant marks a sample as significant
func (c *ContextComputer) SetSignificant(x, y int, significant bool) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.sigma[y][x] = significant
	}
}

// IsSignificant checks if a sample is significant
func (c *ContextComputer) IsSignificant(x, y int) bool {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return false
	}
	return c.sigma[y][x]
}

// ComputeContext computes the VLC context for a quad at position (qx, qy)
// Returns context value (0-15) based on neighbor significance pattern
//
// Context computation follows ITU-T T.814 Clause 7.3.5:
// The context depends on the significance of neighboring samples
func (c *ContextComputer) ComputeContext(qx, qy int, isFirstRow bool) uint8 {
	// For quads, we work with 2x2 blocks
	// qx, qy are quad coordinates (not sample coordinates)

	// Convert to sample coordinates (top-left of quad)
	sx := qx * 2
	sy := qy * 2

	if isFirstRow {
		// First row of quads: context based on left neighbors only
		return c.computeFirstRowContext(sx, sy)
	} else {
		// Non-first rows: context based on left and top neighbors
		return c.computeNonFirstRowContext(sx, sy)
	}
}

// computeFirstRowContext computes context for first row of quads
// Context is based on left neighbor significance
func (c *ContextComputer) computeFirstRowContext(sx, sy int) uint8 {
	// Check left neighbors of the quad (samples at x-1)
	leftTop := c.IsSignificant(sx-1, sy)
	leftBottom := c.IsSignificant(sx-1, sy+1)

	// Compute context based on left neighbor pattern
	context := uint8(0)
	if leftTop {
		context |= 0x1
	}
	if leftBottom {
		context |= 0x2
	}

	return context
}

// computeNonFirstRowContext computes context for non-first rows
// Context is based on left and top neighbor significance
func (c *ContextComputer) computeNonFirstRowContext(sx, sy int) uint8 {
	// Check all 8 neighbors around the 2x2 quad
	//
	// Neighbor layout:
	//   TL  T0  T1  TR
	//   L0  Q0  Q1
	//   L1  Q2  Q3
	//
	// Where Q0-Q3 are the quad samples we're decoding

	// Top neighbors
	topLeft := c.IsSignificant(sx-1, sy-1)   // TL
	top0 := c.IsSignificant(sx, sy-1)        // T0
	top1 := c.IsSignificant(sx+1, sy-1)      // T1
	topRight := c.IsSignificant(sx+2, sy-1)  // TR

	// Left neighbors
	left0 := c.IsSignificant(sx-1, sy)       // L0
	left1 := c.IsSignificant(sx-1, sy+1)     // L1

	// Compute context value based on neighbor pattern
	// This is a simplified version - the full spec has detailed rules
	// for computing the context based on significance patterns

	context := uint8(0)

	// Count significant neighbors
	sigCount := 0
	if topLeft {
		sigCount++
	}
	if top0 {
		sigCount++
	}
	if top1 {
		sigCount++
	}
	if topRight {
		sigCount++
	}
	if left0 {
		sigCount++
	}
	if left1 {
		sigCount++
	}

	// Map neighbor significance pattern to context value
	// This is a simplified mapping - full spec has more detailed rules
	if sigCount == 0 {
		context = 0 // No significant neighbors
	} else if sigCount <= 2 {
		context = uint8(sigCount) // 1-2 significant neighbors
	} else if sigCount <= 4 {
		context = uint8(sigCount + 1) // 3-4 significant neighbors
	} else {
		context = uint8(sigCount + 2) // 5+ significant neighbors
	}

	// Cap context at 15 (4 bits)
	if context > 15 {
		context = 15
	}

	return context
}

// ComputeContextDetailed computes context with detailed neighbor analysis
// This follows the full specification more closely
func (c *ContextComputer) ComputeContextDetailed(qx, qy int, isFirstRow bool) uint8 {
	sx := qx * 2
	sy := qy * 2

	if isFirstRow {
		// First row: simpler context based on left neighbors
		return c.computeFirstRowContext(sx, sy)
	}

	// Non-first row: detailed context computation
	// The context depends on both horizontal and vertical significance patterns

	// Horizontal significance (left neighbors)
	h := uint8(0)
	if c.IsSignificant(sx-1, sy) {
		h |= 0x1
	}
	if c.IsSignificant(sx-1, sy+1) {
		h |= 0x2
	}

	// Vertical significance (top neighbors)
	v := uint8(0)
	if c.IsSignificant(sx, sy-1) {
		v |= 0x1
	}
	if c.IsSignificant(sx+1, sy-1) {
		v |= 0x2
	}

	// Diagonal significance
	d := uint8(0)
	if c.IsSignificant(sx-1, sy-1) {
		d |= 0x1
	}
	if c.IsSignificant(sx+2, sy-1) {
		d |= 0x2
	}

	// Combine into context value
	// This is a simplified combination - full spec has lookup tables
	context := (h & 0x3) | ((v & 0x3) << 2) | ((d & 0x1) << 4)

	if context > 15 {
		context = 15
	}

	return context
}

// UpdateQuadSignificance updates significance map after decoding a quad
func (c *ContextComputer) UpdateQuadSignificance(qx, qy int, rho uint8) {
	// rho is the significance pattern for the 2x2 quad
	// bit 0: top-left, bit 1: top-right, bit 2: bottom-left, bit 3: bottom-right

	sx := qx * 2
	sy := qy * 2

	c.SetSignificant(sx, sy, (rho&0x1) != 0)         // Q0: top-left
	c.SetSignificant(sx+1, sy, (rho&0x2) != 0)       // Q1: top-right
	c.SetSignificant(sx, sy+1, (rho&0x4) != 0)       // Q2: bottom-left
	c.SetSignificant(sx+1, sy+1, (rho&0x8) != 0)     // Q3: bottom-right
}
