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

	// Precinct parameters
	precinctWidth  int // Precinct width (0 = default size of 2^15)
	precinctHeight int // Precinct height (0 = default size of 2^15)

	// Parsed packets
	packets []Packet

	// Multi-layer state tracking
	// Maps "component:resolution:cbIndex" -> true if code-block was included in a previous layer
	cbIncluded map[string]bool

	// Precinct -> code-block order mapping per resolution (mirrors encoder traversal)
	cbPrecinctOrder map[int]map[int][]int
	// Precinct -> code-block positions (CBX/CBY) in packet header order
	cbPrecinctPositions map[int]map[int]map[int][]cbPosition
	// Precinct -> code-block grid dimensions used for tag-tree decoding
	cbPrecinctDims map[int]map[int]map[int]cbGridDim

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
		cbPrecinctOrder:     make(map[int]map[int][]int),
		cbPrecinctPositions: make(map[int]map[int]map[int][]cbPosition),
		cbPrecinctDims:      make(map[int]map[int]map[int]cbGridDim),
	}
}

// SetImageDimensions sets the image and code-block dimensions
func (pd *PacketDecoder) SetImageDimensions(width, height, cbWidth, cbHeight int) {
	pd.imageWidth = width
	pd.imageHeight = height
	pd.cbWidth = cbWidth
	pd.cbHeight = cbHeight
}

// SetPrecinctSize sets the precinct dimensions
func (pd *PacketDecoder) SetPrecinctSize(width, height int) {
	pd.precinctWidth = width
	pd.precinctHeight = height
}

// calculatePrecinctCBDimensions calculates code-block grid dimensions for a precinct
func (pd *PacketDecoder) calculatePrecinctCBDimensions(resolution, precinctIdx, band int) (int, int) {
	sbWidth := subbandDim(pd.imageWidth, pd.numLevels, resolution)
	sbHeight := subbandDim(pd.imageHeight, pd.numLevels, resolution)

	// Each resolution has 3 subbands (HL, LH, HH) for res>0, all with same dimensions.
	numCBX := (sbWidth + pd.cbWidth - 1) / pd.cbWidth
	numCBY := (sbHeight + pd.cbHeight - 1) / pd.cbHeight

	// Total code-blocks in precinct = 3 subbands × numCBX × numCBY
	// But PacketHeaderParser expects grid for one subband, so return single subband dimensions
	return numCBX, numCBY
}

func (pd *PacketDecoder) precinctCBDimensions(resolution, precinctIdx, band int) (int, int) {
	if pd.cbPrecinctDims != nil {
		if resMap, ok := pd.cbPrecinctDims[resolution]; ok {
			if bandMap, ok := resMap[precinctIdx]; ok {
				if dim, ok := bandMap[band]; ok {
					if dim.numCBX > 0 && dim.numCBY > 0 {
						return dim.numCBX, dim.numCBY
					}
				}
			}
		}
	}
	return 0, 0
}

func (pd *PacketDecoder) precinctCBPositions(resolution, precinctIdx, band int) []cbPosition {
	if pd.cbPrecinctPositions == nil {
		return nil
	}
	if resMap, ok := pd.cbPrecinctPositions[resolution]; ok {
		if bandMap, ok := resMap[precinctIdx]; ok {
			return bandMap[band]
		}
	}
	return nil
}

// calculateNumPrecincts calculates the number of precincts for a given resolution
func (pd *PacketDecoder) calculateNumPrecincts(resolution int) int {
	resWidth := resolutionDim(pd.imageWidth, pd.numLevels, resolution)
	resHeight := resolutionDim(pd.imageHeight, pd.numLevels, resolution)

	// Default precinct size is entire resolution (single precinct)
	precinctWidth := pd.precinctWidth
	precinctHeight := pd.precinctHeight
	if precinctWidth == 0 {
		precinctWidth = 1 << 15 // Default 32768
	}
	if precinctHeight == 0 {
		precinctHeight = 1 << 15 // Default 32768
	}

	// Calculate number of precincts based on resolution dimensions
	numPrecinctX := (resWidth + precinctWidth - 1) / precinctWidth
	numPrecinctY := (resHeight + precinctHeight - 1) / precinctHeight

	if numPrecinctX == 0 {
		numPrecinctX = 1
	}
	if numPrecinctY == 0 {
		numPrecinctY = 1
	}

	return numPrecinctX * numPrecinctY
}

// calculateNumCodeBlocks calculates the number of code-blocks for a given resolution
func (pd *PacketDecoder) calculateNumCodeBlocks(resolution int) int {
	if resolution == 0 {
		// Resolution 0: LL subband only (single subband at top-left)
		llWidth := subbandDim(pd.imageWidth, pd.numLevels, resolution)
		llHeight := subbandDim(pd.imageHeight, pd.numLevels, resolution)
		numCBX := (llWidth + pd.cbWidth - 1) / pd.cbWidth
		numCBY := (llHeight + pd.cbHeight - 1) / pd.cbHeight
		return numCBX * numCBY
	} else {
		// Resolution r > 0: HL, LH, HH subbands (3 subbands)
		sbWidth := subbandDim(pd.imageWidth, pd.numLevels, resolution)
		sbHeight := subbandDim(pd.imageHeight, pd.numLevels, resolution)
		numCBX := (sbWidth + pd.cbWidth - 1) / pd.cbWidth
		numCBY := (sbHeight + pd.cbHeight - 1) / pd.cbHeight
		// 3 subbands (HL, LH, HH), each with numCBX * numCBY code-blocks
		return 3 * numCBX * numCBY
	}
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
			precincts := pd.precinctIndicesForResolution(res)
			for comp := 0; comp < pd.numComponents; comp++ {
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
		precincts := pd.precinctIndicesForResolution(res)
		for layer := 0; layer < pd.numLayers; layer++ {
			for comp := 0; comp < pd.numComponents; comp++ {
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
	for res := 0; res < pd.numResolutions; res++ {
		precincts := pd.precinctIndicesForResolution(res)
		for _, precinctIdx := range precincts {
			for comp := 0; comp < pd.numComponents; comp++ {
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
	sets := pd.precinctIndexSets()
	globalPrecincts := pd.globalPrecinctIndices(sets)

	for _, precinctIdx := range globalPrecincts {
		for comp := 0; comp < pd.numComponents; comp++ {
			for res := 0; res < pd.numResolutions; res++ {
				if _, ok := sets[res][precinctIdx]; !ok {
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
	sets := pd.precinctIndexSets()
	globalPrecincts := pd.globalPrecinctIndices(sets)

	for comp := 0; comp < pd.numComponents; comp++ {
		for _, precinctIdx := range globalPrecincts {
			for res := 0; res < pd.numResolutions; res++ {
				if _, ok := sets[res][precinctIdx]; !ok {
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
		numCBX, numCBY := pd.precinctCBDimensions(resolution, precinctIdx, band)
		if numCBX == 0 || numCBY == 0 {
			continue
		}
		positions := pd.precinctCBPositions(resolution, precinctIdx, band)
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
	if byRes, ok := pd.cbPrecinctOrder[resolution]; ok {
		if order, ok := byRes[precinctIdx]; ok {
			maxCodeBlocks = len(order)
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
		pd.cbPrecinctPositions = make(map[int]map[int]map[int][]cbPosition)
	}
	if pd.cbPrecinctDims == nil {
		pd.cbPrecinctDims = make(map[int]map[int]map[int]cbGridDim)
	}

	pw, ph := pd.precinctWidth, pd.precinctHeight
	if pw == 0 {
		pw = 1 << 15
	}
	if ph == 0 {
		ph = 1 << 15
	}

	cbw, cbh := pd.cbWidth, pd.cbHeight
	globalCBIdx := 0

	for res := 0; res < pd.numResolutions; res++ {
		if pd.cbPrecinctOrder[res] == nil {
			pd.cbPrecinctOrder[res] = make(map[int][]int)
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

		if res == 0 {
			llWidth := subbandDim(pd.imageWidth, pd.numLevels, res)
			llHeight := subbandDim(pd.imageHeight, pd.numLevels, res)

			numCBX := (llWidth + cbw - 1) / cbw
			numCBY := (llHeight + cbh - 1) / cbh

			resWidth := resolutionDim(pd.imageWidth, pd.numLevels, res)
			numPrecinctX := (resWidth + pw - 1) / pw
			if numPrecinctX < 1 {
				numPrecinctX = 1
			}

			for cby := 0; cby < numCBY; cby++ {
				for cbx := 0; cbx < numCBX; cbx++ {
					x0 := cbx * cbw
					y0 := cby * cbh
					px := x0 / pw
					py := y0 / ph
					pIdx := py*numPrecinctX + px
					localX := x0 - px*pw
					localY := y0 - py*ph
					cbxLocal := localX / cbw
					cbyLocal := localY / cbh
					addEntry(pIdx, 0, cbxLocal, cbyLocal, globalCBIdx)
					globalCBIdx++
				}
			}
		} else {
			sbWidth := subbandDim(pd.imageWidth, pd.numLevels, res)
			sbHeight := subbandDim(pd.imageHeight, pd.numLevels, res)

			numCBX := (sbWidth + cbw - 1) / cbw
			numCBY := (sbHeight + cbh - 1) / cbh

			resWidth := resolutionDim(pd.imageWidth, pd.numLevels, res)
			numPrecinctX := (resWidth + pw - 1) / pw
			if numPrecinctX < 1 {
				numPrecinctX = 1
			}

			subbands := []struct {
				x0, y0 int
				band   int
			}{
				{sbWidth, 0, 1},        // HL
				{0, sbHeight, 2},       // LH
				{sbWidth, sbHeight, 3}, // HH
			}

			for _, sb := range subbands {
				for cby := 0; cby < numCBY; cby++ {
					for cbx := 0; cbx < numCBX; cbx++ {
						x0 := sb.x0 + cbx*cbw
						y0 := sb.y0 + cby*cbh
						resX, resY := pd.toResolutionCoordinates(x0, y0, res, sb.band, sbWidth, sbHeight)
						if resX < 0 {
							resX = 0
						}
						if resY < 0 {
							resY = 0
						}
						px := resX / pw
						py := resY / ph
						pIdx := py*numPrecinctX + px
						localX := resX - px*pw
						localY := resY - py*ph
						cbxLocal := localX / cbw
						cbyLocal := localY / cbh
						addEntry(pIdx, sb.band, cbxLocal, cbyLocal, globalCBIdx)
						globalCBIdx++
					}
				}
			}
		}

		bands := []int{0}
		if res > 0 {
			bands = []int{1, 2, 3}
		}
		for pIdx, bandMap := range precinctBands {
			for _, band := range bands {
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

				if pd.cbPrecinctPositions[res] == nil {
					pd.cbPrecinctPositions[res] = make(map[int]map[int][]cbPosition)
				}
				if pd.cbPrecinctPositions[res][pIdx] == nil {
					pd.cbPrecinctPositions[res][pIdx] = make(map[int][]cbPosition)
				}
				if pd.cbPrecinctDims[res] == nil {
					pd.cbPrecinctDims[res] = make(map[int]map[int]cbGridDim)
				}
				if pd.cbPrecinctDims[res][pIdx] == nil {
					pd.cbPrecinctDims[res][pIdx] = make(map[int]cbGridDim)
				}

				positions := make([]cbPosition, 0, len(entries))
				maxX, maxY := 0, 0
				for _, entry := range entries {
					pd.cbPrecinctOrder[res][pIdx] = append(pd.cbPrecinctOrder[res][pIdx], entry.global)
					positions = append(positions, cbPosition{X: entry.cbx, Y: entry.cby})
					if entry.cbx+1 > maxX {
						maxX = entry.cbx + 1
					}
					if entry.cby+1 > maxY {
						maxY = entry.cby + 1
					}
				}
				pd.cbPrecinctPositions[res][pIdx][band] = positions
				pd.cbPrecinctDims[res][pIdx][band] = cbGridDim{numCBX: maxX, numCBY: maxY}
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
func (pd *PacketDecoder) precinctIndicesForResolution(resolution int) []int {
	order := pd.cbPrecinctOrder[resolution]
	indices := make([]int, 0, len(order))
	for idx := range order {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

func (pd *PacketDecoder) precinctIndexSets() []map[int]struct{} {
	sets := make([]map[int]struct{}, pd.numResolutions)
	for res := 0; res < pd.numResolutions; res++ {
		sets[res] = make(map[int]struct{})
		for idx := range pd.cbPrecinctOrder[res] {
			sets[res][idx] = struct{}{}
		}
	}
	return sets
}

func (pd *PacketDecoder) globalPrecinctIndices(sets []map[int]struct{}) []int {
	union := make(map[int]struct{})
	for _, set := range sets {
		for idx := range set {
			union[idx] = struct{}{}
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
