package t2

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
		pe.precincts[component][resolution][precinctIdx] = []*Precinct{{
			Index:      precinctIdx,
			CodeBlocks: make([]*PrecinctCodeBlock, 0),
		}}
	}

	// Add code-block to precinct
	precinct := pe.precincts[component][resolution][precinctIdx][0]
	precinct.CodeBlocks = append(precinct.CodeBlocks, codeBlock)
}

// EncodePackets encodes all packets according to progression order
func (pe *PacketEncoder) EncodePackets() ([]Packet, error) {
	pe.packets = make([]Packet, 0)

	switch pe.progression {
	case ProgressionLRCP:
		return pe.encodeLRCP()
	case ProgressionRLCP:
		return pe.encodeRLCP()
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

				for precinctIdx, precincts := range pe.precincts[comp][res] {
					for _, precinct := range precincts {
						packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precinct)
						if err != nil {
							return nil, fmt.Errorf("failed to encode packet (L=%d,R=%d,C=%d,P=%d): %w",
								layer, res, comp, precinctIdx, err)
						}
						pe.packets = append(pe.packets, packet)
					}
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

				for precinctIdx, precincts := range pe.precincts[comp][res] {
					for _, precinct := range precincts {
						packet, err := pe.encodePacket(layer, res, comp, precinctIdx, precinct)
						if err != nil {
							return nil, fmt.Errorf("failed to encode packet (R=%d,L=%d,C=%d,P=%d): %w",
								res, layer, comp, precinctIdx, err)
						}
						pe.packets = append(pe.packets, packet)
					}
				}
			}
		}
	}

	return pe.packets, nil
}

// encodePacket encodes a single packet
func (pe *PacketEncoder) encodePacket(layer, resolution, component, precinctIdx int, precinct *Precinct) (Packet, error) {
	// DEBUG
	if resolution == 0 && layer < 2 {
		fmt.Printf("[ENCODE PACKET] Layer=%d Res=%d Comp=%d numCB=%d\n",
			layer, resolution, component, len(precinct.CodeBlocks))
	}

	packet := Packet{
		LayerIndex:      layer,
		ResolutionLevel: resolution,
		ComponentIndex:  component,
		PrecinctIndex:   precinctIdx,
		HeaderPresent:   len(precinct.CodeBlocks) > 0,
	}

	if !packet.HeaderPresent {
		// Empty packet
		return packet, nil
	}

	// Encode packet header with layer awareness
	header, cbIncls, err := pe.encodePacketHeaderLayered(precinct, layer, resolution)
	if err != nil {
		return packet, fmt.Errorf("failed to encode packet header: %w", err)
	}
	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Encode packet body (code-block contributions for this layer)
	body := &bytes.Buffer{}
	for i := range cbIncls {
		cbIncl := &cbIncls[i]
		if cbIncl.Included {
			// In TERMALL mode, prepend PassLengths metadata before code-block data
			if cbIncl.UseTERMALL && len(cbIncl.PassLengths) > 0 {
				// Write number of passes (1 byte)
				numPasses := byte(len(cbIncl.PassLengths))
				body.WriteByte(numPasses)
				// Write each pass length (2 bytes each, big-endian)
				for _, passLen := range cbIncl.PassLengths {
					_ = binary.Write(body, binary.BigEndian, uint16(passLen))
				}
				// Note: DataLength was already updated in encodePacketHeaderLayered
			}
			// Write code-block data (will be byte-stuffed later by encoder.go)
			body.Write(cbIncl.Data)
		}
	}
	packet.Body = body.Bytes()

	return packet, nil
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

		// Encode number of passes (simplified - should use more efficient encoding)
		// Using a simple unary code
		for i := 0; i < numPasses; i++ {
			if i < numPasses-1 {
				bitBuf.writeBit(0)
			} else {
				bitBuf.writeBit(1)
			}
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

				cbKey := fmt.Sprintf("%d:%d", resolution, cb.Index)
				if cbKey == "0:0" && layer < 2 {
					fmt.Printf("[PACKET ENC Layer=%d CB 0:0] LayerPasses=%v, totalPasses=%d, prevPasses=%d, newPasses=%d, included=%v\n",
						layer, cb.LayerPasses, totalPasses, prevPasses, newPasses, included)
				}
			}
		} else {
			// Fallback: use old single-layer method
			included = cb.Data != nil && len(cb.Data) > 0
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
		} else {
			bitBuf.writeBit(0)
			cbIncls = append(cbIncls, cbIncl)
			continue
		}

		if fmt.Sprintf("%d:%d", resolution, cb.Index) == "0:0" && layer < 2 {
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] Wrote inclusion bit=1, bitCount=%d\n",
				layer, bitBuf.bitCount)
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

		if fmt.Sprintf("%d:%d", resolution, cb.Index) == "0:0" && layer < 2 {
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] Before encoding passes: bitCount=%d, will encode %d passes\n",
				layer, bitBuf.bitCount, newPasses)
		}

		// Encode number of passes (simplified unary code)
		for i := 0; i < newPasses; i++ {
			if i < newPasses-1 {
				bitBuf.writeBit(0)
			} else {
				bitBuf.writeBit(1)
			}
		}

		if fmt.Sprintf("%d:%d", resolution, cb.Index) == "0:0" && layer < 2 {
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] After encoding passes: bitCount=%d\n",
				layer, bitBuf.bitCount)
		}

		// Get data for this layer
		var layerData []byte

		// DEBUG: Check if we're encoding multi-layer
		cbKey := fmt.Sprintf("%d:%d", resolution, cb.Index)
		if cbKey == "0:0" && layer < 2 {
			if cb.LayerData != nil {
				fmt.Printf("[PACKET ENC Layer=%d CB 0:0] Multi-layer mode, %d layers\n", layer, len(cb.LayerData))
			} else {
				fmt.Printf("[PACKET ENC Layer=%d CB 0:0] Single-layer mode\n", layer)
			}
		}

		if cb.LayerData != nil && layer < len(cb.LayerData) {
			// Multi-layer: use pre-calculated layer data (incremental)
			layerData = cb.LayerData[layer]

			if cbKey == "0:0" {
				showBytes := 3
				if len(layerData) < showBytes {
					showBytes = len(layerData)
				}
				fmt.Printf("[PACKET ENC Layer=%d CB 0:0] layerData len=%d, first bytes=%02x\n",
					layer, len(layerData), layerData[:showBytes])
			}
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

		if cbKey == "0:0" && layer < 2 {
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] Before DataLength: bitCount=%d, bytePos=%d\n",
				layer, bitBuf.bitCount, bitBuf.buf.Len())
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] DataLength=%d (0x%04x)\n",
				layer, cbIncl.DataLength, cbIncl.DataLength)
		}

		for i := 15; i >= 0; i-- {
			bit := (encodedLength >> i) & 1
			bitBuf.writeBit(bit)
		}

		if cbKey == "0:0" && layer < 2 {
			fmt.Printf("[PACKET ENC Layer=%d CB 0:0] After DataLength: bitCount=%d, bytePos=%d\n",
				layer, bitBuf.bitCount, bitBuf.buf.Len())
		}

		cbIncls = append(cbIncls, cbIncl)
	}

	// Flush remaining bits
	bitBuf.flush()

	headerBytes := header.Bytes()
	if layer < 2 && resolution == 0 {
		fmt.Printf("[PACKET ENC Header Layer=%d Res=%d] Header length=%d bytes\n",
			layer, resolution, len(headerBytes))
		showBytes := 10
		if len(headerBytes) < showBytes {
			showBytes = len(headerBytes)
		}
		fmt.Printf("[PACKET ENC Header Layer=%d Res=%d] Header bytes: %02x\n",
			layer, resolution, headerBytes[:showBytes])
	}

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
