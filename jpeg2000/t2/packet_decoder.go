package t2

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

	// Parsed packets
	packets []Packet

	// Multi-layer state tracking
	// Maps "component:resolution:cbIndex" -> true if code-block was included in a previous layer
	cbIncluded map[string]bool
}

// NewPacketDecoder creates a new packet decoder
func NewPacketDecoder(data []byte, numComponents, numLayers, numResolutions int, progression ProgressionOrder, codeBlockStyle uint8) *PacketDecoder {
	return &PacketDecoder{
		data:           data,
		offset:         0,
		numComponents:  numComponents,
		numLayers:      numLayers,
		numResolutions: numResolutions,
		progression:    progression,
		imageWidth:     0,  // Will be set later if needed
		imageHeight:    0,  // Will be set later if needed
		cbWidth:        64, // Default code-block size
		cbHeight:       64, // Default code-block size
		numLevels:      numResolutions - 1,
		codeBlockStyle: codeBlockStyle,
		packets:        make([]Packet, 0),
		cbIncluded:     make(map[string]bool),
	}
}

// SetImageDimensions sets the image and code-block dimensions
func (pd *PacketDecoder) SetImageDimensions(width, height, cbWidth, cbHeight int) {
	pd.imageWidth = width
	pd.imageHeight = height
	pd.cbWidth = cbWidth
	pd.cbHeight = cbHeight
}

// calculateNumCodeBlocks calculates the number of code-blocks for a given resolution
func (pd *PacketDecoder) calculateNumCodeBlocks(resolution int) int {
	if resolution == 0 {
		// Resolution 0: LL subband only (single subband at top-left)
		llWidth := pd.imageWidth >> pd.numLevels
		llHeight := pd.imageHeight >> pd.numLevels
		numCBX := (llWidth + pd.cbWidth - 1) / pd.cbWidth
		numCBY := (llHeight + pd.cbHeight - 1) / pd.cbHeight
		return numCBX * numCBY
	} else {
		// Resolution r > 0: HL, LH, HH subbands (3 subbands)
		level := pd.numLevels - resolution + 1
		sbWidth := pd.imageWidth >> level
		sbHeight := pd.imageHeight >> level
		numCBX := (sbWidth + pd.cbWidth - 1) / pd.cbWidth
		numCBY := (sbHeight + pd.cbHeight - 1) / pd.cbHeight
		// 3 subbands (HL, LH, HH), each with numCBX * numCBY code-blocks
		return 3 * numCBX * numCBY
	}
}

// DecodePackets decodes all packets according to progression order
func (pd *PacketDecoder) DecodePackets() ([]Packet, error) {
	// NOTE: We do NOT remove byte-stuffing upfront!
	// Instead, the bitReader will handle stuffed bytes during header parsing,
	// and we'll unstuff packet bodies when reading them.
	pd.offset = 0

	switch pd.progression {
	case ProgressionLRCP:
		return pd.decodeLRCP()
	case ProgressionRLCP:
		return pd.decodeRLCP()
	default:
		return nil, fmt.Errorf("unsupported progression order: %v", pd.progression)
	}
}

// decodeLRCP decodes packets in Layer-Resolution-Component-Position order
func (pd *PacketDecoder) decodeLRCP() ([]Packet, error) {
	for layer := 0; layer < pd.numLayers; layer++ {
		for res := 0; res < pd.numResolutions; res++ {
			for comp := 0; comp < pd.numComponents; comp++ {
				// For each precinct (simplified: single precinct per resolution)
				packet, err := pd.decodePacket(layer, res, comp, 0)
				if err != nil {
					return nil, fmt.Errorf("failed to decode packet (L=%d,R=%d,C=%d): %w",
						layer, res, comp, err)
				}
				pd.packets = append(pd.packets, packet)
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
				// For each precinct (simplified: single precinct per resolution)
				packet, err := pd.decodePacket(layer, res, comp, 0)
				if err != nil {
					return nil, fmt.Errorf("failed to decode packet (R=%d,L=%d,C=%d): %w",
						res, layer, comp, err)
				}
				pd.packets = append(pd.packets, packet)
			}
		}
	}

	return pd.packets, nil
}

// decodePacket decodes a single packet
func (pd *PacketDecoder) decodePacket(layer, resolution, component, precinctIdx int) (Packet, error) {
	packet := Packet{
		LayerIndex:      layer,
		ResolutionLevel:  resolution,
		ComponentIndex:  component,
		PrecinctIndex:   precinctIdx,
	}


	// Check if we've reached end of data
	if pd.offset >= len(pd.data) {
		packet.HeaderPresent = false
		return packet, nil
	}

	// Check for empty packet (header not present)
	if pd.offset < len(pd.data) && pd.data[pd.offset] == 0x00 {
		if layer < 2 && resolution == 0 {
			fmt.Printf("[PACKET DEC Layer=%d Res=%d] Empty packet detected at offset %d\n", layer, resolution, pd.offset)
		}
		packet.HeaderPresent = false
		pd.offset++
		return packet, nil
	}

	packet.HeaderPresent = true

	if layer == 0 || (layer == 1 && resolution == 0) {
		fmt.Printf("[PACKET DEC Layer=%d Res=%d] Decoding header at offset %d, byte=%02x\n",
			layer, resolution, pd.offset, pd.data[pd.offset])
	}

	// Decode packet header
	header, cbIncls, err := pd.decodePacketHeader(layer, resolution, component)
	if err != nil {
		return packet, fmt.Errorf("failed to decode packet header: %w", err)
	}
	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Decode packet body (code-block contributions)
	// Check if TERMALL mode is enabled (bit 2 of CodeBlockStyle)
	useTERMALL := (pd.codeBlockStyle & 0x04) != 0

	body := &bytes.Buffer{}
	for i := range cbIncls {
		cbIncl := &cbIncls[i] // Get pointer to modify the slice element
		if cbIncl.Included && cbIncl.DataLength > 0 {
			// In TERMALL mode, read PassLengths metadata first
			if useTERMALL {
				// Read number of passes (1 byte)
				if pd.offset >= len(pd.data) {
					break
				}
				numPasses := int(pd.data[pd.offset])
				pd.offset++

				// Read pass lengths (2 bytes each, big-endian)
				cbIncl.PassLengths = make([]int, numPasses)
				for j := 0; j < numPasses; j++ {
					if pd.offset+2 > len(pd.data) {
						break
					}
					passLen := binary.BigEndian.Uint16(pd.data[pd.offset : pd.offset+2])
					cbIncl.PassLengths[j] = int(passLen)
					pd.offset += 2
				}
				cbIncl.UseTERMALL = true

				// Adjust cbIncl.DataLength: header contains TOTAL (metadata + data)
				// We've already consumed metadata bytes, so subtract them
				metadataBytes := 1 + numPasses*2
				if cbIncl.DataLength >= metadataBytes {
					cbIncl.DataLength -= metadataBytes
				} else {
					cbIncl.DataLength = 0
				}
			}

			// Read code-block data
			if layer == 0 && resolution == 5 {
				fmt.Printf("[PACKET DEC Body Layer=%d Res=%d] Before read: cbIncl.DataLength=%d, offset=%d\n",
					layer, resolution, cbIncl.DataLength, pd.offset)
			}

			if pd.offset+cbIncl.DataLength > len(pd.data) {
				// Not enough data - this might be normal at end of stream
				// Just read what's available
				remainingData := len(pd.data) - pd.offset
				if remainingData > 0 {
					cbData := pd.data[pd.offset:len(pd.data)]
					// Store the data we have and update CodeBlockIncl
					cbIncl.Data = cbData
					cbIncl.DataLength = len(cbData)
					body.Write(cbData)
					pd.offset = len(pd.data)
				}
				break
			}
			// Read and unstuff code-block data
			// DataLength is the UNSTUFFED length, but we need to read STUFFED bytes from bitstream
			cbData, bytesRead := readAndUnstuff(pd.data[pd.offset:], cbIncl.DataLength)
			cbIncl.Data = cbData
			body.Write(cbData)
			pd.offset += bytesRead

			if layer == 0 && resolution == 5 {
				fmt.Printf("[PACKET DEC Body Layer=%d Res=%d] Read %d stuffed bytes -> %d unstuffed bytes, new offset=%d\n",
					layer, resolution, bytesRead, len(cbData), pd.offset)
			}
		}
	}
	packet.Body = body.Bytes()

	if layer == 0 || (layer == 1 && resolution == 0) {
		fmt.Printf("[PACKET DEC Layer=%d Res=%d] After decoding, offset=%d\n",
			layer, resolution, pd.offset)
	}

	return packet, nil
}

// decodePacketHeader decodes a packet header
// This is a simplified implementation matching our simplified encoder
func (pd *PacketDecoder) decodePacketHeader(layer, resolution, component int) ([]byte, []CodeBlockIncl, error) {
	headerStart := pd.offset
	bitReader := newBitReader(pd.data[pd.offset:])
	cbIncls := make([]CodeBlockIncl, 0)

	if (layer < 2 && resolution == 0) || (layer == 0 && resolution == 5) {
		showBytes := 20
		if pd.offset+showBytes > len(pd.data) {
			showBytes = len(pd.data) - pd.offset
		}
		fmt.Printf("[PACKET DEC Header Layer=%d Res=%d] First %d bytes from offset %d: %02x\n",
			layer, resolution, showBytes, pd.offset, pd.data[pd.offset:pd.offset+showBytes])

		// Also show what the encoder claims to have written
		if layer == 1 && resolution == 0 {
			fmt.Printf("[PACKET DEC Header] Expected to see encoder's header: 80100040\n")
		}
	}

	// Calculate number of code-blocks for this resolution level
	maxCodeBlocks := pd.calculateNumCodeBlocks(resolution)

	if layer == 0 && resolution == 5 {
		fmt.Printf("[PACKET DEC Header Layer=%d Res=%d] maxCodeBlocks=%d\n",
			layer, resolution, maxCodeBlocks)
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

		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] Inclusion bit=%d\n",
				layer, resolution, inclBit)
		}

		if !cbIncl.Included {
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		// Check if this is the first inclusion for this code-block
		cbIncl.FirstInclusion = !pd.cbIncluded[cbKey]

		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] FirstInclusion=%v\n",
				layer, resolution, cbIncl.FirstInclusion)
		}

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

		// Read number of coding passes (simplified unary code)
		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] Before NumPasses: bytePos=%d, bitPos=%d\n",
				layer, resolution, bitReader.bytePos, bitReader.bitPos)
		}

		numPasses := 1
		bitsRead := 0
		for {
			bit, err := bitReader.readBit()
			bitsRead++
			if err != nil || bit == 1 {
				break
			}
			numPasses++
		}
		cbIncl.NumPasses = numPasses

		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] NumPasses=%d (read %d bits)\n",
				layer, resolution, numPasses, bitsRead)
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] After NumPasses: bytePos=%d, bitPos=%d\n",
				layer, resolution, bitReader.bytePos, bitReader.bitPos)
		}

		// Read data length (16-bit fixed length for simplicity)
		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] Before DataLength: bytePos=%d, bitPos=%d\n",
				layer, resolution, bitReader.bytePos, bitReader.bitPos)
		}

		dataLen := 0
		for bit := 0; bit < 16; bit++ {
			b, err := bitReader.readBit()
			if err != nil {
				break
			}
			dataLen = (dataLen << 1) | b
		}
		cbIncl.DataLength = dataLen

		if cbKey == "0:0:0" && layer < 2 {
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] Decoded DataLength=%d (0x%04x)\n",
				layer, resolution, dataLen, dataLen)
			fmt.Printf("[PACKET DEC Header Layer=%d Res=%d CB 0:0] After DataLength: bytePos=%d, bitPos=%d\n",
				layer, resolution, bitReader.bytePos, bitReader.bitPos)
		}

		cbIncls = append(cbIncls, cbIncl)

		// NOTE: Removed incorrect safety check that would break on dataLen=0
		// In JPEG 2000, empty code-blocks are valid and we need to read all maxCodeBlocks
	}

	// Update offset to after header
	bytesRead := bitReader.bytesRead()
	pd.offset += bytesRead
	header := pd.data[headerStart : headerStart+bytesRead]

	if layer == 0 && resolution == 5 {
		fmt.Printf("[PACKET DEC Header Layer=%d Res=%d] Read %d code-blocks, Header bytesRead=%d (physicalPos=%d, bytePos=%d, bitPos=%d), new offset=%d\n",
			layer, resolution, len(cbIncls), bytesRead, bitReader.physicalPos, bitReader.bytePos, bitReader.bitPos, pd.offset)
	}

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
	return br.physicalPos
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
