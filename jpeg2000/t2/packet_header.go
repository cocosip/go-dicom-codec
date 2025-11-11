package t2

import (
	"fmt"
	"io"
)

// PacketHeaderParser parses JPEG 2000 packet headers
// Reference: ISO/IEC 15444-1:2019 Annex B.10
type PacketHeaderParser struct {
	data   []byte
	pos    int
	bitPos int // Current bit position within byte (0-7)

	// Tag trees for this precinct
	inclTagTree *TagTreeDecoder // Inclusion tag tree
	zbpTagTree  *TagTreeDecoder // Zero bit-plane tag tree

	// Precinct dimensions (in code-blocks)
	numCBX int
	numCBY int

	// Current layer being decoded
	currentLayer int

	// Code-block state (persists across packets/layers)
	codeBlockStates []*CodeBlockState
}

// CodeBlockState tracks the state of a code-block across multiple packets
type CodeBlockState struct {
	Included       bool // Has been included in any previous packet
	FirstLayer     int  // Layer in which it was first included
	ZeroBitPlanes  int  // Number of MSB zero bit-planes
	NumPassesTotal int  // Total number of passes decoded
	DataAccum      []byte // Accumulated compressed data
}

// NewPacketHeaderParser creates a new packet header parser
func NewPacketHeaderParser(data []byte, numCBX, numCBY int) *PacketHeaderParser {
	// Create tag trees
	inclTree := NewTagTree(numCBX, numCBY)
	zbpTree := NewTagTree(numCBX, numCBY)

	// Initialize code-block states
	numCB := numCBX * numCBY
	cbStates := make([]*CodeBlockState, numCB)
	for i := range cbStates {
		cbStates[i] = &CodeBlockState{
			Included:      false,
			FirstLayer:    -1,
			ZeroBitPlanes: 0,
			DataAccum:     make([]byte, 0),
		}
	}

	return &PacketHeaderParser{
		data:            data,
		pos:             0,
		bitPos:          0,
		inclTagTree:     NewTagTreeDecoder(inclTree),
		zbpTagTree:      NewTagTreeDecoder(zbpTree),
		numCBX:          numCBX,
		numCBY:          numCBY,
		codeBlockStates: cbStates,
		currentLayer:    0,
	}
}

// ParseHeader parses a single packet header
// Returns the packet information including code-block contributions
func (php *PacketHeaderParser) ParseHeader() (*Packet, error) {
	packet := &Packet{
		LayerIndex:     php.currentLayer,
		CodeBlockIncls: make([]CodeBlockIncl, 0),
	}

	// Read packet present bit (SOP marker bit in some implementations)
	// For now, assume packet is present if we have data
	if php.pos >= len(php.data) {
		packet.HeaderPresent = false
		return packet, nil
	}

	// Read empty packet flag (1 bit)
	emptyBit, err := php.readBit()
	if err != nil {
		return nil, fmt.Errorf("failed to read empty packet bit: %w", err)
	}

	packet.HeaderPresent = (emptyBit == 1)

	if !packet.HeaderPresent {
		// Empty packet
		return packet, nil
	}

	// Parse code-block inclusion information
	headerStart := php.pos

	// For each code-block in the precinct
	for cby := 0; cby < php.numCBY; cby++ {
		for cbx := 0; cbx < php.numCBX; cbx++ {
			cbIdx := cby*php.numCBX + cbx
			cbState := php.codeBlockStates[cbIdx]

			var cbIncl CodeBlockIncl

			// Decode inclusion
			included, firstLayer, err := php.inclTagTree.DecodeInclusion(
				cbx, cby, php.currentLayer, php.readBit)
			if err != nil {
				return nil, fmt.Errorf("failed to decode inclusion for CB[%d,%d]: %w", cbx, cby, err)
			}

			cbIncl.Included = included

			if !included {
				// Code-block not included in this packet
				continue
			}

			// Check if this is first inclusion
			if !cbState.Included {
				// First time this code-block is included
				cbIncl.FirstInclusion = true
				cbState.Included = true
				cbState.FirstLayer = firstLayer

				// Decode number of zero bit-planes
				zbp, err := php.zbpTagTree.DecodeZeroBitPlanes(cbx, cby, php.readBit)
				if err != nil {
					return nil, fmt.Errorf("failed to decode ZBP for CB[%d,%d]: %w", cbx, cby, err)
				}
				cbState.ZeroBitPlanes = zbp
			} else {
				cbIncl.FirstInclusion = false
			}

			// Decode number of coding passes
			numPasses, err := php.decodeNumPasses()
			if err != nil {
				return nil, fmt.Errorf("failed to decode num passes for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.NumPasses = numPasses
			cbState.NumPassesTotal += numPasses

			// Decode length of code-block contribution
			dataLength, err := php.decodeDataLength(numPasses)
			if err != nil {
				return nil, fmt.Errorf("failed to decode data length for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.DataLength = dataLength

			packet.CodeBlockIncls = append(packet.CodeBlockIncls, cbIncl)
		}
	}

	// Align to byte boundary
	php.alignToByte()

	// Header ends here
	packet.Header = php.data[headerStart:php.pos]

	return packet, nil
}

// readBit reads a single bit from the bitstream
func (php *PacketHeaderParser) readBit() (int, error) {
	if php.pos >= len(php.data) {
		return 0, io.EOF
	}

	bit := int((php.data[php.pos] >> (7 - php.bitPos)) & 1)
	php.bitPos++

	if php.bitPos >= 8 {
		php.bitPos = 0
		php.pos++
	}

	return bit, nil
}

// decodeNumPasses decodes the number of coding passes
// Reference: ISO/IEC 15444-1:2019 Annex B.10.7
func (php *PacketHeaderParser) decodeNumPasses() (int, error) {
	// Number of passes is encoded with a variable-length code:
	// 0       → 1 pass
	// 10      → 2 passes
	// 110     → 3-5 passes (+ 2-bit value)
	// 111     → 6-37 passes (+ 5-bit value)
	// etc.

	bit1, err := php.readBit()
	if err != nil {
		return 0, err
	}

	if bit1 == 0 {
		return 1, nil
	}

	bit2, err := php.readBit()
	if err != nil {
		return 0, err
	}

	if bit2 == 0 {
		return 2, nil
	}

	bit3, err := php.readBit()
	if err != nil {
		return 0, err
	}

	if bit3 == 0 {
		// 110 + 2 bits → 3-5 passes (value 0-2 maps to 3-5)
		val, err := php.readBits(2)
		if err != nil {
			return 0, err
		}
		return 3 + val, nil
	}

	// 111 + 5 bits → 6-37 passes (value 0-31 maps to 6-37)
	val, err := php.readBits(5)
	if err != nil {
		return 0, err
	}
	return 6 + val, nil
}

// decodeDataLength decodes the length of code-block contribution
// Reference: ISO/IEC 15444-1:2019 Annex B.10.8
func (php *PacketHeaderParser) decodeDataLength(numPasses int) (int, error) {
	// Length is encoded incrementally:
	// Read bits until we determine the length

	// Simple implementation: read length with variable bit encoding
	// Real implementation would use proper JPEG 2000 length encoding

	// For MVP, use a simplified encoding:
	// Read 8 bits for length < 256
	// Read 16 bits if first 8 bits = 0xFF

	firstByte, err := php.readBits(8)
	if err != nil {
		return 0, err
	}

	if firstByte < 0xFF {
		return firstByte, nil
	}

	// Length >= 255, read another 8 bits
	secondByte, err := php.readBits(8)
	if err != nil {
		return 0, err
	}

	return 255 + secondByte, nil
}

// readBits reads multiple bits from the bitstream
func (php *PacketHeaderParser) readBits(n int) (int, error) {
	if n <= 0 || n > 32 {
		return 0, fmt.Errorf("invalid number of bits: %d", n)
	}

	result := 0
	for i := 0; i < n; i++ {
		bit, err := php.readBit()
		if err != nil {
			return 0, err
		}
		result = (result << 1) | bit
	}

	return result, nil
}

// alignToByte aligns the bit position to the next byte boundary
func (php *PacketHeaderParser) alignToByte() {
	if php.bitPos != 0 {
		php.pos++
		php.bitPos = 0
	}
}

// SetLayer sets the current layer index for decoding
func (php *PacketHeaderParser) SetLayer(layer int) {
	php.currentLayer = layer
}

// Reset resets the parser to the beginning
func (php *PacketHeaderParser) Reset() {
	php.pos = 0
	php.bitPos = 0
	php.currentLayer = 0
	php.inclTagTree.Reset()
	php.zbpTagTree.Reset()

	// Reset code-block states
	for _, cbState := range php.codeBlockStates {
		cbState.Included = false
		cbState.FirstLayer = -1
		cbState.ZeroBitPlanes = 0
		cbState.NumPassesTotal = 0
		cbState.DataAccum = cbState.DataAccum[:0]
	}
}

// Position returns the current byte position
func (php *PacketHeaderParser) Position() int {
	return php.pos
}
