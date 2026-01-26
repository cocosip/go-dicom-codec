package t2

import (
	"bytes"
	"fmt"
	"sort"
)

// PacketDecoder decodes JPEG 2000 packets
// Reference: ISO/IEC 15444-1:2019 Annex B
type PacketDecoder struct {
	// Input bitstream
	data   []byte
	offset int

	// Decoding parameters
	numComponents  int
	numLayers      int
	numResolutions int
	progression    ProgressionOrder
	imageWidth     int
	imageHeight    int
	cbWidth        int
	cbHeight       int
	numLevels      int
	codeBlockStyle uint8 // Code-block style (for TERMALL detection)

	// Per-component geometry (tile-component bounds)
	compBounds []componentBounds
	compDx     []int
	compDy     []int

	// Precinct parameters
	precinctWidth   int // Precinct width (0 = default size of 2^15)
	precinctHeight  int // Precinct height (0 = default size of 2^15)
	precinctWidths  []int
	precinctHeights []int

	// Parsed packets
	packets []Packet

	// Multi-layer state tracking
	// Maps "component:resolution:cbIndex" -> true if code-block was included in a previous layer
	cbIncluded map[string]bool

	// Precinct -> code-block order mapping per resolution (mirrors encoder traversal)
	cbPrecinctOrder map[int]map[int]map[int][]int
	// Precinct -> code-block positions (CBX/CBY) in packet header order
	cbPrecinctPositions map[int]map[int]map[int]map[int][]cbPosition
	// Precinct -> code-block grid dimensions used for tag-tree decoding
	cbPrecinctDims map[int]map[int]map[int]map[int]cbGridDim

	// Persisted code-block states per component/resolution/precinct for tag-tree decoding
	cbStates map[string]*packetHeaderContext
}

type cbGridDim struct {
	numCBX int
	numCBY int
}

type packetHeaderContext struct {
	incl   *TagTreeDecoder
	zbp    *TagTreeDecoder
	states []*CodeBlockState
}

// NewPacketDecoder creates a new packet decoder
func NewPacketDecoder(data []byte, numComponents, numLayers, numResolutions int, progression ProgressionOrder, codeBlockStyle uint8) *PacketDecoder {
	return &PacketDecoder{
		data:                data,
		offset:              0,
		numComponents:       numComponents,
		numLayers:           numLayers,
		numResolutions:      numResolutions,
		progression:         progression,
		imageWidth:          0,  // Will be set later if needed
		imageHeight:         0,  // Will be set later if needed
		cbWidth:             64, // Default code-block size
		cbHeight:            64, // Default code-block size
		numLevels:           numResolutions - 1,
		codeBlockStyle:      codeBlockStyle,
		packets:             make([]Packet, 0),
		cbIncluded:          make(map[string]bool),
		compBounds:          make([]componentBounds, numComponents),
		cbPrecinctOrder:     make(map[int]map[int]map[int][]int),
		cbPrecinctPositions: make(map[int]map[int]map[int]map[int][]cbPosition),
		cbPrecinctDims:      make(map[int]map[int]map[int]map[int]cbGridDim),
	}
}

// SetImageDimensions sets the image and code-block dimensions
func (pd *PacketDecoder) SetImageDimensions(width, height, cbWidth, cbHeight int) {
	pd.imageWidth = width
	pd.imageHeight = height
	pd.cbWidth = cbWidth
	pd.cbHeight = cbHeight
	if pd.compBounds == nil {
		pd.compBounds = make([]componentBounds, pd.numComponents)
	}
	for i := range pd.compBounds {
		if pd.compBounds[i].x1 == 0 && pd.compBounds[i].y1 == 0 {
			pd.compBounds[i] = componentBounds{x0: 0, y0: 0, x1: width, y1: height}
		}
	}
}

// SetComponentBounds sets the tile-component bounds for a component.
func (pd *PacketDecoder) SetComponentBounds(component, x0, y0, x1, y1 int) {
	if component < 0 || component >= pd.numComponents {
		return
	}
	if pd.compBounds == nil {
		pd.compBounds = make([]componentBounds, pd.numComponents)
	}
	pd.compBounds[component] = componentBounds{x0: x0, y0: y0, x1: x1, y1: y1}
}

// SetComponentSampling sets the sampling factors for a component.
func (pd *PacketDecoder) SetComponentSampling(component, dx, dy int) {
	if component < 0 || component >= pd.numComponents {
		return
	}
	if pd.compDx == nil {
		pd.compDx = make([]int, pd.numComponents)
	}
	if pd.compDy == nil {
		pd.compDy = make([]int, pd.numComponents)
	}
	pd.compDx[component] = dx
	pd.compDy[component] = dy
}

func (pd *PacketDecoder) componentBoundsFor(component int) componentBounds {
	if component >= 0 && component < len(pd.compBounds) {
		b := pd.compBounds[component]
		if b.x1 != 0 || b.y1 != 0 {
			return b
		}
	}
	return componentBounds{x0: 0, y0: 0, x1: pd.imageWidth, y1: pd.imageHeight}
}

func (pd *PacketDecoder) componentSamplingFor(component int) (int, int) {
	dx := 1
	dy := 1
	if component >= 0 && component < len(pd.compDx) && pd.compDx[component] > 0 {
		dx = pd.compDx[component]
	}
	if component >= 0 && component < len(pd.compDy) && pd.compDy[component] > 0 {
		dy = pd.compDy[component]
	}
	return dx, dy
}

// SetPrecinctSize sets the precinct dimensions
func (pd *PacketDecoder) SetPrecinctSize(width, height int) {
	pd.precinctWidth = width
	pd.precinctHeight = height
	pd.precinctWidths = nil
	pd.precinctHeights = nil
}

// SetPrecinctSizes sets per-resolution precinct sizes (in pixels).
func (pd *PacketDecoder) SetPrecinctSizes(widths, heights []int) {
	pd.precinctWidths = append([]int(nil), widths...)
	pd.precinctHeights = append([]int(nil), heights...)
}

func (pd *PacketDecoder) precinctSizeForResolution(resolution int) (int, int) {
	pw := 0
	ph := 0
	if resolution >= 0 {
		if resolution < len(pd.precinctWidths) {
			pw = pd.precinctWidths[resolution]
		}
		if resolution < len(pd.precinctHeights) {
			ph = pd.precinctHeights[resolution]
		}
	}
	if pw == 0 {
		pw = pd.precinctWidth
	}
	if ph == 0 {
		ph = pd.precinctHeight
	}
	if pw == 0 {
		pw = 1 << 15
	}
	if ph == 0 {
		ph = 1 << 15
	}
	return pw, ph
}

// calculatePrecinctCBDimensions calculates code-block grid dimensions for a precinct
func (pd *PacketDecoder) calculatePrecinctCBDimensions(component, resolution, precinctIdx, band int) (int, int) {
	b := pd.componentBoundsFor(component)
	width := b.x1 - b.x0
	height := b.y1 - b.y0
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	_, _, _, _, bands := bandInfosForResolution(width, height, b.x0, b.y0, pd.numLevels, resolution)
	for _, bandInfo := range bands {
		if bandInfo.band == band {
			numCBX := (bandInfo.width + pd.cbWidth - 1) / pd.cbWidth
			numCBY := (bandInfo.height + pd.cbHeight - 1) / pd.cbHeight
			return numCBX, numCBY
		}
	}
	return 0, 0
}

func (pd *PacketDecoder) precinctCBDimensions(component, resolution, precinctIdx, band int) (int, int) {
	if pd.cbPrecinctDims != nil {
		if compMap, ok := pd.cbPrecinctDims[component]; ok {
			if resMap, ok := compMap[resolution]; ok {
				if bandMap, ok := resMap[precinctIdx]; ok {
					if dim, ok := bandMap[band]; ok {
						if dim.numCBX > 0 && dim.numCBY > 0 {
							return dim.numCBX, dim.numCBY
						}
					}
				}
			}
		}
	}
	return 0, 0
}

func (pd *PacketDecoder) precinctCBPositions(component, resolution, precinctIdx, band int) []cbPosition {
	if pd.cbPrecinctPositions == nil {
		return nil
	}
	if compMap, ok := pd.cbPrecinctPositions[component]; ok {
		if resMap, ok := compMap[resolution]; ok {
			if bandMap, ok := resMap[precinctIdx]; ok {
				return bandMap[band]
			}
		}
	}
	return nil
}

// calculateNumPrecincts calculates the number of precincts for a given resolution
func (pd *PacketDecoder) calculateNumPrecincts(component, resolution int) int {
	b := pd.componentBoundsFor(component)
	width := b.x1 - b.x0
	height := b.y1 - b.y0
	if width <= 0 || height <= 0 {
		return 0
	}
	resWidth, resHeight, resX0, resY0 := resolutionDimsWithOrigin(width, height, b.x0, b.y0, pd.numLevels, resolution)

	precinctWidth, precinctHeight := pd.precinctSizeForResolution(resolution)

	// Calculate number of precincts based on resolution dimensions and origin alignment
	startX := floorDiv(resX0, precinctWidth) * precinctWidth
	startY := floorDiv(resY0, precinctHeight) * precinctHeight
	endX := ceilDiv(resX0+resWidth, precinctWidth) * precinctWidth
	endY := ceilDiv(resY0+resHeight, precinctHeight) * precinctHeight
	numPrecinctX := (endX - startX) / precinctWidth
	numPrecinctY := (endY - startY) / precinctHeight

	if numPrecinctX == 0 {
		numPrecinctX = 1
	}
	if numPrecinctY == 0 {
		numPrecinctY = 1
	}

	return numPrecinctX * numPrecinctY
}

// calculateNumCodeBlocks calculates the number of code-blocks for a given resolution
func (pd *PacketDecoder) calculateNumCodeBlocks(component, resolution int) int {
	b := pd.componentBoundsFor(component)
	width := b.x1 - b.x0
	height := b.y1 - b.y0
	if width <= 0 || height <= 0 {
		return 0
	}
	_, _, _, _, bands := bandInfosForResolution(width, height, b.x0, b.y0, pd.numLevels, resolution)
	total := 0
	for _, bandInfo := range bands {
		if bandInfo.width <= 0 || bandInfo.height <= 0 {
			continue
		}
		numCBX := (bandInfo.width + pd.cbWidth - 1) / pd.cbWidth
		numCBY := (bandInfo.height + pd.cbHeight - 1) / pd.cbHeight
		total += numCBX * numCBY
	}
	return total
}

// DecodePackets decodes all packets according to progression order
func (pd *PacketDecoder) DecodePackets() ([]Packet, error) {
	// Packet headers use OpenJPEG-style bit stuffing handled by PacketHeaderParser.
	// Packet bodies are raw code-block data (no byte stuffing).
	pd.offset = 0

	pd.buildPrecinctOrder()

	switch pd.progression {
	case ProgressionLRCP:
		return pd.decodeComplete(pd.decodeLRCP())
	case ProgressionRLCP:
		return pd.decodeComplete(pd.decodeRLCP())
	case ProgressionRPCL:
		return pd.decodeComplete(pd.decodeRPCL())
	case ProgressionPCRL:
		return pd.decodeComplete(pd.decodePCRL())
	case ProgressionCPRL:
		return pd.decodeComplete(pd.decodeCPRL())
	default:
		return nil, fmt.Errorf("unsupported progression order: %v", pd.progression)
	}
}

// decodeComplete is a helper to log packet count when debug is on.
func (pd *PacketDecoder) decodeComplete(pkts []Packet, err error) ([]Packet, error) {
	return pkts, err
}

// decodeLRCP decodes packets in Layer-Resolution-Component-Position order
func (pd *PacketDecoder) decodeLRCP() ([]Packet, error) {
	for layer := 0; layer < pd.numLayers; layer++ {
		for res := 0; res < pd.numResolutions; res++ {
			for comp := 0; comp < pd.numComponents; comp++ {
				precincts := pd.precinctIndicesForResolution(comp, res)
				for _, precinctIdx := range precincts {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (L=%d,R=%d,C=%d,P=%d): %w",
							layer, res, comp, precinctIdx, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}

	return pd.packets, nil
}

// decodeRLCP decodes packets in Resolution-Layer-Component-Position order
func (pd *PacketDecoder) decodeRLCP() ([]Packet, error) {
	for res := 0; res < pd.numResolutions; res++ {
		for layer := 0; layer < pd.numLayers; layer++ {
			for comp := 0; comp < pd.numComponents; comp++ {
				precincts := pd.precinctIndicesForResolution(comp, res)
				for _, precinctIdx := range precincts {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (R=%d,L=%d,C=%d,P=%d): %w",
							res, layer, comp, precinctIdx, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}

	return pd.packets, nil
}

// decodeRPCL decodes packets in Resolution-Position-Component-Layer order
func (pd *PacketDecoder) decodeRPCL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for res := 0; res < pd.numResolutions; res++ {
		positions := posMaps.byRes[res]
		for _, pos := range positions {
			for comp := 0; comp < pd.numComponents; comp++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (R=%d,P=%d,C=%d,L=%d): %w",
							res, precinctIdx, comp, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodePCRL decodes packets in Position-Component-Resolution-Layer order
func (pd *PacketDecoder) decodePCRL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for _, pos := range posMaps.all {
		for comp := 0; comp < pd.numComponents; comp++ {
			for res := 0; res < pd.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (P=%d,C=%d,R=%d,L=%d): %w",
							precinctIdx, comp, res, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodeCPRL decodes packets in Component-Position-Resolution-Layer order
func (pd *PacketDecoder) decodeCPRL() ([]Packet, error) {
	posMaps := pd.buildPositionMaps()
	for comp := 0; comp < pd.numComponents; comp++ {
		positions := posMaps.byComp[comp]
		for _, pos := range positions {
			for res := 0; res < pd.numResolutions; res++ {
				resMap := posMaps.byCompRes[comp][res]
				if resMap == nil {
					continue
				}
				precinctIdx, ok := resMap[pos]
				if !ok {
					continue
				}
				for layer := 0; layer < pd.numLayers; layer++ {
					packet, err := pd.decodePacket(layer, res, comp, precinctIdx)
					if err != nil {
						return nil, fmt.Errorf("failed to decode packet (C=%d,P=%d,R=%d,L=%d): %w",
							comp, precinctIdx, res, layer, err)
					}
					pd.packets = append(pd.packets, packet)
				}
			}
		}
	}
	return pd.packets, nil
}

// decodePacket decodes a single packet
func (pd *PacketDecoder) decodePacket(layer, resolution, component, precinctIdx int) (Packet, error) {
	packet := Packet{
		LayerIndex:      layer,
		ResolutionLevel: resolution,
		ComponentIndex:  component,
		PrecinctIndex:   precinctIdx,
	}

	// Check if we've reached end of data
	if pd.offset >= len(pd.data) {
		packet.HeaderPresent = false
		return packet, nil
	}

	bands := pd.bandsForResolution(resolution)
	bandStates := make([]*packetHeaderBand, 0, len(bands))

	if pd.cbStates == nil {
		pd.cbStates = make(map[string]*packetHeaderContext)
	}

	for _, band := range bands {
		numCBX, numCBY := pd.precinctCBDimensions(component, resolution, precinctIdx, band)
		if numCBX == 0 || numCBY == 0 {
			continue
		}
		positions := pd.precinctCBPositions(component, resolution, precinctIdx, band)
		stateKey := fmt.Sprintf("%d:%d:%d:%d", component, resolution, precinctIdx, band)
		ctx := pd.cbStates[stateKey]
		if ctx == nil {
			ctx = &packetHeaderContext{}
			pd.cbStates[stateKey] = ctx
		}
		bandStates = append(bandStates, &packetHeaderBand{
			numCBX:          numCBX,
			numCBY:          numCBY,
			cbPositions:     positions,
			inclTagTree:     ctx.incl,
			zbpTagTree:      ctx.zbp,
			codeBlockStates: ctx.states,
		})
	}

	termAll := (pd.codeBlockStyle & 0x04) != 0
	header, cbIncls, bytesRead, headerPresent, err := parsePacketHeaderMulti(pd.data[pd.offset:], layer, bandStates, termAll)
	if err != nil {
		return packet, fmt.Errorf("failed to parse packet header: %w", err)
	}

	packet.HeaderPresent = headerPresent
	if !packet.HeaderPresent {
		pd.offset += bytesRead
		return packet, nil
	}

	pd.offset += bytesRead
	for i, band := range bands {
		stateKey := fmt.Sprintf("%d:%d:%d:%d", component, resolution, precinctIdx, band)
		ctx := pd.cbStates[stateKey]
		if ctx == nil || i >= len(bandStates) {
			continue
		}
		ctx.states = bandStates[i].codeBlockStates
		ctx.incl = bandStates[i].inclTagTree
		ctx.zbp = bandStates[i].zbpTagTree
	}

	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Fallback: if data length missing, infer from remaining bytes for single included block
	includedWithZero := 0
	for _, cb := range packet.CodeBlockIncls {
		if cb.Included && cb.DataLength <= 0 {
			includedWithZero++
		}
	}
	if includedWithZero == 1 {
		remain := len(pd.data) - pd.offset
		for i := range packet.CodeBlockIncls {
			if packet.CodeBlockIncls[i].Included && packet.CodeBlockIncls[i].DataLength <= 0 {
				packet.CodeBlockIncls[i].DataLength = remain
			}
		}
	}

	// Decode packet body (code-block contributions)
	body := &bytes.Buffer{}
	for i := range packet.CodeBlockIncls {
		cbIncl := &packet.CodeBlockIncls[i] // Get pointer to modify the slice element
		if cbIncl.Included && cbIncl.DataLength > 0 {
			// Read code-block data (raw bytes)
			if cbIncl.DataLength == 0 {
				continue
			}
			if pd.offset >= len(pd.data) {
				break
			}

			if pd.offset+cbIncl.DataLength > len(pd.data) {
				cbIncl.DataLength = len(pd.data) - pd.offset
			}
			cbData := pd.data[pd.offset : pd.offset+cbIncl.DataLength]
			cbIncl.Data = cbData
			body.Write(cbData)
			pd.offset += cbIncl.DataLength
		}
	}
	packet.Body = body.Bytes()

	return packet, nil
}

// decodePacketHeader decodes a packet header
// This is a simplified implementation matching our simplified encoder
func (pd *PacketDecoder) decodePacketHeader(layer, resolution, component, precinctIdx int) ([]byte, []CodeBlockIncl, error) {
	headerStart := pd.offset
	bitReader := newBitReader(pd.data[pd.offset:])
	cbIncls := make([]CodeBlockIncl, 0)

	// Calculate number of code-blocks for this precinct at this resolution level
	maxCodeBlocks := 0
	if byComp, ok := pd.cbPrecinctOrder[component]; ok {
		if byRes, ok := byComp[resolution]; ok {
			if order, ok := byRes[precinctIdx]; ok {
				maxCodeBlocks = len(order)
			}
		}
	}

	for i := 0; i < maxCodeBlocks; i++ {
		// Read inclusion bit
		inclBit, err := bitReader.readBit()
		if err != nil {
			break // End of header
		}

		cbKey := fmt.Sprintf("%d:%d:%d", component, resolution, i)

		cbIncl := CodeBlockIncl{
			Included: inclBit == 1,
		}

		if !cbIncl.Included {
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		// Check if this is the first inclusion for this code-block
		cbIncl.FirstInclusion = !pd.cbIncluded[cbKey]

		// Mark as included for future layers
		pd.cbIncluded[cbKey] = true

		// Read zero bitplanes only on first inclusion
		if cbIncl.FirstInclusion {
			// Read zero bitplanes (simplified: unary encoding, read until we see a 1 bit)
			zbp := 0
			for {
				bit, err := bitReader.readBit()
				if err != nil || bit == 1 {
					break
				}
				zbp++
			}
			cbIncl.ZeroBitplanes = zbp
		}

		// Read number of coding passes using JPEG2000 standard encoding
		// Must match encodeNumPasses in packet_header_tagtree.go
		numPasses, err := decodeNumPassesStandard(bitReader)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode number of passes: %w", err)
		}
		cbIncl.NumPasses = numPasses

		// Read data length (16-bit fixed length for simplicity)
		dataLen := 0
		for bit := 0; bit < 16; bit++ {
			b, err := bitReader.readBit()
			if err != nil {
				break
			}
			dataLen = (dataLen << 1) | b
		}
		cbIncl.DataLength = dataLen

		cbIncls = append(cbIncls, cbIncl)

		// NOTE: Removed incorrect safety check that would break on dataLen=0
		// In JPEG 2000, empty code-blocks are valid and we need to read all maxCodeBlocks
	}

	// Update offset to after header
	bytesRead := bitReader.bytesRead()
	pd.offset += bytesRead
	header := pd.data[headerStart : headerStart+bytesRead]

	return header, cbIncls, nil
}

// bitReader helps with bit-level reading
type bitReader struct {
	data        []byte
	bytePos     int // Logical position (after unstuffing)
	bitPos      int
	totalBit    int
	physicalPos int // Physical position in stuffed bitstream
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{
		data:        data,
		bytePos:     0,
		bitPos:      0,
		physicalPos: 0,
	}
}

func (br *bitReader) readBit() (int, error) {
	// At the start of reading a new byte, check for stuffed bytes
	if br.bitPos == 0 && br.bytePos > 0 {
		// Check if previous byte was 0xFF and current is stuffed 0x00
		prevByte := br.data[br.bytePos-1]
		if prevByte == 0xFF && br.bytePos < len(br.data) && br.data[br.bytePos] == 0x00 {
			// Skip the stuffed 0x00 byte
			br.bytePos++
			br.physicalPos++ // Count the skipped stuffed byte in physical position
		}
	}

	if br.bytePos >= len(br.data) {
		return 0, fmt.Errorf("end of data")
	}

	// If this is the first bit of a byte, we're starting to read a new physical byte
	if br.bitPos == 0 {
		br.physicalPos++
	}

	bit := int((br.data[br.bytePos] >> (7 - br.bitPos)) & 1)
	br.bitPos++
	br.totalBit++

	if br.bitPos == 8 {
		br.bitPos = 0
		br.bytePos++
	}

	return bit, nil
}

func (br *bitReader) bytesRead() int {
	// Return physical bytes read from stuffed bitstream
	// physicalPos is incremented at the start of reading each byte
	// so it already accounts for the current byte being read

	bytesRead := br.physicalPos

	// Special case: If we just finished reading a byte (bitPos == 0) and that byte was 0xFF,
	// the NEXT byte should be a stuffed 0x00 that will be skipped when readBit() is called next.
	// We need to account for this pending stuffed byte in our count.
	if br.bitPos == 0 && br.bytePos > 0 {
		prevByte := br.data[br.bytePos-1]
		if prevByte == 0xFF && br.bytePos < len(br.data) && br.data[br.bytePos] == 0x00 {
			// There's a pending stuffed 0x00 byte that hasn't been skipped yet
			bytesRead++
		}
	}

	return bytesRead
}

// removeByteStuffing removes JPEG 2000 byte-stuffing (0xFF 0x00 -> 0xFF)
func removeByteStuffing(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		result = append(result, data[i])
		if data[i] == 0xFF && i+1 < len(data) && data[i+1] == 0x00 {
			// Skip the stuffed 0x00 byte
			i += 2
		} else {
			i++
		}
	}
	return result
}

// readAndUnstuff reads stuffed bytes from data and unstuffs them until we have targetUnstuffedLen bytes
// Returns the unstuffed data and the number of stuffed bytes read
func readAndUnstuff(data []byte, targetUnstuffedLen int) ([]byte, int) {
	result := make([]byte, 0, targetUnstuffedLen)
	i := 0
	for i < len(data) && len(result) < targetUnstuffedLen {
		result = append(result, data[i])
		if data[i] == 0xFF && i+1 < len(data) && data[i+1] == 0x00 {
			// Skip the stuffed 0x00 byte
			i += 2
		} else {
			i++
		}
	}
	return result, i
}

// readByteWithUnstuff reads a single byte from data at offset, handling byte unstuffing
// Returns the unstuffed byte and the number of bytes consumed (1 or 2 if 0xFF 0x00 was skipped)
func readByteWithUnstuff(data []byte, offset int) (byte, int) {
	if offset >= len(data) {
		return 0, 0
	}

	b := data[offset]

	// If this byte is 0xFF and next is 0x00 (stuffed byte), skip the 0x00
	if b == 0xFF && offset+1 < len(data) && data[offset+1] == 0x00 {
		// Return 0xFF and consume 2 bytes (0xFF 0x00)
		return 0xFF, 2
	}

	// Normal byte
	return b, 1
}

// GetPackets returns the decoded packets
func (pd *PacketDecoder) GetPackets() []Packet {
	return pd.packets
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildPrecinctOrder builds the mapping from precinct index to code-block order
// for each resolution, mirroring the encoder traversal.
func (pd *PacketDecoder) buildPrecinctOrder() {
	if len(pd.cbPrecinctOrder) > 0 {
		return
	}

	if pd.cbPrecinctPositions == nil {
		pd.cbPrecinctPositions = make(map[int]map[int]map[int]map[int][]cbPosition)
	}
	if pd.cbPrecinctDims == nil {
		pd.cbPrecinctDims = make(map[int]map[int]map[int]map[int]cbGridDim)
	}

	cbw, cbh := pd.cbWidth, pd.cbHeight
	for comp := 0; comp < pd.numComponents; comp++ {
		if pd.cbPrecinctOrder[comp] == nil {
			pd.cbPrecinctOrder[comp] = make(map[int]map[int][]int)
		}
		if pd.cbPrecinctPositions[comp] == nil {
			pd.cbPrecinctPositions[comp] = make(map[int]map[int]map[int][]cbPosition)
		}
		if pd.cbPrecinctDims[comp] == nil {
			pd.cbPrecinctDims[comp] = make(map[int]map[int]map[int]cbGridDim)
		}

		b := pd.componentBoundsFor(comp)
		compWidth := b.x1 - b.x0
		compHeight := b.y1 - b.y0
		if compWidth <= 0 || compHeight <= 0 {
			continue
		}

		globalCBIdx := 0
		for res := 0; res < pd.numResolutions; res++ {
			pw, ph := pd.precinctSizeForResolution(res)
			if pd.cbPrecinctOrder[comp][res] == nil {
				pd.cbPrecinctOrder[comp][res] = make(map[int][]int)
			}
			type cbEntry struct {
				cbx    int
				cby    int
				global int
			}
			precinctBands := make(map[int]map[int][]cbEntry)
			addEntry := func(pIdx, band, cbxLocal, cbyLocal, global int) {
				if precinctBands[pIdx] == nil {
					precinctBands[pIdx] = make(map[int][]cbEntry)
				}
				precinctBands[pIdx][band] = append(precinctBands[pIdx][band], cbEntry{cbx: cbxLocal, cby: cbyLocal, global: global})
			}

			resW, resH, resX0, resY0, bands := bandInfosForResolution(compWidth, compHeight, b.x0, b.y0, pd.numLevels, res)
			if resW <= 0 || resH <= 0 {
				continue
			}
			startX := floorDiv(resX0, pw) * pw
			startY := floorDiv(resY0, ph) * ph
			endX := ceilDiv(resX0+resW, pw) * pw
			numPrecinctX := (endX - startX) / pw
			if numPrecinctX < 1 {
				numPrecinctX = 1
			}

			for _, bandInfo := range bands {
				if bandInfo.width <= 0 || bandInfo.height <= 0 {
					continue
				}
				numCBX := (bandInfo.width + cbw - 1) / cbw
				numCBY := (bandInfo.height + cbh - 1) / cbh
				for cby := 0; cby < numCBY; cby++ {
					for cbx := 0; cbx < numCBX; cbx++ {
						cbX0 := cbx * cbw
						cbY0 := cby * cbh
						absResX0 := resX0 + cbX0
						absResY0 := resY0 + cbY0
						px := (absResX0 - startX) / pw
						py := (absResY0 - startY) / ph
						pIdx := py*numPrecinctX + px
						localX := absResX0 - (startX + px*pw)
						localY := absResY0 - (startY + py*ph)
						cbxLocal := localX / cbw
						cbyLocal := localY / cbh
						addEntry(pIdx, bandInfo.band, cbxLocal, cbyLocal, globalCBIdx)
						globalCBIdx++
					}
				}
			}

			bandOrder := []int{0}
			if res > 0 {
				bandOrder = []int{1, 2, 3}
			}
			for pIdx, bandMap := range precinctBands {
				for _, band := range bandOrder {
					entries := bandMap[band]
					if len(entries) == 0 {
						continue
					}
					sort.Slice(entries, func(i, j int) bool {
						if entries[i].cby != entries[j].cby {
							return entries[i].cby < entries[j].cby
						}
						return entries[i].cbx < entries[j].cbx
					})

					if pd.cbPrecinctPositions[comp][res] == nil {
						pd.cbPrecinctPositions[comp][res] = make(map[int]map[int][]cbPosition)
					}
					if pd.cbPrecinctPositions[comp][res][pIdx] == nil {
						pd.cbPrecinctPositions[comp][res][pIdx] = make(map[int][]cbPosition)
					}
					if pd.cbPrecinctDims[comp][res] == nil {
						pd.cbPrecinctDims[comp][res] = make(map[int]map[int]cbGridDim)
					}
					if pd.cbPrecinctDims[comp][res][pIdx] == nil {
						pd.cbPrecinctDims[comp][res][pIdx] = make(map[int]cbGridDim)
					}

					positions := make([]cbPosition, 0, len(entries))
					maxX, maxY := 0, 0
					for _, entry := range entries {
						pd.cbPrecinctOrder[comp][res][pIdx] = append(pd.cbPrecinctOrder[comp][res][pIdx], entry.global)
						positions = append(positions, cbPosition{X: entry.cbx, Y: entry.cby})
						if entry.cbx+1 > maxX {
							maxX = entry.cbx + 1
						}
						if entry.cby+1 > maxY {
							maxY = entry.cby + 1
						}
					}
					pd.cbPrecinctPositions[comp][res][pIdx][band] = positions
					pd.cbPrecinctDims[comp][res][pIdx][band] = cbGridDim{numCBX: maxX, numCBY: maxY}
				}
			}
		}
	}
}

// toResolutionCoordinates mirrors encoder coordinate mapping.
func (pd *PacketDecoder) toResolutionCoordinates(globalX, globalY, resolution, band, sbWidth, sbHeight int) (int, int) {
	if resolution == 0 {
		return globalX, globalY
	}

	resX := globalX
	resY := globalY
	switch band {
	case 1: // HL
		resX = globalX - sbWidth
	case 2: // LH
		resY = globalY - sbHeight
	case 3: // HH
		resX = globalX - sbWidth
		resY = globalY - sbHeight
	}
	return resX, resY
}

func (pd *PacketDecoder) bandsForResolution(resolution int) []int {
	if resolution == 0 {
		return []int{0}
	}
	return []int{1, 2, 3}
}

// precinctIndicesForResolution returns sorted precinct indices that contain code-blocks for a resolution.
func (pd *PacketDecoder) precinctIndicesForResolution(component, resolution int) []int {
	byComp, ok := pd.cbPrecinctOrder[component]
	if !ok {
		return nil
	}
	order := byComp[resolution]
	indices := make([]int, 0, len(order))
	for idx := range order {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

func (pd *PacketDecoder) buildPositionMaps() *positionMaps {
	return buildPositionMaps(positionInputs{
		numComponents:     pd.numComponents,
		numResolutions:    pd.numResolutions,
		precinctIndices:   pd.precinctIndicesForResolution,
		componentBounds:   pd.componentBoundsFor,
		componentSampling: pd.componentSamplingFor,
		precinctSize:      pd.precinctSizeForResolution,
	})
}

func (pd *PacketDecoder) precinctIndicesForResolutionAll(resolution int) []int {
	union := make(map[int]struct{})
	for comp := 0; comp < pd.numComponents; comp++ {
		if byComp, ok := pd.cbPrecinctOrder[comp]; ok {
			for idx := range byComp[resolution] {
				union[idx] = struct{}{}
			}
		}
	}
	indices := make([]int, 0, len(union))
	for idx := range union {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

func (pd *PacketDecoder) precinctIndexSets(component int) []map[int]struct{} {
	sets := make([]map[int]struct{}, pd.numResolutions)
	for res := 0; res < pd.numResolutions; res++ {
		sets[res] = make(map[int]struct{})
		if byComp, ok := pd.cbPrecinctOrder[component]; ok {
			for idx := range byComp[res] {
				sets[res][idx] = struct{}{}
			}
		}
	}
	return sets
}

func (pd *PacketDecoder) globalPrecinctIndicesAll() []int {
	union := make(map[int]struct{})
	for comp := 0; comp < pd.numComponents; comp++ {
		sets := pd.precinctIndexSets(comp)
		for _, set := range sets {
			for idx := range set {
				union[idx] = struct{}{}
			}
		}
	}
	indices := make([]int, 0, len(union))
	for idx := range union {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

// decodeNumPassesStandard decodes the number of coding passes using JPEG2000 standard encoding
// Must match encodeNumPasses in packet_header_tagtree.go
func decodeNumPassesStandard(br *bitReader) (int, error) {
	// Number of passes is encoded with a variable-length code (OpenJPEG opj_t2_getnumpasses):
	// 0           → 1 pass (1 bit)
	// 10          → 2 passes (2 bits)
	// 11xx        → 3-5 passes where xx ≠ 11 (4 bits total)
	// 1111xxxxx   → 6-36 passes where xxxxx ≠ 11111 (9 bits total)
	// 111111111xxxxxxx → 37-164 passes (16 bits total)

	// Read first bit
	bit1, err := br.readBit()
	if err != nil {
		return 0, err
	}
	if bit1 == 0 {
		return 1, nil
	}

	// Read second bit
	bit2, err := br.readBit()
	if err != nil {
		return 0, err
	}
	if bit2 == 0 {
		return 2, nil
	}

	// Read 2 bits
	val2 := 0
	for i := 0; i < 2; i++ {
		bit, err := br.readBit()
		if err != nil {
			return 0, err
		}
		val2 = (val2 << 1) | bit
	}
	if val2 != 3 {
		// 11xx where xx ≠ 11 → 3-5 passes
		return 3 + val2, nil
	}

	// Read 5 bits
	val5 := 0
	for i := 0; i < 5; i++ {
		bit, err := br.readBit()
		if err != nil {
			return 0, err
		}
		val5 = (val5 << 1) | bit
	}
	if val5 != 31 {
		// 1111xxxxx where xxxxx ≠ 11111 → 6-36 passes
		return 6 + val5, nil
	}

	// Read 7 bits for 37-164 passes
	val7 := 0
	for i := 0; i < 7; i++ {
		bit, err := br.readBit()
		if err != nil {
			return 0, err
		}
		val7 = (val7 << 1) | bit
	}
	return 37 + val7, nil
}
