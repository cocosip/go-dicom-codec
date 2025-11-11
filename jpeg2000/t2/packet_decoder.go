package t2

import (
	"bytes"
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

	// Parsed packets
	packets []Packet
}

// NewPacketDecoder creates a new packet decoder
func NewPacketDecoder(data []byte, numComponents, numLayers, numResolutions int, progression ProgressionOrder) *PacketDecoder {
	return &PacketDecoder{
		data:           data,
		offset:         0,
		numComponents:  numComponents,
		numLayers:      numLayers,
		numResolutions: numResolutions,
		progression:    progression,
		packets:        make([]Packet, 0),
	}
}

// DecodePackets decodes all packets according to progression order
func (pd *PacketDecoder) DecodePackets() ([]Packet, error) {
	// Remove byte-stuffing first
	unstuffed := removeByteStuffing(pd.data)
	pd.data = unstuffed
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
		ResolutionLevel: resolution,
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
		packet.HeaderPresent = false
		pd.offset++
		return packet, nil
	}

	packet.HeaderPresent = true

	// Decode packet header
	header, cbIncls, err := pd.decodePacketHeader()
	if err != nil {
		return packet, fmt.Errorf("failed to decode packet header: %w", err)
	}
	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Decode packet body (code-block contributions)
	body := &bytes.Buffer{}
	for _, cbIncl := range cbIncls {
		if cbIncl.Included && cbIncl.DataLength > 0 {
			// Read code-block data
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
// This is a simplified implementation matching our encoder
func (pd *PacketDecoder) decodePacketHeader() ([]byte, []CodeBlockIncl, error) {
	headerStart := pd.offset
	bitReader := newBitReader(pd.data[pd.offset:])
	cbIncls := make([]CodeBlockIncl, 0)

	// For simplified implementation, assume we know the number of code-blocks
	// In production, this would come from precinct structure
	// For now, decode until we hit a pattern that indicates end of header

	// Try to decode code-block inclusions
	maxCodeBlocks := 64 // Safety limit
	for i := 0; i < maxCodeBlocks; i++ {
		// Read inclusion bit
		inclBit, err := bitReader.readBit()
		if err != nil {
			break // End of header
		}

		cbIncl := CodeBlockIncl{
			Included: inclBit == 1,
		}

		if !cbIncl.Included {
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		// Code-block is included
		cbIncl.FirstInclusion = true // Simplified

		// Read zero bitplanes (simplified: skip tag tree, read termination)
		for {
			bit, err := bitReader.readBit()
			if err != nil || bit == 1 {
				break
			}
		}

		// Read number of coding passes (simplified unary code)
		numPasses := 1
		for {
			bit, err := bitReader.readBit()
			if err != nil || bit == 1 {
				break
			}
			numPasses++
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

		// Safety check: if we've read a reasonable amount, stop
		if len(cbIncls) > 0 && dataLen == 0 {
			break
		}
	}

	// Update offset to after header
	bytesRead := bitReader.bytesRead()
	pd.offset += bytesRead
	header := pd.data[headerStart : headerStart+bytesRead]

	return header, cbIncls, nil
}

// bitReader helps with bit-level reading
type bitReader struct {
	data     []byte
	bytePos  int
	bitPos   int
	totalBit int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{
		data:    data,
		bytePos: 0,
		bitPos:  0,
	}
}

func (br *bitReader) readBit() (int, error) {
	if br.bytePos >= len(br.data) {
		return 0, fmt.Errorf("end of data")
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
	if br.bitPos > 0 {
		return br.bytePos + 1
	}
	return br.bytePos
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

// GetPackets returns the decoded packets
func (pd *PacketDecoder) GetPackets() []Packet {
	return pd.packets
}
