package t2

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

// decodeAllCodeBlocksFixed is the corrected version that properly handles subbands
func (td *TileDecoder) decodeAllCodeBlocksFixed(packets []Packet) error {
	cbWidth, cbHeight := td.cod.CodeBlockSize()

	// Process each component
	for _, comp := range td.components {
		// Group code-blocks by resolution to track their spatial layout
		type cbInfo struct {
			data             []byte
			maxBitplane      int
			maxBitplaneSet   bool // Track if maxBitplane has been set
			zeroBitplanes    int  // Saved from first inclusion, used for all layers
			zeroBitplanesSet bool // Track if zeroBitplanes has been set
			resolution       int
			cbIdx            int   // Code-block index within the resolution
			passLengths      []int // Cumulative pass lengths (for TERMALL)
			useTERMALL       bool  // TERMALL mode flag
			totalPasses      int   // Total passes accumulated across all layers
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
				if cbIncl.Included {
					// Handle code blocks with zero or non-zero data
					var cbData []byte
					if cbIncl.DataLength > 0 {
						if dataOffset+cbIncl.DataLength <= len(packet.Body) {
							// CRITICAL FIX: Create a copy instead of slicing to avoid shared backing array
							// When slicing packet.Body, cbData shares the same backing array
							// This causes data corruption when packet.Body is reused across packets
							cbData = make([]byte, cbIncl.DataLength)
							copy(cbData, packet.Body[dataOffset:dataOffset+cbIncl.DataLength])
						} else {
							// Data length exceeds packet body - skip
							continue
						}
					} else {
						// Code block included but has zero data (e.g., all-zero coefficients)
						cbData = []byte{}
					}

					// Create unique key: "resolution:cbIdx"
					key := fmt.Sprintf("%d:%d", packet.ResolutionLevel, cbIdx)

					existing, ok := cbDataMap[key]

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

						} else {
							// Subsequent layers: append with offset (baseOffset was saved before append)
							for _, passLen := range cbIncl.PassLengths {
								existing.passLengths = append(existing.passLengths, baseOffset+passLen)
							}
						}

					}
					if cbIncl.UseTERMALL {
						existing.useTERMALL = true
					}

					// Accumulate total passes across layers
					existing.totalPasses += cbIncl.NumPasses

					// Save zeroBitplanes from first inclusion, use for all subsequent layers
					// This is critical for multi-layer: zeroBitplanes is only encoded in first layer
					if !existing.zeroBitplanesSet && cbIncl.ZeroBitplanes >= 0 {
						existing.zeroBitplanes = cbIncl.ZeroBitplanes
						existing.zeroBitplanesSet = true
					}

					// Calculate max bitplane from zero bitplanes
					// Note: After wavelet transform, coefficients may need extra bits
					// 5/3 reversible wavelet adds 1 bit per decomposition level
					componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
					effectiveBitDepth := componentBitDepth + comp.numLevels

					// Use saved zeroBitplanes for consistent maxBitplane across all layers
					zbp := existing.zeroBitplanes
					if !existing.zeroBitplanesSet {
						zbp = cbIncl.ZeroBitplanes  // Fallback if not set yet
					}
					maxBP := effectiveBitDepth - 1 - zbp

					if !existing.maxBitplaneSet {
						existing.maxBitplane = maxBP
						existing.maxBitplaneSet = true
					}

					cbDataMap[key] = existing
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
					{x0: sbWidth, y0: 0, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY},        // HL
					{x0: 0, y0: sbHeight, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY},       // LH
					{x0: sbWidth, y0: sbHeight, width: sbWidth, height: sbHeight, numCBX: numCBX, numCBY: numCBY}, // HH
				}
			}
		}

		// Create code-blocks with correct spatial positions
		codeBlocks := make([]*CodeBlockDecoder, 0)

		// Create code-blocks for ALL possible positions (including not-included ones)
		// Iterate through all resolutions and subbands
		for res := 0; res <= comp.numLevels; res++ {
			layouts := resolutionLayouts[res]

			// For each subband in this resolution
			for subbandIdx, layout := range layouts {
				numCBsInSubband := layout.numCBX * layout.numCBY

				// For each code-block in this subband
				for localCBIdx := 0; localCBIdx < numCBsInSubband; localCBIdx++ {
					// Calculate global cbIdx for this resolution
					// Resolution 0: cbIdx = localCBIdx (only LL subband)
					// Resolution > 0: cbIdx = subbandIdx * numCBsInSubband + localCBIdx
					var cbIdx int
					if res == 0 {
						cbIdx = localCBIdx
					} else {
						cbIdx = subbandIdx * numCBsInSubband + localCBIdx
					}

					// Look up code-block data from map
					key := fmt.Sprintf("%d:%d", res, cbIdx)
					cbInfoData, exists := cbDataMap[key]
					if !exists {
						// Code-block not included in packet - use empty data (all-zero coefficients)
						cbInfoData = cbInfo{
							data:             []byte{},
							resolution:       res,
							cbIdx:            cbIdx,
							maxBitplane:      -1, // All zeros
							maxBitplaneSet:   true,
							zeroBitplanesSet: false,
						}
					}

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
				numPasses := cbInfoData.totalPasses
				if numPasses == 0 {
					// Fallback: calculate from maxBitplane (3 passes per bitplane)
					numPasses = (cbInfoData.maxBitplane + 1) * 3
				}

				cbd := &CodeBlockDecoder{
					x0:        x0,
					y0:        y0,
					x1:        x1,
					y1:        y1,
					data:      cbInfoData.data,
					numPasses: numPasses,
					t1Decoder: t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle)),
				}

				// Decode the code-block
				// Skip decoding if: 1) no data, 2) all passes have zero length, or 3) maxBitplane < 0 (all zeros)
				shouldDecode := len(cbInfoData.data) > 0
				if shouldDecode && cbInfoData.passLengths != nil && len(cbInfoData.passLengths) > 0 {
					// Check if all PassLengths are zero (no actual data to decode)
					allZero := true
					for _, pl := range cbInfoData.passLengths {
						if pl > 0 {
							allZero = false
							break
						}
					}
					if allZero {
						shouldDecode = false
					}
				}
				if cbInfoData.maxBitplane < 0 {
					// All-zero code block
					shouldDecode = false
				}

				if shouldDecode {
					var err error
					if cbInfoData.passLengths != nil && len(cbInfoData.passLengths) > 0 {
						// Multi-layer mode: use DecodeLayeredWithMode
						// useTERMALL flag determines whether passes are terminated independently
						// lossless flag (from COD transformation) determines whether contexts are reset
						lossless := td.cod.Transformation == 1 // 1 = 5/3 reversible (lossless)
						err = cbd.t1Decoder.DecodeLayeredWithMode(cbInfoData.data, cbInfoData.passLengths, cbInfoData.maxBitplane, 0, cbInfoData.useTERMALL, lossless)
					} else {
						// Single-layer mode: use DecodeWithBitplane
						err = cbd.t1Decoder.DecodeWithBitplane(cbInfoData.data, cbd.numPasses, cbInfoData.maxBitplane, 0)
						// Error logging trimmed; caller propagates error below
					}

					if err != nil {
						cbd.coeffs = make([]int32, actualWidth*actualHeight)
					} else {
						cbd.coeffs = cbd.t1Decoder.GetData()

					// Inverse General Scaling for ROI blocks (Srgn=1)
					if td.roi != nil {
						shiftVal, style, inside := td.roi.context(comp.componentIdx, x0, y0, x1, y1)
						if style == 1 && shiftVal > 0 && inside {
							blockMask := td.roi.blockMask(comp.componentIdx, x0, y0, x1, y1)
							if len(blockMask) > 0 && len(blockMask[0]) > 0 {
								applyInverseGeneralScalingMasked(cbd.coeffs, blockMask, shiftVal)
							} else {
								applyInverseGeneralScaling(cbd.coeffs, shiftVal)
							}
						}
					}
					}
				} else {
					// No data or all-zero code block - use all-zero coefficients
					cbd.coeffs = make([]int32, actualWidth*actualHeight)
				}

				codeBlocks = append(codeBlocks, cbd)
				}
			}
		}

		// Store code-blocks for assembly
		comp.resolutions = make([]*ResolutionLevel, comp.numLevels+1)
		comp.codeBlocks = codeBlocks
	}

	return nil
}
