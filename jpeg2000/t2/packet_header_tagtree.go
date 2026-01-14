package t2

import (
	"bytes"
	"fmt"
)

// encodePacketHeaderWithTagTree encodes a packet header using tag-tree encoding
// This matches OpenJPEG's approach and achieves much better compression
func (pe *PacketEncoder) encodePacketHeaderWithTagTree(precinct *Precinct, layer int, resolution int) ([]byte, []CodeBlockIncl, error) {
	header := &bytes.Buffer{}
	cbIncls := make([]CodeBlockIncl, 0)

	// Ensure precinct has grid dimensions
	if precinct.NumCodeBlocksX == 0 || precinct.NumCodeBlocksY == 0 {
		// Calculate from codeblocks
		maxX, maxY := 0, 0
		for _, cb := range precinct.CodeBlocks {
			if cb.CBX+1 > maxX {
				maxX = cb.CBX + 1
			}
			if cb.CBY+1 > maxY {
				maxY = cb.CBY + 1
			}
		}
		precinct.NumCodeBlocksX = maxX
		precinct.NumCodeBlocksY = maxY
	}

	// Create tag-trees for this packet if needed
	if precinct.InclTree == nil || precinct.ZBPTree == nil {
		precinct.InclTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
		precinct.ZBPTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
	}

	// Reset tag-trees for new packet encoding
	precinct.InclTree.ResetEncoding()
	precinct.ZBPTree.ResetEncoding()

	// First pass: populate tag-trees with values for all codeblocks
	for _, cb := range precinct.CodeBlocks {
		// Determine inclusion layer for this codeblock
		var inclLayer int
		if cb.Included {
			// Already included in previous packet - set to 0
			inclLayer = 0
		} else {
			// Check if included in current layer
			included := false
			if cb.LayerData != nil && layer < len(cb.LayerData) {
				if layer < len(cb.LayerPasses) {
					totalPasses := cb.LayerPasses[layer]
					prevPasses := 0
					if layer > 0 {
						prevPasses = cb.LayerPasses[layer-1]
					}
					newPasses := totalPasses - prevPasses
					included = newPasses > 0
				}
			} else {
				hasData := cb.Data != nil && len(cb.Data) > 0
				included = hasData
			}

			if included {
				inclLayer = layer
			} else {
				inclLayer = 999 // Not included
			}
		}

		// Set values in tag-trees (only for codeblocks that exist)
		precinct.InclTree.SetValue(cb.CBX, cb.CBY, inclLayer)
		precinct.ZBPTree.SetValue(cb.CBX, cb.CBY, cb.ZeroBitPlanes)
	}

	// Create bit writer
	bitBuf := newBitWriter(header)

	// Write packet present flag (1 bit) - matches OpenJPEG's opj_t2_encode_packet
	// 0 = empty packet, 1 = packet has data
	if len(precinct.CodeBlocks) == 0 {
		bitBuf.writeBit(0)
		bitBuf.flush()
		return header.Bytes(), cbIncls, nil
	}
	bitBuf.writeBit(1) // Packet has data

	// Second pass: encode packet header for each codeblock in stored order
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

		// Encode inclusion using tag-tree
		if !cb.Included {
			// First time this codeblock might be included
			// Use tag-tree to encode inclusion layer
			threshold := layer + 1
			err := precinct.InclTree.Encode(bitBuf, cb.CBX, cb.CBY, threshold)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to encode inclusion tag-tree: %w", err)
			}

			if !included {
				// Not included in this layer
				cbIncls = append(cbIncls, cbIncl)
				continue
			}

			// First inclusion - encode zero bitplanes using tag-tree
			err = precinct.ZBPTree.Encode(bitBuf, cb.CBX, cb.CBY, 999)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to encode zero-bitplane tag-tree: %w", err)
			}

			cb.Included = true
		} else {
			// Already included in previous layer
			if included {
				// Still has data in this layer
				bitBuf.writeBit(1)
			} else {
				// No data in this layer
				bitBuf.writeBit(0)
				cbIncls = append(cbIncls, cbIncl)
				continue
			}
		}

		// Encode number of coding passes for this layer
		cbIncl.NumPasses = newPasses

		// Encode number of passes using JPEG2000 standard encoding
		// Matches OpenJPEG's opj_t2_putnumpasses() in t2.c:184-198
		if err := encodeNumPasses(bitBuf, newPasses); err != nil {
			return nil, nil, fmt.Errorf("failed to encode number of passes: %w", err)
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
		if cb.LayerData != nil && layer < len(cb.LayerPasses) {
			totalPasses := cb.LayerPasses[layer]
			prevPasses := 0
			if layer > 0 {
				prevPasses = cb.LayerPasses[layer-1]
			}

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

// encodeNumPasses encodes the number of coding passes using JPEG2000 standard encoding
// Matches OpenJPEG's opj_t2_putnumpasses() in t2.c:184-198
func encodeNumPasses(bw *bitWriter, n int) error {
	if n == 1 {
		// 1 pass: "0" (1 bit)
		bw.writeBit(0)
	} else if n == 2 {
		// 2 passes: "10" (2 bits)
		bw.writeBits(2, 2) // value=2 (0b10), bits=2
	} else if n <= 5 {
		// 3-5 passes: "11xx" (4 bits)
		// 0xc = 0b1100, combined with (n-3) in lower 2 bits
		val := 0x0c | (n - 3)
		bw.writeBits(val, 4)
	} else if n <= 36 {
		// 6-36 passes: "111100000" + 5-bit value (9 bits total)
		// 0x1e0 = 0b111100000 (upper 4 bits are 1111, next 5 bits are 00000)
		val := 0x1e0 | (n - 6)
		bw.writeBits(val, 9)
	} else if n <= 164 {
		// 37-164 passes: "1111111110000000" + 7-bit value (16 bits total)
		// 0xff80 = 0b1111111110000000 (upper 9 bits are 1, next 7 bits are 0)
		val := 0xff80 | (n - 37)
		bw.writeBits(val, 16)
	} else {
		return fmt.Errorf("number of passes %d exceeds maximum 164", n)
	}
	return nil
}
