package t2

import (
	"fmt"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

// decodeAllCodeBlocksFixed is the corrected version that properly handles subbands
func (td *TileDecoder) decodeAllCodeBlocksFixed(packets []Packet) error {
	cbWidth, cbHeight := td.cod.CodeBlockSize()

	// Process each component
	for _, comp := range td.components {
		// Group code-blocks by resolution to track their spatial layout
		type cbInfo struct {
			data           []byte
			maxBitplane    int
			maxBitplaneSet bool // Track if maxBitplane has been set
			resolution     int
			cbIdx          int   // Code-block index within the resolution
			passLengths    []int // Cumulative pass lengths (for TERMALL)
			useTERMALL     bool  // TERMALL mode flag
			totalPasses    int   // Total passes accumulated across all layers
		}
		cbDataMap := make(map[string]cbInfo) // map[key]cbInfo where key = "res:cbIdx"

		for i := range packets {
			packet := &packets[i]
			if packet.ComponentIndex != comp.componentIdx {
				continue
			}


			// Extract code-block contributions from this packet
			dataOffset := 0
			for cbIdx, cbIncl := range packet.CodeBlockIncls {
				if cbIncl.Included && cbIncl.DataLength > 0 {
					if dataOffset+cbIncl.DataLength <= len(packet.Body) {
						// Accumulate code-block data
						cbData := packet.Body[dataOffset : dataOffset+cbIncl.DataLength]

						// Create unique key: "resolution:cbIdx"
						key := fmt.Sprintf("%d:%d", packet.ResolutionLevel, cbIdx)
						existing, ok := cbDataMap[key]

						// DEBUG CB 0:0 data accumulation
						if key == "0:0" {
							showBytes := 3
							if len(cbData) < showBytes {
								showBytes = len(cbData)
							}
							fmt.Printf("[DATA ACCUM Layer=%d CB 0:0] cbData len=%d, existing len=%d, first bytes=%02x\n",
								packet.LayerIndex, len(cbData), len(existing.data), cbData[:showBytes])
						}

						// Save base offset BEFORE appending data
						baseOffset := len(existing.data)

						if ok {
							// Append to existing data
							existing.data = append(existing.data, cbData...)
						} else {
							existing.data = cbData
							existing.resolution = packet.ResolutionLevel
							existing.cbIdx = cbIdx
						}

						dataOffset += cbIncl.DataLength

						// Accumulate PassLengths from each layer
						if cbIncl.PassLengths != nil && len(cbIncl.PassLengths) > 0 {
							if existing.passLengths == nil {
								// First layer: use as-is
								existing.passLengths = make([]int, len(cbIncl.PassLengths))
								copy(existing.passLengths, cbIncl.PassLengths)

								// DEBUG first CB
								if key == "0:0" {
									fmt.Printf("[FIXED ACCUM Layer=%d CB 0:0] FIRST passLengths=%v dataLen=%d\n",
										packet.LayerIndex, existing.passLengths, len(existing.data))
								}
							} else {
								// Subsequent layers: append with offset (baseOffset was saved before append)
								if key == "0:0" {
									fmt.Printf("[FIXED ACCUM Layer=%d CB 0:0] BEFORE baseOffset=%d cbIncl.PassLengths=%v existing.data=%d cbData=%d\n",
										packet.LayerIndex, baseOffset, cbIncl.PassLengths, len(existing.data), len(cbData))
								}

								for _, passLen := range cbIncl.PassLengths {
									existing.passLengths = append(existing.passLengths, baseOffset+passLen)
								}

								if key == "0:0" {
									fmt.Printf("[FIXED ACCUM Layer=%d CB 0:0] AFTER accumulated=%v totalDataLen=%d\n",
										packet.LayerIndex, existing.passLengths, len(existing.data))
								}
							}

						}
					if cbIncl.UseTERMALL {
						existing.useTERMALL = true
					}

					// Accumulate total passes across layers
					existing.totalPasses += cbIncl.NumPasses

					// Calculate max bitplane from zero bitplanes
						// Note: After wavelet transform, coefficients may need extra bits
						// 5/3 reversible wavelet adds 1 bit per decomposition level
						componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
						effectiveBitDepth := componentBitDepth + comp.numLevels
						maxBP := effectiveBitDepth - 1 - cbIncl.ZeroBitplanes
						if !existing.maxBitplaneSet || maxBP > existing.maxBitplane {
							existing.maxBitplane = maxBP
												existing.maxBitplaneSet = true
						}

						cbDataMap[key] = existing
					}
				}
			}
		}


		// Calculate subband dimensions for each resolution
		// Resolution 0: LL subband (top-left after all decompositions)
		// Resolution r > 0: HL, LH, HH subbands from decomposition level (numLevels - r + 1)

		type subbandLayout struct {
			x0, y0         int // Subband origin in coefficient array
			width, height  int // Subband dimensions
			numCBX, numCBY int // Number of code-blocks in X and Y
		}

		// Calculate layout for each resolution
		resolutionLayouts := make(map[int][]subbandLayout) // [resolution][]subbandLayout

		for res := 0; res <= comp.numLevels; res++ {
			if res == 0 {
				// Resolution 0: LL subband only
				llWidth := comp.width >> comp.numLevels
				llHeight := comp.height >> comp.numLevels
				numCBX := (llWidth + cbWidth - 1) / cbWidth
				numCBY := (llHeight + cbHeight - 1) / cbHeight

				resolutionLayouts[res] = []subbandLayout{
					{x0: 0, y0: 0, width: llWidth, height: llHeight, numCBX: numCBX, numCBY: numCBY},
				}
			} else {
				// Resolution r: HL, LH, HH subbands
				level := comp.numLevels - res + 1
				sbWidth := comp.width >> level
				sbHeight := comp.height >> level
				numCBX := (sbWidth + cbWidth - 1) / cbWidth
				numCBY := (sbHeight + cbHeight - 1) / cbHeight

				resolutionLayouts[res] = []subbandLayout{
					{x0: sbWidth, y0: 0, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY},         // HL
					{x0: 0, y0: sbHeight, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY},        // LH
					{x0: sbWidth, y0: sbHeight, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY}, // HH
				}
			}
		}

		// Create code-blocks with correct spatial positions
		codeBlocks := make([]*CodeBlockDecoder, 0)

		// Convert map to sorted slice to ensure deterministic processing order
		// Sort by resolution first, then by code block index
		cbList := make([]cbInfo, 0, len(cbDataMap))
		for _, info := range cbDataMap {
			cbList = append(cbList, info)
		}


		// Sort by resolution, then by cbIdx
		sort.Slice(cbList, func(i, j int) bool {
			if cbList[i].resolution != cbList[j].resolution {
				return cbList[i].resolution < cbList[j].resolution
			}
			return cbList[i].cbIdx < cbList[j].cbIdx
		})

		for _, cbInfo := range cbList {
			res := cbInfo.resolution
			cbIdx := cbInfo.cbIdx
			layouts := resolutionLayouts[res]


			// Calculate which subband this code-block belongs to
			subbandIdx := 0
			localCBIdx := cbIdx
			totalCBsPerSubband := 0

			// For resolution 0, there's only one subband (LL)
			// For resolution > 0, code-blocks are ordered as: HL, LH, HH
			if res > 0 {
				totalCBsPerSubband = layouts[0].numCBX * layouts[0].numCBY
				subbandIdx = localCBIdx / totalCBsPerSubband
				if subbandIdx >= len(layouts) {
					subbandIdx = len(layouts) - 1
				}
				localCBIdx = localCBIdx % totalCBsPerSubband
			}

			layout := layouts[subbandIdx]

			// Calculate code-block position within subband
			cbx := localCBIdx % layout.numCBX
			cby := localCBIdx / layout.numCBX

			// Calculate code-block bounds within the subband
			localX0 := cbx * cbWidth
			localY0 := cby * cbHeight
			localX1 := localX0 + cbWidth
			localY1 := localY0 + cbHeight

			// Clip to subband bounds
			if localX1 > layout.width {
				localX1 = layout.width
			}
			if localY1 > layout.height {
				localY1 = layout.height
			}

			// Convert to global coordinates in coefficient array
			x0 := layout.x0 + localX0
			y0 := layout.y0 + localY0
			x1 := layout.x0 + localX1
			y1 := layout.y0 + localY1

			actualWidth := x1 - x0
			actualHeight := y1 - y0

			if actualWidth <= 0 || actualHeight <= 0 {
				continue
			}

			// Create code-block decoder
			// Use accumulated totalPasses from all layers, or calculate from maxBitplane if not available
			numPasses := cbInfo.totalPasses
			if numPasses == 0 {
				// Fallback: calculate from maxBitplane (3 passes per bitplane)
				numPasses = (cbInfo.maxBitplane + 1) * 3
			}

			cbd := &CodeBlockDecoder{
				x0:        x0,
				y0:        y0,
				x1:        x1,
				y1:        y1,
				data:      cbInfo.data,
				numPasses: numPasses,
				t1Decoder: t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle)),
			}

			// Decode the code-block
			if len(cbInfo.data) > 0 {
				var err error
			if cbInfo.passLengths != nil && len(cbInfo.passLengths) > 0 {
				// Multi-layer mode: use DecodeLayeredWithMode
				// useTERMALL flag determines whether passes are terminated independently
				// lossless flag (from COD transformation) determines whether contexts are reset
				lossless := td.cod.Transformation == 1 // 1 = 5/3 reversible (lossless)
				if res == 0 && cbIdx == 0 {
					fmt.Printf("[T1 DECODE CB 0:0] Multi-layer mode: dataLen=%d, passLengths=%v, maxBitplane=%d, useTERMALL=%v, lossless=%v\n",
						len(cbInfo.data), cbInfo.passLengths, cbInfo.maxBitplane, cbInfo.useTERMALL, lossless)
				}
				err = cbd.t1Decoder.DecodeLayeredWithMode(cbInfo.data, cbInfo.passLengths, cbInfo.maxBitplane, 0, cbInfo.useTERMALL, lossless)
			} else {
				// Single-layer mode: use DecodeWithBitplane
				if res == 0 && cbIdx == 0 {
					fmt.Printf("[T1 DECODE CB 0:0] Single-layer mode: dataLen=%d, numPasses=%d, maxBitplane=%d, data=%02x\n",
						len(cbInfo.data), cbd.numPasses, cbInfo.maxBitplane, cbInfo.data)
				}
				err = cbd.t1Decoder.DecodeWithBitplane(cbInfo.data, cbd.numPasses, cbInfo.maxBitplane, 0)
				if res == 0 && cbIdx == 0 {
					fmt.Printf("[T1 DECODE CB 0:0] DecodeWithBitplane returned err=%v\n", err)
				}
			}
				if err != nil {
					cbd.coeffs = make([]int32, actualWidth*actualHeight)
				} else {
					cbd.coeffs = cbd.t1Decoder.GetData()
				}
			} else {
				cbd.coeffs = make([]int32, actualWidth*actualHeight)
			}

			codeBlocks = append(codeBlocks, cbd)
		}

		// Store code-blocks for assembly
		comp.resolutions = make([]*ResolutionLevel, comp.numLevels+1)
		comp.codeBlocks = codeBlocks
	}

	return nil
}
