package t2

import (
	"fmt"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

// encodePacketHeaderWithTagTree encodes a packet header using tag-tree encoding
// This matches OpenJPEG's approach and achieves much better compression
func (pe *PacketEncoder) encodePacketHeaderWithTagTree(precinct *Precinct, layer int, _ int) ([]byte, []CodeBlockIncl, error) {
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

	// Reset tag-trees only on first layer (OpenJPEG semantics)
	if layer == 0 {
		precinct.InclTree.ResetEncoding()
		precinct.ZBPTree.ResetEncoding()
	}

	// First pass: populate tag-trees with values for this layer
	for _, cb := range precinct.CodeBlocks {
		// Determine inclusion layer for this codeblock
		if !cb.Included {
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
				hasData := len(cb.Data) > 0
				included = hasData
			}
			if included {
				precinct.InclTree.SetValue(cb.CBX, cb.CBY, layer)
			}
		}

		if layer == 0 {
			precinct.ZBPTree.SetValue(cb.CBX, cb.CBY, cb.ZeroBitPlanes)
		}
	}

	// Create bit writer (OpenJPEG-style bit stuffing)
	bitBuf := newBioWriter()

	// Write packet present flag (1 bit) - matches OpenJPEG's opj_t2_encode_packet
	// 0 = empty packet, 1 = packet has data
	if len(precinct.CodeBlocks) == 0 {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
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
			hasData := len(cb.Data) > 0
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

		cbIncl.DataLength = dataLen

		// Encode length using OpenJPEG-style comma code + length indicators.
		if cb.NumLenBits <= 0 {
			cb.NumLenBits = 3
		}

		prevPasses := 0
		totalPasses := newPasses
		if cb.LayerPasses != nil && layer < len(cb.LayerPasses) {
			totalPasses = cb.LayerPasses[layer]
			if layer > 0 {
				prevPasses = cb.LayerPasses[layer-1]
			}
		} else if cb.NumPassesTotal > 0 {
			prevPasses = cb.NumPassesTotal - newPasses
			if prevPasses < 0 {
				prevPasses = 0
			}
			totalPasses = prevPasses + newPasses
		}

		termAll := cb.UseTERMALL
		passLens := buildPassLengths(cb.PassLengths, cb.Passes)
		if termAll && (passLens == nil || totalPasses > len(passLens)) {
			termAll = false
		}

		encodeCodeBlockLengths(bitBuf, cb, cbIncl.DataLength, prevPasses, newPasses, termAll, passLens)

		cbIncls = append(cbIncls, cbIncl)
	}

	// Flush remaining bits
	return bitBuf.flush(), cbIncls, nil
}

// encodePacketHeaderWithTagTreeMulti encodes a packet header across all bands in a precinct.
// It writes a single packet-present bit, then iterates bands in order.
func (pe *PacketEncoder) encodePacketHeaderWithTagTreeMulti(precincts []*Precinct, layer int) ([]byte, []CodeBlockIncl, error) {
	cbIncls := make([]CodeBlockIncl, 0)
	bitBuf := newBioWriter()

	if len(precincts) == 0 {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
	}

	hasBlocks := false
	layerContribution := func(cb *PrecinctCodeBlock) (bool, int) {
		included := false
		newPasses := 0
		if cb.LayerData != nil && layer < len(cb.LayerData) {
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
			included = len(cb.Data) > 0
			newPasses = cb.NumPassesTotal
		}
		return included, newPasses
	}

	for _, precinct := range precincts {
		if precinct != nil && len(precinct.CodeBlocks) > 0 {
			hasBlocks = true
			break
		}
	}

	if !hasBlocks {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
	}
	bitBuf.writeBit(1)

	for _, precinct := range precincts {
		if precinct == nil || len(precinct.CodeBlocks) == 0 {
			continue
		}
		sort.Slice(precinct.CodeBlocks, func(i, j int) bool {
			ai := precinct.CodeBlocks[i]
			aj := precinct.CodeBlocks[j]
			if ai.CBY != aj.CBY {
				return ai.CBY < aj.CBY
			}
			return ai.CBX < aj.CBX
		})

		if precinct.NumCodeBlocksX == 0 || precinct.NumCodeBlocksY == 0 {
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

		if precinct.InclTree == nil || precinct.ZBPTree == nil ||
			precinct.InclTree.Width() != precinct.NumCodeBlocksX ||
			precinct.InclTree.Height() != precinct.NumCodeBlocksY {
			precinct.InclTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
			precinct.ZBPTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
		}

		if layer == 0 {
			precinct.InclTree.ResetEncoding()
			precinct.ZBPTree.ResetEncoding()
		}

		for _, cb := range precinct.CodeBlocks {
			if !cb.Included {
				included, _ := layerContribution(cb)
				if included {
					precinct.InclTree.SetValue(cb.CBX, cb.CBY, layer)
				}
			}
			if layer == 0 {
				precinct.ZBPTree.SetValue(cb.CBX, cb.CBY, cb.ZeroBitPlanes)
			}
		}
	}

	for _, precinct := range precincts {
		if precinct == nil || len(precinct.CodeBlocks) == 0 {
			continue
		}

		for _, cb := range precinct.CodeBlocks {
			included, newPasses := layerContribution(cb)
			firstIncl := !cb.Included && included

			cbIncl := CodeBlockIncl{
				Included:       included,
				FirstInclusion: firstIncl,
			}

			if !cb.Included {
				threshold := layer + 1
				if err := precinct.InclTree.Encode(bitBuf, cb.CBX, cb.CBY, threshold); err != nil {
					return nil, nil, fmt.Errorf("failed to encode inclusion tag-tree: %w", err)
				}

				if !included {
					cbIncls = append(cbIncls, cbIncl)
					continue
				}

				if err := precinct.ZBPTree.Encode(bitBuf, cb.CBX, cb.CBY, 999); err != nil {
					return nil, nil, fmt.Errorf("failed to encode zero-bitplane tag-tree: %w", err)
				}

				cb.Included = true
			} else {
				if included {
					bitBuf.writeBit(1)
				} else {
					bitBuf.writeBit(0)
					cbIncls = append(cbIncls, cbIncl)
					continue
				}
			}

			cbIncl.NumPasses = newPasses
			if err := encodeNumPasses(bitBuf, newPasses); err != nil {
				return nil, nil, fmt.Errorf("failed to encode number of passes: %w", err)
			}

			var layerData []byte
			if cb.LayerData != nil && layer < len(cb.LayerData) {
				layerData = cb.LayerData[layer]
			} else {
				layerData = cb.Data
			}
			dataLen := len(layerData)
			cbIncl.Data = layerData

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

			cbIncl.DataLength = dataLen

			prevPasses := 0
			totalPasses := newPasses
			if cb.LayerPasses != nil && layer < len(cb.LayerPasses) {
				totalPasses = cb.LayerPasses[layer]
				if layer > 0 {
					prevPasses = cb.LayerPasses[layer-1]
				}
			} else if cb.NumPassesTotal > 0 {
				prevPasses = cb.NumPassesTotal - newPasses
				if prevPasses < 0 {
					prevPasses = 0
				}
				totalPasses = prevPasses + newPasses
			}

			termAll := cb.UseTERMALL
			passLens := buildPassLengths(cb.PassLengths, cb.Passes)
			if termAll && (passLens == nil || totalPasses > len(passLens)) {
				termAll = false
			}

			encodeCodeBlockLengths(bitBuf, cb, cbIncl.DataLength, prevPasses, newPasses, termAll, passLens)

			cbIncls = append(cbIncls, cbIncl)
		}
	}

	return bitBuf.flush(), cbIncls, nil
}

// encodeNumPasses encodes the number of coding passes using JPEG2000 standard encoding
// Matches OpenJPEG's opj_t2_putnumpasses() in t2.c:184-198
type packetBitWriter interface {
	writeBit(int)
	writeBits(int, int)
}

func encodeNumPasses(bw packetBitWriter, n int) error {
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
		// 6-36 passes: "1111xxxxx" (9 bits total)
		// 0x1e0 = 0b111100000 (prefix 1111, then 5 bits for value)
		// OpenJPEG: opj_bio_write(bio, 0x1e0 | (n - 6), 9)
		val := 0x1e0 | (n - 6)
		bw.writeBits(val, 9)
	} else if n <= 164 {
		// 37-164 passes: "111111111" + 7-bit value (16 bits total)
		// 0xff80 = 0b1111111110000000 (prefix 111111111, then 7 bits for value)
		// OpenJPEG: opj_bio_write(bio, 0xff80 | (n - 37), 16)
		val := 0xff80 | (n - 37)
		bw.writeBits(val, 16)
	} else {
		return fmt.Errorf("number of passes %d exceeds maximum 164", n)
	}
	return nil
}

func encodeCodeBlockLengths(bw packetBitWriter, cb *PrecinctCodeBlock, dataLen, prevPasses, newPasses int, termAll bool, passLens []int) {
	if newPasses <= 0 {
		encodeCommaCode(bw, 0)
		return
	}
	if cb.NumLenBits <= 0 {
		cb.NumLenBits = 3
	}

	// Fallback: no per-pass lengths, emit a single segment length.
	if passLens == nil || prevPasses+newPasses > len(passLens) {
		increment := (floorLog2(dataLen) + 1) - (cb.NumLenBits + floorLog2(newPasses))
		if increment < 0 {
			increment = 0
		}
		encodeCommaCode(bw, increment)
		cb.NumLenBits += increment
		bitCount := cb.NumLenBits + floorLog2(newPasses)
		bw.writeBits(dataLen, bitCount)
		return
	}

	increment := 0
	nump := 0
	segLen := 0
	lastPass := prevPasses + newPasses - 1
	for passIdx := prevPasses; passIdx <= lastPass; passIdx++ {
		nump++
		segLen += passLens[passIdx]
		terminate := termAll || passIdx == lastPass
		if terminate {
			need := (floorLog2(segLen) + 1) - (cb.NumLenBits + floorLog2(nump))
			if need > increment {
				increment = need
			}
			segLen = 0
			nump = 0
		}
	}
	if increment < 0 {
		increment = 0
	}
	encodeCommaCode(bw, increment)
	cb.NumLenBits += increment

	nump = 0
	segLen = 0
	for passIdx := prevPasses; passIdx <= lastPass; passIdx++ {
		nump++
		segLen += passLens[passIdx]
		terminate := termAll || passIdx == lastPass
		if terminate {
			bitCount := cb.NumLenBits + floorLog2(nump)
			bw.writeBits(segLen, bitCount)
			segLen = 0
			nump = 0
		}
	}
}

func buildPassLengths(cumulative []int, passes []t1.PassData) []int {
	if len(cumulative) > 0 {
		out := make([]int, len(cumulative))
		prev := 0
		for i, v := range cumulative {
			if v < prev {
				v = prev
			}
			out[i] = v - prev
			prev = v
		}
		return out
	}
	if len(passes) > 0 {
		out := make([]int, len(passes))
		for i, p := range passes {
			if p.Len > 0 {
				out[i] = p.Len
			} else if p.ActualBytes > 0 {
				out[i] = p.ActualBytes
			}
		}
		return out
	}
	return nil
}

func encodeCommaCode(bw packetBitWriter, n int) {
	for i := 0; i < n; i++ {
		bw.writeBit(1)
	}
	bw.writeBit(0)
}
