package t2

// Tag Tree Decoding for JPEG 2000 Packet Headers
// Reference: ISO/IEC 15444-1:2019 Annex B.10.2

// TagTreeDecoder handles tag tree decoding operations
type TagTreeDecoder struct {
	tree *TagTree
}

// NewTagTreeDecoder creates a new tag tree decoder
func NewTagTreeDecoder(tree *TagTree) *TagTreeDecoder {
	return &TagTreeDecoder{
		tree: tree,
	}
}

// Decode decodes a tag tree value for a given leaf node
// This is called during packet header parsing
//
// Parameters:
//   - leafX, leafY: Coordinates of the leaf node
//   - threshold: Stop when node value < threshold
//   - bitReader: Function to read one bit from bitstream
//
// Returns the decoded value (or current lower bound if not fully decoded)
func (ttd *TagTreeDecoder) Decode(leafX, leafY, threshold int, bitReader func() (int, error)) (int, error) {
	if leafX < 0 || leafX >= ttd.tree.Width || leafY < 0 || leafY >= ttd.tree.Height {
		return 0, nil
	}

	// Check if already decoded to sufficient precision
	leafIdx := leafY*ttd.tree.Width + leafX
	if ttd.tree.Nodes[leafIdx] >= 0 && ttd.tree.Nodes[leafIdx] < threshold {
		return ttd.tree.Nodes[leafIdx], nil
	}

	// Start from leaf and traverse up to root
	// Decode from root down, updating state
	value := ttd.decodeNode(leafIdx, threshold, bitReader)

	return value, nil
}

// decodeNode decodes a single node in the tag tree
func (ttd *TagTreeDecoder) decodeNode(nodeIdx, threshold int, bitReader func() (int, error)) int {
	// If node value is already known and below threshold, return it
	if ttd.tree.Nodes[nodeIdx] >= 0 && ttd.tree.Nodes[nodeIdx] < threshold {
		return ttd.tree.Nodes[nodeIdx]
	}

	// Get current state (lower bound on value)
	state := ttd.tree.States[nodeIdx]

	// Read bits until we get a 1 or reach threshold
	for state < threshold {
		bit, err := bitReader()
		if err != nil {
			// No more bits available - return current state as lower bound
			return state
		}

		if bit == 1 {
			// Got a 1 - value is exactly 'state'
			ttd.tree.Nodes[nodeIdx] = state
			ttd.tree.States[nodeIdx] = state
			return state
		}

		// Got a 0 - value is > state, increment state
		state++
		ttd.tree.States[nodeIdx] = state
	}

	// Reached threshold - value is >= threshold
	// Don't set Nodes[nodeIdx] since we don't know exact value yet
	return state
}

// DecodeInclusion decodes the inclusion tag tree for a code-block
// Returns:
//   - included: true if code-block is included in this packet
//   - layerIndex: the layer in which the code-block first appears (-1 if not included)
func (ttd *TagTreeDecoder) DecodeInclusion(cbX, cbY, currentLayer int, bitReader func() (int, error)) (bool, int, error) {
	// Decode tag tree with threshold = currentLayer + 1
	value := ttd.decodeNode(cbY*ttd.tree.Width+cbX, currentLayer+1, bitReader)

	if value <= currentLayer {
		// Code-block is included, first appears in layer 'value'
		return true, value, nil
	}

	// Code-block not included in this layer
	return false, -1, nil
}

// DecodeZeroBitPlanes decodes the zero bit-planes tag tree for a code-block
// This is only used when a code-block is first included
//
// Returns the number of missing MSB bit-planes
func (ttd *TagTreeDecoder) DecodeZeroBitPlanes(cbX, cbY int, bitReader func() (int, error)) (int, error) {
	// For zero bit-planes, we decode until we get a 1 bit
	// The value represents the number of missing MSB bit-planes

	leafIdx := cbY*ttd.tree.Width + cbX

	// Check if already decoded
	if ttd.tree.Nodes[leafIdx] >= 0 {
		return ttd.tree.Nodes[leafIdx], nil
	}

	// Decode with no threshold (decode until we get exact value)
	value := 0
	for {
		bit, err := bitReader()
		if err != nil {
			// Error reading bit
			return value, err
		}

		if bit == 1 {
			// Got exact value
			ttd.tree.Nodes[leafIdx] = value
			ttd.tree.States[leafIdx] = value
			return value, nil
		}

		// Got 0, increment
		value++
		ttd.tree.States[leafIdx] = value

		// Sanity check - prevent infinite loop
		if value > 31 {
			// Maximum 31 zero bit-planes for 32-bit coefficients
			ttd.tree.Nodes[leafIdx] = value
			return value, nil
		}
	}
}

// Reset resets the tag tree decoder state
func (ttd *TagTreeDecoder) Reset() {
	ttd.tree.Reset()
}

// GetTree returns the underlying tag tree
func (ttd *TagTreeDecoder) GetTree() *TagTree {
	return ttd.tree
}
