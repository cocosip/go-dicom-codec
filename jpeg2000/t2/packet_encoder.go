package t2

import (
	"bytes"
	"fmt"
	"sort"
)

// PacketEncoder encodes JPEG 2000 packets
// Reference: ISO/IEC 15444-1:2019 Annex B
type PacketEncoder struct {
	// Encoding parameters
	numComponents  int
	numLayers      int
	numResolutions int
	progression    ProgressionOrder

	// Precinct information
	precincts map[int]map[int]map[int][]*Precinct // [component][resolution][precinct]

	// Geometry parameters (for position-based progression)
	tileX0, tileY0  int
	tileX1, tileY1  int
	compDx          []int
	compDy          []int
	compBounds      []componentBounds
	precinctWidths  []int
	precinctHeights []int

	// Output buffer
	packets []Packet
}

// NewPacketEncoder creates a new packet encoder
func NewPacketEncoder(numComponents, numLayers, numResolutions int, progression ProgressionOrder) *PacketEncoder {
	return &PacketEncoder{
		numComponents:  numComponents,
		numLayers:      numLayers,
		numResolutions: numResolutions,
		progression:    progression,
		precincts:      make(map[int]map[int]map[int][]*Precinct),
	}
}

// SetImageDimensions sets the tile bounds for progression ordering.
func (pe *PacketEncoder) SetImageDimensions(width, height int) {
	pe.tileX0 = 0
	pe.tileY0 = 0
	pe.tileX1 = width
	pe.tileY1 = height
}

// SetTileBounds sets the tile bounds in reference grid coordinates.
func (pe *PacketEncoder) SetTileBounds(x0, y0, x1, y1 int) {
	pe.tileX0 = x0
	pe.tileY0 = y0
	pe.tileX1 = x1
	pe.tileY1 = y1
}

// SetComponentSampling sets the sampling factors for a component.
func (pe *PacketEncoder) SetComponentSampling(component, dx, dy int) {
	if component < 0 || component >= pe.numComponents {
		return
	}
	if pe.compDx == nil {
		pe.compDx = make([]int, pe.numComponents)
	}
	if pe.compDy == nil {
		pe.compDy = make([]int, pe.numComponents)
	}
	pe.compDx[component] = dx
	pe.compDy[component] = dy
}

// SetComponentBounds sets the tile-component bounds for a component.
func (pe *PacketEncoder) SetComponentBounds(component, x0, y0, x1, y1 int) {
	if component < 0 || component >= pe.numComponents {
		return
	}
	if pe.compBounds == nil {
		pe.compBounds = make([]componentBounds, pe.numComponents)
	}
	pe.compBounds[component] = componentBounds{x0: x0, y0: y0, x1: x1, y1: y1}
}

// SetPrecinctSizes sets per-resolution precinct sizes (in pixels).
func (pe *PacketEncoder) SetPrecinctSizes(widths, heights []int) {
	pe.precinctWidths = append([]int(nil), widths...)
	pe.precinctHeights = append([]int(nil), heights...)
}

// AddCodeBlock adds a code-block to a precinct
func (pe *PacketEncoder) AddCodeBlock(component, resolution, precinctIdx int, codeBlock *PrecinctCodeBlock) {
	// Ensure maps exist
	if pe.precincts[component] == nil {
		pe.precincts[component] = make(map[int]map[int][]*Precinct)
	}
	if pe.precincts[component][resolution] == nil {
		pe.precincts[component][resolution] = make(map[int][]*Precinct)
	}
	if pe.precincts[component][resolution][precinctIdx] == nil {
		pe.precincts[component][resolution][precinctIdx] = []*Precinct{}
	}

	// Add code-block to precinct (one precinct per band)
	var precinct *Precinct
	for _, p := range pe.precincts[component][resolution][precinctIdx] {
		if p.SubbandIdx == codeBlock.Band {
			precinct = p
			break
		}
	}
	if precinct == nil {
		precinct = &Precinct{
			Index:      precinctIdx,
			SubbandIdx: codeBlock.Band,
			CodeBlocks: make([]*PrecinctCodeBlock, 0),
		}
		pe.precincts[component][resolution][precinctIdx] = append(
			pe.precincts[component][resolution][precinctIdx],
			precinct,
		)
	}

	// Add code-block to precinct
	precinct.CodeBlocks = append(precinct.CodeBlocks, codeBlock)

	// Update grid dimensions based on code-block position
	if codeBlock.CBX+1 > precinct.NumCodeBlocksX {
		precinct.NumCodeBlocksX = codeBlock.CBX + 1
	}
	if codeBlock.CBY+1 > precinct.NumCodeBlocksY {
		precinct.NumCodeBlocksY = codeBlock.CBY + 1
	}
}

// EncodePackets encodes all packets according to progression order
func (pe *PacketEncoder) EncodePackets() ([]Packet, error) {
	pe.packets = make([]Packet, 0)

	switch pe.progression {
	case ProgressionLRCP:
		return pe.encodeLRCP()
	case ProgressionRLCP:
		return pe.encodeRLCP()
	case ProgressionRPCL:
		return pe.encodeRPCL()
	case ProgressionPCRL:
		return pe.encodePCRL()
	case ProgressionCPRL:
		return pe.encodeCPRL()
	default:
		return nil, fmt.Errorf("unsupported progression order: %v", pe.progression)
	}
}

// encodeLRCP encodes packets in Layer-Resolution-Component-Position order
func (pe *PacketEncoder) encodeLRCP() ([]Packet, error) {
	for layer := 0; layer < pe.numLayers; layer++ {
		for res := 0; res < pe.numResolutions; res++ {
			for comp := 0; comp < pe.numComponents; comp++ {
				// Get precincts for this component/resolution
				if pe.precincts[comp] == nil || pe.precincts[comp][res] == nil {
					continue
				}

				for _, precinctIdx := range pe.sortedPrecincts(comp, res) {
					precincts := pe.getPrecincts(comp, res, precinctIdx)
					if len(precincts) == 0 {
						continue
					}
					packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precincts)
					if err != nil {
						return nil, fmt.Errorf("failed to encode packet (L=%d,R=%d,C=%d,P=%d): %w",
							layer, res, comp, precinctIdx, err)
					}
					pe.packets = append(pe.packets, packet)
				}
			}
		}
	}

	return pe.packets, nil
}

// encodeRLCP encodes packets in Resolution-Layer-Component-Position order
func (pe *PacketEncoder) encodeRLCP() ([]Packet, error) {
	for res := 0; res < pe.numResolutions; res++ {
		for layer := 0; layer < pe.numLayers; layer++ {
			for comp := 0; comp < pe.numComponents; comp++ {
				// Get precincts for this component/resolution
				if pe.precincts[comp] == nil || pe.precincts[comp][res] == nil {
					continue
				}

				for _, precinctIdx := range pe.sortedPrecincts(comp, res) {
					precincts := pe.getPrecincts(comp, res, precinctIdx)
					if len(precincts) == 0 {
						continue
					}
					packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precincts)
					if err != nil {
						return nil, fmt.Errorf("failed to encode packet (R=%d,L=%d,C=%d,P=%d): %w",
							res, layer, comp, precinctIdx, err)
					}
					pe.packets = append(pe.packets, packet)
				}
			}
		}
	}

	return pe.packets, nil
}

// encodeRPCL encodes packets in Resolution-Position-Component-Layer order
func (pe *PacketEncoder) encodeRPCL() ([]Packet, error) {
	posMaps := pe.buildPositionMaps()
	for res := 0; res < pe.numResolutions; res++ {
		positions := posMaps.byRes[res]
		for _, pos := range positions {
			for comp := 0; comp < pe.numComponents; comp++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pe.numLayers; layer++ {
					precincts := pe.getPrecincts(comp, res, precinctIdx)
					if len(precincts) == 0 {
						continue
					}
					packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precincts)
					if err != nil {
						return nil, fmt.Errorf("failed to encode packet (R=%d,P=%d,C=%d,L=%d): %w",
							res, precinctIdx, comp, layer, err)
					}
					pe.packets = append(pe.packets, packet)
				}
			}
		}
	}
	return pe.packets, nil
}

// encodePCRL encodes packets in Position-Component-Resolution-Layer order
func (pe *PacketEncoder) encodePCRL() ([]Packet, error) {
	posMaps := pe.buildPositionMaps()
	for _, pos := range posMaps.all {
		for comp := 0; comp < pe.numComponents; comp++ {
			for res := 0; res < pe.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pe.numLayers; layer++ {
					precincts := pe.getPrecincts(comp, res, precinctIdx)
					if len(precincts) == 0 {
						continue
					}
					packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precincts)
					if err != nil {
						return nil, fmt.Errorf("failed to encode packet (P=%d,C=%d,R=%d,L=%d): %w",
							precinctIdx, comp, res, layer, err)
					}
					pe.packets = append(pe.packets, packet)
				}
			}
		}
	}
	return pe.packets, nil
}

// encodeCPRL encodes packets in Component-Position-Resolution-Layer order
func (pe *PacketEncoder) encodeCPRL() ([]Packet, error) {
	posMaps := pe.buildPositionMaps()
	for comp := 0; comp < pe.numComponents; comp++ {
		positions := posMaps.byComp[comp]
		for _, pos := range positions {
			for res := 0; res < pe.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pe.numLayers; layer++ {
					precincts := pe.getPrecincts(comp, res, precinctIdx)
					if len(precincts) == 0 {
						continue
					}
					packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precincts)
					if err != nil {
						return nil, fmt.Errorf("failed to encode packet (C=%d,P=%d,R=%d,L=%d): %w",
							comp, precinctIdx, res, layer, err)
					}
					pe.packets = append(pe.packets, packet)
				}
			}
		}
	}
	return pe.packets, nil
}

// Helpers to keep precinct traversal deterministic (sorted keys)
func (pe *PacketEncoder) sortedPrecincts(comp, res int) []int {
	keys := make([]int, 0)
	if pe.precincts[comp] == nil || pe.precincts[comp][res] == nil {
		return keys
	}
	for k := range pe.precincts[comp][res] {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func (pe *PacketEncoder) sortedPrecinctsAllComponents(res int) []int {
	set := make(map[int]struct{})
	for comp := 0; comp < pe.numComponents; comp++ {
		if pe.precincts[comp] == nil || pe.precincts[comp][res] == nil {
			continue
		}
		for k := range pe.precincts[comp][res] {
			set[k] = struct{}{}
		}
	}
	return sortKeys(set)
}

func (pe *PacketEncoder) sortedPrecinctsAll() []int {
	set := make(map[int]struct{})
	for comp := 0; comp < pe.numComponents; comp++ {
		if pe.precincts[comp] == nil {
			continue
		}
		for _, resMap := range pe.precincts[comp] {
			for k := range resMap {
				set[k] = struct{}{}
			}
		}
	}
	return sortKeys(set)
}

func (pe *PacketEncoder) getPrecincts(comp, res, precinctIdx int) []*Precinct {
	if pe.precincts[comp] == nil || pe.precincts[comp][res] == nil {
		return nil
	}
	return pe.precincts[comp][res][precinctIdx]
}

func sortKeys(m map[int]struct{}) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func (pe *PacketEncoder) componentSamplingFor(component int) (int, int) {
	dx := 1
	dy := 1
	if component >= 0 && component < len(pe.compDx) && pe.compDx[component] > 0 {
		dx = pe.compDx[component]
	}
	if component >= 0 && component < len(pe.compDy) && pe.compDy[component] > 0 {
		dy = pe.compDy[component]
	}
	return dx, dy
}

func (pe *PacketEncoder) componentBoundsFor(component int) componentBounds {
	if component >= 0 && component < len(pe.compBounds) {
		b := pe.compBounds[component]
		if b.x1 != 0 || b.y1 != 0 {
			return b
		}
	}

	dx, dy := pe.componentSamplingFor(component)
	x0 := ceilDiv(pe.tileX0, dx)
	y0 := ceilDiv(pe.tileY0, dy)
	x1 := ceilDiv(pe.tileX1, dx)
	y1 := ceilDiv(pe.tileY1, dy)
	return componentBounds{x0: x0, y0: y0, x1: x1, y1: y1}
}

func (pe *PacketEncoder) precinctSizeForResolution(resolution int) (int, int) {
	pw := 0
	ph := 0
	if resolution >= 0 {
		if resolution < len(pe.precinctWidths) {
			pw = pe.precinctWidths[resolution]
		}
		if resolution < len(pe.precinctHeights) {
			ph = pe.precinctHeights[resolution]
		}
	}
	if pw == 0 {
		pw = 1 << 15
	}
	if ph == 0 {
		ph = 1 << 15
	}
	return pw, ph
}

func (pe *PacketEncoder) buildPositionMaps() *positionMaps {
	return buildPositionMaps(positionInputs{
		numComponents:     pe.numComponents,
		numResolutions:    pe.numResolutions,
		precinctIndices:   pe.sortedPrecincts,
		componentBounds:   pe.componentBoundsFor,
		componentSampling: pe.componentSamplingFor,
		precinctSize:      pe.precinctSizeForResolution,
	})
}

// encodePacket encodes a single packet
func (pe *PacketEncoder) encodePacket(layer, resolution, component, precinctIdx int, precincts []*Precinct) (Packet, error) {
	packet := Packet{
		LayerIndex:      layer,
		ResolutionLevel: resolution,
		ComponentIndex:  component,
		PrecinctIndex:   precinctIdx,
		HeaderPresent:   false,
	}

	ordered := orderPrecinctsByBand(precincts, resolution)

	// Encode packet header with tag-tree encoding (aligned with OpenJPEG)
	// This properly handles PassLengths and TERMALL metadata for multi-layer encoding
	header, cbIncls, err := pe.encodePacketHeaderWithTagTreeMulti(ordered, layer)
	if err != nil {
		return packet, fmt.Errorf("failed to encode packet header: %w", err)
	}
	packet.Header = header
	packet.CodeBlockIncls = cbIncls
	packet.HeaderPresent = len(header) > 0

	// Encode packet body (code-block contributions for this layer)
	body := &bytes.Buffer{}
	for i := range cbIncls {
		cbIncl := &cbIncls[i]
		if cbIncl.Included {
			body.Write(cbIncl.Data)
		}
	}
	packet.Body = body.Bytes()

	return packet, nil
}

func orderPrecinctsByBand(precincts []*Precinct, resolution int) []*Precinct {
	if len(precincts) == 0 {
		return nil
	}

	var bandOrder []int
	if resolution == 0 {
		bandOrder = []int{0}
	} else {
		bandOrder = []int{1, 2, 3}
	}

	ordered := make([]*Precinct, 0, len(precincts))
	for _, band := range bandOrder {
		for _, precinct := range precincts {
			if precinct != nil && precinct.SubbandIdx == band {
				ordered = append(ordered, precinct)
			}
		}
	}

	return ordered
}

// encodePacketHeader encodes a packet header
// This is a simplified implementation - full implementation would use tag trees
func (pe *PacketEncoder) encodePacketHeader(precinct *Precinct, layer int) ([]byte, []CodeBlockIncl, error) {
	header := &bytes.Buffer{}
	cbIncls := make([]CodeBlockIncl, 0)

	// Simplified header encoding:
	// For each code-block in precinct:
	//   - 1 bit: included (1) or not (0)
	//   - If included and first time:
	//     - encode zero bitplanes
	//     - encode number of passes
	//   - If included and not first time:
	//     - encode number of passes
	//   - encode data length

	bitBuf := newBitWriter(header)

	// DEBUG: Count code blocks with/without data
	cbWithData := 0
	cbWithoutData := 0
	for _, cb := range precinct.CodeBlocks {
		if cb.Data != nil && len(cb.Data) > 0 {
			cbWithData++
		} else {
			cbWithoutData++
		}
	}

	for _, cb := range precinct.CodeBlocks {
		// Determine if this code-block is included in this layer
		// Simplified: include all code-blocks with data
		included := cb.Data != nil && len(cb.Data) > 0
		firstIncl := !cb.Included

		cbIncl := CodeBlockIncl{
			Included:       included,
			FirstInclusion: firstIncl,
		}

		// Write inclusion bit
		if included {
			bitBuf.writeBit(1)
		} else {
			bitBuf.writeBit(0)
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		// If first inclusion, encode zero bitplanes
		if firstIncl {
			// Simplified: encode zero bitplanes directly (should use tag tree)
			zbp := cb.ZeroBitPlanes
			for zbp > 0 {
				bitBuf.writeBit(0)
				zbp--
			}
			bitBuf.writeBit(1) // Termination bit

			// Mark as included
			cb.Included = true
		}

		// Encode number of coding passes for this layer
		// Simplified: use all available passes
		numPasses := cb.NumPassesTotal
		cbIncl.NumPasses = numPasses

		// Encode number of passes using JPEG2000 standard encoding
		if err := encodeNumPasses(bitBuf, numPasses); err != nil {
			return nil, nil, fmt.Errorf("failed to encode number of passes: %w", err)
		}

		// Encode data length
		dataLen := len(cb.Data)
		cbIncl.DataLength = dataLen
		cbIncl.Data = cb.Data

		// Encode length (simplified - use fixed-length encoding for now)
		// In real implementation, would use variable-length encoding
		// Using 16-bit length for simplicity
		for i := 15; i >= 0; i-- {
			bit := (dataLen >> i) & 1
			bitBuf.writeBit(bit)
		}

		cbIncls = append(cbIncls, cbIncl)
	}

	// Flush remaining bits
	bitBuf.flush()

	return header.Bytes(), cbIncls, nil
}

// bitWriter helps with bit-level writing
type bitWriter struct {
	buf      *bytes.Buffer
	bitBuf   byte
	bitCount int
}

func newBitWriter(buf *bytes.Buffer) *bitWriter {
	return &bitWriter{buf: buf}
}

func (bw *bitWriter) writeBit(bit int) {
	if bit != 0 {
		bw.bitBuf |= 1 << (7 - bw.bitCount)
	}
	bw.bitCount++

	if bw.bitCount == 8 {
		bw.buf.WriteByte(bw.bitBuf)
		bw.bitBuf = 0
		bw.bitCount = 0
	}
}

// WriteBit writes a single bit (implements BitWriter interface)
func (bw *bitWriter) WriteBit(bit int) error {
	bw.writeBit(bit)
	return nil
}

// writeBits writes n bits from value (MSB first)
func (bw *bitWriter) writeBits(value, n int) {
	for i := n - 1; i >= 0; i-- {
		bit := (value >> i) & 1
		bw.writeBit(bit)
	}
}

func (bw *bitWriter) flush() {
	if bw.bitCount > 0 {
		bw.buf.WriteByte(bw.bitBuf)
		bw.bitBuf = 0
		bw.bitCount = 0
	}
}

// encodePacketHeaderLayered encodes a packet header for multi-layer support
// This version properly handles layer-specific pass allocation
func (pe *PacketEncoder) encodePacketHeaderLayered(precinct *Precinct, layer int, resolution int) ([]byte, []CodeBlockIncl, error) {
	header := &bytes.Buffer{}
	cbIncls := make([]CodeBlockIncl, 0)

	bitBuf := newBitWriter(header)

	includedCount := 0

	for _, cb := range precinct.CodeBlocks {
		// Determine if this code-block is included in this layer
		included := false
		newPasses := 0

		if cb.LayerData != nil && layer < len(cb.LayerData) {
			// Multi-layer data available
			if layer < len(cb.LayerPasses) {
				totalPasses := cb.LayerPasses[layer]
				prevPasses := 0
				if layer > 0 {
					prevPasses = cb.LayerPasses[layer-1]
				}
				newPasses = totalPasses - prevPasses
				included = newPasses > 0
			}
		} else {
			// Fallback: use old single-layer method
			hasData := cb.Data != nil && len(cb.Data) > 0
			included = hasData
			newPasses = cb.NumPassesTotal
		}

		firstIncl := !cb.Included && included

		cbIncl := CodeBlockIncl{
			Included:       included,
			FirstInclusion: firstIncl,
		}

		// Write inclusion bit
		if included {
			bitBuf.writeBit(1)
			includedCount++
		} else {
			bitBuf.writeBit(0)
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		// If first inclusion, encode zero bitplanes
		if firstIncl {
			zbp := cb.ZeroBitPlanes
			for zbp > 0 {
				bitBuf.writeBit(0)
				zbp--
			}
			bitBuf.writeBit(1) // Termination bit
			cb.Included = true
		}

		// Encode number of coding passes for this layer
		cbIncl.NumPasses = newPasses

		// Encode number of passes (simplified unary code)
		for i := 0; i < newPasses; i++ {
			if i < newPasses-1 {
				bitBuf.writeBit(0)
			} else {
				bitBuf.writeBit(1)
			}
		}

		// Get data for this layer
		var layerData []byte

		if cb.LayerData != nil && layer < len(cb.LayerData) {
			// Multi-layer: use pre-calculated layer data (incremental)
			layerData = cb.LayerData[layer]
		} else {
			// Fallback to single-layer data
			layerData = cb.Data
		}

		dataLen := len(layerData)
		cbIncl.Data = layerData

		// Copy PassLengths for multi-layer support
		// In multi-layer mode, extract only the pass lengths for this layer
		if cb.LayerData != nil && layer < len(cb.LayerPasses) {
			// Get the range of passes for this layer
			totalPasses := cb.LayerPasses[layer]
			prevPasses := 0
			if layer > 0 {
				prevPasses = cb.LayerPasses[layer-1]
			}

			// Extract pass lengths for this layer's passes
			// Convert from absolute to relative (within this layer's data)
			if totalPasses <= len(cb.PassLengths) {
				layerPassLengths := make([]int, totalPasses-prevPasses)
				baseOffset := 0
				if prevPasses > 0 && prevPasses <= len(cb.PassLengths) {
					baseOffset = cb.PassLengths[prevPasses-1]
				}
				for i := prevPasses; i < totalPasses && i < len(cb.PassLengths); i++ {
					layerPassLengths[i-prevPasses] = cb.PassLengths[i] - baseOffset
				}
				cbIncl.PassLengths = layerPassLengths

			}
		} else {
			// Single layer: use all pass lengths as-is
			cbIncl.PassLengths = cb.PassLengths
		}
		cbIncl.UseTERMALL = cb.UseTERMALL

		// Calculate total DataLength including metadata
		if cbIncl.UseTERMALL && len(cbIncl.PassLengths) > 0 {
			metadataBytes := 1 + len(cbIncl.PassLengths)*2
			cbIncl.DataLength = dataLen + metadataBytes
		} else {
			cbIncl.DataLength = dataLen
		}

		// Encode length (16-bit fixed for simplicity)
		// This encodes the TOTAL length (metadata + data) in TERMALL mode
		encodedLength := cbIncl.DataLength

		for i := 15; i >= 0; i-- {
			bit := (encodedLength >> i) & 1
			bitBuf.writeBit(bit)
		}

		cbIncls = append(cbIncls, cbIncl)
	}

	// Flush remaining bits
	bitBuf.flush()

	headerBytes := header.Bytes()

	return headerBytes, cbIncls, nil
}

// applyByteStuffing applies JPEG 2000 byte-stuffing to code-block data
// Any 0xFF byte must be followed by 0x00 to distinguish it from markers
func applyByteStuffing(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// Count 0xFF bytes to pre-allocate
	ffCount := 0
	for _, b := range data {
		if b == 0xFF {
			ffCount++
		}
	}

	if ffCount == 0 {
		return data // No stuffing needed
	}

	// Allocate result buffer
	result := make([]byte, len(data)+ffCount)
	j := 0
	for _, b := range data {
		result[j] = b
		j++
		if b == 0xFF {
			result[j] = 0x00 // Stuff byte
			j++
		}
	}

	return result
}

// GetPackets returns the encoded packets
func (pe *PacketEncoder) GetPackets() []Packet {
	return pe.packets
}
