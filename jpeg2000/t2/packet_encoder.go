package t2

import (
	"bytes"
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

	// Encode packet header
	header, cbIncls, err := pe.encodePacketHeader(precinct, layer)
	if err != nil {
		return packet, fmt.Errorf("failed to encode packet header: %w", err)
	}
	packet.Header = header
	packet.CodeBlockIncls = cbIncls

	// Encode packet body (code-block contributions)
	body := &bytes.Buffer{}
	for _, cbIncl := range cbIncls {
		if cbIncl.Included {
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
			bitBuf.writeBit((dataLen >> i) & 1)
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

// GetPackets returns the encoded packets
func (pe *PacketEncoder) GetPackets() []Packet {
	return pe.packets
}
