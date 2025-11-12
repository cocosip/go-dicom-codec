package t2

import (
	"fmt"
)

// TagTree represents a hierarchical data structure for encoding/decoding
// inclusion and zero bitplane information in JPEG 2000 packet headers.
// Implementation follows ISO/IEC 15444-1 Section B.10.2
type TagTree struct {
	width   int       // Width of the leaf level (number of code-blocks)
	height  int       // Height of the leaf level
	levels  int       // Number of levels in the tree
	nodes   [][]int   // Node values at each level [level][index]
	states  [][]int   // Node states (decoded value so far) [level][index]
}

// NewTagTree creates a new tag tree with the given leaf dimensions
func NewTagTree(width, height int) *TagTree {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}

	tt := &TagTree{
		width:  width,
		height: height,
	}

	// Calculate number of levels needed
	// Each level reduces dimensions by half (rounded up)
	levels := 0
	w, h := width, height
	for w > 1 || h > 1 {
		levels++
		w = (w + 1) / 2
		h = (h + 1) / 2
	}
	levels++ // Include root level
	tt.levels = levels

	// Allocate nodes and states for each level
	tt.nodes = make([][]int, levels)
	tt.states = make([][]int, levels)

	w, h = width, height
	for i := 0; i < levels; i++ {
		size := w * h
		tt.nodes[i] = make([]int, size)
		tt.states[i] = make([]int, size)

		// Initialize all states to 0 (no bits decoded yet)
		for j := 0; j < size; j++ {
			tt.states[i][j] = 0
		}

		// Move to next level (parent level)
		w = (w + 1) / 2
		h = (h + 1) / 2
	}

	return tt
}

// BitReader interface for reading bits from packet header
type BitReader interface {
	ReadBit() (int, error)
}

// Decode decodes the tag tree value for the specified leaf position (x, y)
// up to the threshold value. Returns the decoded value.
// The threshold parameter indicates we only need to know if value < threshold.
func (tt *TagTree) Decode(br BitReader, x, y, threshold int) (int, error) {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return 0, fmt.Errorf("tag tree position out of bounds: (%d,%d) not in [0,%d)x[0,%d)",
			x, y, tt.width, tt.height)
	}

	// Start from root level and work down to leaf
	level := tt.levels - 1
	levelWidth := (tt.width + (1 << level) - 1) >> level

	// Position at root
	px, py := 0, 0

	// Decode from root to leaf
	for level >= 0 {
		// Get node index at current level
		idx := py*levelWidth + px

		if idx >= len(tt.states[level]) {
			// Out of bounds - use default value
			value := 0
			if level > 0 {
				// Move to child level
				level--
				levelWidth = (tt.width + (1 << level) - 1) >> level
				px = (x >> level)
				py = (y >> level)
			} else {
				// At leaf level
				return value, nil
			}
			continue
		}

		// Decode bits until state[idx] >= threshold or we read a 1 bit
		for tt.states[level][idx] < threshold {
			bit, err := br.ReadBit()
			if err != nil {
				// End of stream - return current state
				return tt.states[level][idx], nil
			}

			if bit == 0 {
				// 0 bit means value is at least states[idx]+1
				tt.states[level][idx]++
			} else {
				// 1 bit means value equals states[idx]
				break
			}
		}

		// Update node value
		tt.nodes[level][idx] = tt.states[level][idx]

		if level == 0 {
			// At leaf level - return final value
			return tt.states[level][idx], nil
		}

		// Move to child level
		level--
		levelWidth = (tt.width + (1 << level) - 1) >> level

		// Calculate child position
		px = (x >> level)
		py = (y >> level)
	}

	return 0, nil
}

// GetValue returns the current decoded value at position (x, y)
// without reading any additional bits
func (tt *TagTree) GetValue(x, y int) int {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return 0
	}

	idx := y*tt.width + x
	if idx >= len(tt.states[0]) {
		return 0
	}

	return tt.states[0][idx]
}

// SetValue sets the value at position (x, y) (used during encoding)
func (tt *TagTree) SetValue(x, y, value int) {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return
	}

	idx := y*tt.width + x
	if idx >= len(tt.nodes[0]) {
		return
	}

	tt.nodes[0][idx] = value
	tt.states[0][idx] = value
}

// Reset resets all node states to 0 (for decoding a new packet)
func (tt *TagTree) Reset() {
	for level := 0; level < tt.levels; level++ {
		for i := range tt.states[level] {
			tt.states[level][i] = 0
		}
	}
}

// TagTreeDecoder is an alias for TagTree for backward compatibility
type TagTreeDecoder = TagTree

// NewTagTreeDecoder creates a new tag tree decoder (alias for backward compatibility)
func NewTagTreeDecoder(tree *TagTree) *TagTreeDecoder {
	return tree
}

// DecodeInclusion decodes code-block inclusion information from tag tree
// Returns (included, firstLayer, error)
func (tt *TagTree) DecodeInclusion(x, y, currentLayer int, readBit func() (int, error)) (bool, int, error) {
	// Create a bit reader wrapper
	br := &bitReaderFunc{readBit: readBit}

	// Decode tag tree value up to currentLayer+1
	value, err := tt.Decode(br, x, y, currentLayer+1)
	if err != nil {
		return false, 0, err
	}

	// If value > currentLayer, code-block is not included in this layer
	if value > currentLayer {
		return false, value, nil
	}

	// Code-block is included, value is the first layer
	return true, value, nil
}

// DecodeZeroBitPlanes decodes the number of zero bit-planes using tag tree
func (tt *TagTree) DecodeZeroBitPlanes(x, y int, readBit func() (int, error)) (int, error) {
	// Create a bit reader wrapper
	br := &bitReaderFunc{readBit: readBit}

	// Decode tag tree value (no threshold, read until completion)
	zbp, err := tt.Decode(br, x, y, 32) // 32 is arbitrary high threshold
	if err != nil {
		return 0, err
	}

	return zbp, nil
}

// bitReaderFunc wraps a function to implement BitReader interface
type bitReaderFunc struct {
	readBit func() (int, error)
}

func (br *bitReaderFunc) ReadBit() (int, error) {
	return br.readBit()
}

// PacketBitReader wraps a byte slice and provides bit-by-bit reading
type PacketBitReader struct {
	data   []byte
	offset int  // Byte offset
	bitPos int  // Bit position within current byte (0-7, MSB first)
}

// NewPacketBitReader creates a new bit reader
func NewPacketBitReader(data []byte) *PacketBitReader {
	return &PacketBitReader{
		data:   data,
		offset: 0,
		bitPos: 0,
	}
}

// ReadBit reads a single bit (MSB first)
func (pbr *PacketBitReader) ReadBit() (int, error) {
	if pbr.offset >= len(pbr.data) {
		return 0, fmt.Errorf("end of packet data")
	}

	// Read bit at current position (MSB = bit 7)
	bit := int((pbr.data[pbr.offset] >> (7 - pbr.bitPos)) & 1)

	// Advance to next bit
	pbr.bitPos++
	if pbr.bitPos >= 8 {
		pbr.bitPos = 0
		pbr.offset++
	}

	return bit, nil
}

// ReadBits reads n bits and returns them as an integer (MSB first)
func (pbr *PacketBitReader) ReadBits(n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 32 {
		return 0, fmt.Errorf("cannot read more than 32 bits at once")
	}

	value := 0
	for i := 0; i < n; i++ {
		bit, err := pbr.ReadBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | bit
	}

	return value, nil
}

// ByteAlign aligns to the next byte boundary
func (pbr *PacketBitReader) ByteAlign() {
	if pbr.bitPos != 0 {
		pbr.bitPos = 0
		pbr.offset++
	}
}

// Position returns the current byte offset and bit position
func (pbr *PacketBitReader) Position() (byteOffset, bitPos int) {
	return pbr.offset, pbr.bitPos
}

// Seek sets the position to the specified byte offset
func (pbr *PacketBitReader) Seek(offset int) {
	pbr.offset = offset
	pbr.bitPos = 0
}

// Remaining returns the number of bytes remaining
func (pbr *PacketBitReader) Remaining() int {
	remaining := len(pbr.data) - pbr.offset
	if pbr.bitPos > 0 {
		remaining-- // Current byte is partially consumed
	}
	if remaining < 0 {
		return 0
	}
	return remaining
}
