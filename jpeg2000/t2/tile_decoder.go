package t2

import (
	"fmt"
	"math"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// BlockDecoder is an interface for T1 block decoders (EBCOT or HTJ2K)
type BlockDecoder interface {
	// DecodeWithBitplane decodes a code-block with known max bitplane
	DecodeWithBitplane(data []byte, numPasses int, maxBitplane int, roishift int) error
	// DecodeLayered decodes a code-block with TERMALL mode
	DecodeLayered(data []byte, passLengths []int, maxBitplane int, roishift int) error
	// GetData returns the decoded coefficients
	GetData() []int32
}

// BlockDecoderFactory creates block decoders for a specific code-block size
type BlockDecoderFactory func(width, height int, cblkstyle int) BlockDecoder

// TileDecoder decodes a single JPEG 2000 tile
type TileDecoder struct {
	// Tile information
	tile *codestream.Tile
	siz  *codestream.SIZSegment
	cod  *codestream.CODSegment
	qcd  *codestream.QCDSegment

	// Component decoders
	components []*ComponentDecoder

	// Decoded code-blocks (shared across components)
	codeBlocks []*CodeBlockDecoder

	// Output
	decodedData [][]int32 // [component][pixel]

	// ROI
	roi *ROIInfo

	// HTJ2K support
	isHTJ2K             bool
	blockDecoderFactory BlockDecoderFactory
}

// ComponentDecoder decodes a single component within a tile
type ComponentDecoder struct {
	componentIdx int
	width        int
	height       int
	numLevels    int // Number of DWT levels

	// Resolution levels
	resolutions []*ResolutionLevel

	// Code-blocks for this component
	codeBlocks []*CodeBlockDecoder

	// Decoded coefficients (after EBCOT, before IDWT)
	coefficients []int32

	// Final decoded samples (after IDWT)
	samples []int32
}

// ResolutionLevel represents one resolution level of a component
// Fields reserved for future complete implementation
type ResolutionLevel struct {
	_ int               // level (reserved)
	_ int               // width (reserved)
	_ int               // height (reserved)
	_ []*SubbandDecoder // subbands (reserved)
}

// SubbandDecoder decodes a single subband
// Fields reserved for future complete implementation
type SubbandDecoder struct {
	_ codestream.SubbandType // subbandType (reserved)
	_ int                    // width (reserved)
	_ int                    // height (reserved)
	_ []*CodeBlockDecoder    // codeBlocks (reserved)
	_ []int32                // coeffs (reserved)
}

// CodeBlockDecoder decodes a single code-block
type CodeBlockDecoder struct {
	x0, y0    int
	x1, y1    int
	band      int
	data      []byte // Compressed data
	numPasses int
	t1Decoder BlockDecoder // Can be EBCOT T1 or HTJ2K decoder
	coeffs    []int32      // Decoded coefficients
}

type ROIInfo struct {
	Rects            []ROIRect   // legacy/global
	RectsByComponent [][]ROIRect // per-component rectangles
	Shifts           []int       // per component shift (MaxShift or GeneralScaling)
	Styles           []byte      // per component Srgn style (0=MaxShift, 1=GeneralScaling)
	Masks            []*ROIMask  // per-component mask
}

// ROIRect is an axis-aligned rectangle.
type ROIRect struct {
	X0, Y0 int
	X1, Y1 int
}

func (r ROIRect) Intersects(x0, y0, x1, y1 int) bool {
	return r.X0 < x1 && x0 < r.X1 && r.Y0 < y1 && y0 < r.Y1
}

// ROIMask wraps a boolean mask.
type ROIMask struct {
	Width  int
	Height int
	Data   []bool
}

func (r *ROIInfo) intersects(compIdx, x0, y0, x1, y1 int) bool {
	if r == nil {
		return false
	}

	// Prefer mask if available
	if compIdx >= 0 && compIdx < len(r.Masks) && r.Masks[compIdx] != nil {
		m := r.Masks[compIdx]
		// sample mask region; if any pixel inside ROI
		if m.Width > 0 && m.Height > 0 && len(m.Data) == m.Width*m.Height {
			clamp := func(v, max int) int {
				if v < 0 {
					return 0
				}
				if v > max {
					return max
				}
				return v
			}
			x0c := clamp(x0, m.Width)
			y0c := clamp(y0, m.Height)
			x1c := clamp(x1, m.Width)
			y1c := clamp(y1, m.Height)
			for yy := y0c; yy < y1c; yy++ {
				row := yy * m.Width
				for xx := x0c; xx < x1c; xx++ {
					if m.Data[row+xx] {
						return true
					}
				}
			}
		}
	}

	if compIdx >= 0 && compIdx < len(r.RectsByComponent) && len(r.RectsByComponent[compIdx]) > 0 {
		for _, rect := range r.RectsByComponent[compIdx] {
			if rect.Intersects(x0, y0, x1, y1) {
				return true
			}
		}
		return false
	}

	for _, rect := range r.Rects {
		if rect.Intersects(x0, y0, x1, y1) {
			return true
		}
	}
	return false
}

func (r *ROIInfo) context(compIdx, x0, y0, x1, y1 int) (int, byte, bool) {
	if r == nil || compIdx < 0 || compIdx >= len(r.Shifts) {
		return 0, 0, false
	}
	shift := r.Shifts[compIdx]
	style := byte(0)
	if compIdx < len(r.Styles) {
		style = r.Styles[compIdx]
	}
	if shift <= 0 {
		return 0, style, false
	}
	inside := false
	if compIdx >= 0 && compIdx < len(r.Masks) && r.Masks[compIdx] != nil {
		blockMask := r.blockMask(compIdx, x0, y0, x1, y1)
		if len(blockMask) > 0 && len(blockMask[0]) > 0 {
			inside = maskAnyTrue(blockMask)
		}
	}
	if !inside {
		inside = r.intersects(compIdx, x0, y0, x1, y1)
	}
	return shift, style, inside
}

// blockMask extracts a boolean mask for the given block region.
func (r *ROIInfo) blockMask(compIdx, x0, y0, x1, y1 int) [][]bool {
	if r == nil || compIdx < 0 || compIdx >= len(r.Masks) || r.Masks[compIdx] == nil {
		return nil
	}
	m := r.Masks[compIdx]
	if m.Width <= 0 || m.Height <= 0 || len(m.Data) != m.Width*m.Height {
		return nil
	}
	w := x1 - x0
	h := y1 - y0
	if w <= 0 || h <= 0 {
		return nil
	}
	out := make([][]bool, h)
	for j := 0; j < h; j++ {
		out[j] = make([]bool, w)
		srcY := y0 + j
		if srcY < 0 || srcY >= m.Height {
			continue
		}
		rowOffset := srcY * m.Width
		for i := 0; i < w; i++ {
			srcX := x0 + i
			if srcX < 0 || srcX >= m.Width {
				continue
			}
			out[j][i] = m.Data[rowOffset+srcX]
		}
	}
	return out
}

// NewTileDecoder creates a new tile decoder
func NewTileDecoder(
	tile *codestream.Tile,
	siz *codestream.SIZSegment,
	cod *codestream.CODSegment,
	qcd *codestream.QCDSegment,
	roi *ROIInfo,
	isHTJ2K bool,
	blockDecoderFactory BlockDecoderFactory,
) *TileDecoder {
	td := &TileDecoder{
		tile:                tile,
		siz:                 siz,
		cod:                 cod,
		qcd:                 qcd,
		roi:                 roi,
		isHTJ2K:             isHTJ2K,
		blockDecoderFactory: blockDecoderFactory,
	}

	return td
}

// Decode decodes the tile and returns the pixel data for each component
func (td *TileDecoder) Decode() ([][]int32, error) {
	// Initialize component decoders
	numComponents := int(td.siz.Csiz)
	td.components = make([]*ComponentDecoder, numComponents)
	td.decodedData = make([][]int32, numComponents)

	for i := 0; i < numComponents; i++ {
		comp := &ComponentDecoder{
			componentIdx: i,
			width:        int(td.siz.Xsiz), // Simplified - should calculate per-component
			height:       int(td.siz.Ysiz),
			numLevels:    int(td.cod.NumberOfDecompositionLevels),
		}

		td.components[i] = comp
	}

	// Parse packets ONCE for all components
	packetDec := NewPacketDecoder(
		td.tile.Data,
		int(td.siz.Csiz),
		int(td.cod.NumberOfLayers),
		int(td.cod.NumberOfDecompositionLevels)+1, // numResolutions = numLevels + 1
		ProgressionOrder(td.cod.ProgressionOrder),
		td.cod.CodeBlockStyle,
	)

	// Set image dimensions and code-block size
	cbWidth, cbHeight := td.cod.CodeBlockSize()
	packetDec.SetImageDimensions(int(td.siz.Xsiz), int(td.siz.Ysiz), cbWidth, cbHeight)

	// Set precinct size if defined in COD segment
	if len(td.cod.PrecinctSizes) > 0 {
		// Use precinct size from first resolution level (simplified)
		// In full implementation, should handle per-resolution precinct sizes
		ppx := td.cod.PrecinctSizes[0].PPx
		ppy := td.cod.PrecinctSizes[0].PPy
		precinctWidth := 1 << ppx
		precinctHeight := 1 << ppy
		packetDec.SetPrecinctSize(precinctWidth, precinctHeight)
	}

	packets, err := packetDec.DecodePackets()
	if err != nil {
		return nil, fmt.Errorf("failed to decode packets: %w", err)
	}

	if err := td.decodeAllCodeBlocks(packets); err != nil {
		return nil, fmt.Errorf("failed to decode code-blocks: %w", err)
	}

	// Process each component
	for i := 0; i < numComponents; i++ {
		comp := td.components[i]

		// Assemble subbands
		if err := td.assembleSubbands(comp); err != nil {
			return nil, fmt.Errorf("failed to assemble subbands for component %d: %w", i, err)
		}

		// Apply IDWT
		if err := td.applyIDWT(comp); err != nil {
			return nil, fmt.Errorf("IDWT failed for component %d: %w", i, err)
		}

		// Level shift - DISABLED: DC shift should be applied at codec level (decoder.go), not here
		// to match OpenJPEG pipeline: T1^-1 -> DWT^-1 -> MCT^-1 -> DC shift^-1
		// td.levelShift(comp)

		td.decodedData[i] = comp.samples
	}

	return td.decodedData, nil
}

// decodeAllCodeBlocks decodes code-blocks for all components from packets
func (td *TileDecoder) decodeAllCodeBlocks(packets []Packet) error {
	cbWidth, cbHeight := td.cod.CodeBlockSize()

	// Process each component
	for _, comp := range td.components {
		// Map packet-local code-block indices to global ordering per precinct/resolution
		precinctOrder := td.buildPrecinctOrder(comp, cbWidth, cbHeight)

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

			resOrder := precinctOrder[packet.ResolutionLevel]
			if resOrder == nil {
				continue
			}
			cbOrder := resOrder[packet.PrecinctIndex]
			if cbOrder == nil {
				continue
			}

			// Extract code-block contributions from this packet
			dataOffset := 0
			for cbIdx, cbIncl := range packet.CodeBlockIncls {
				if cbIncl.Included {
					if cbIdx >= len(cbOrder) {
						dataOffset += cbIncl.DataLength
						continue
					}
					actualCBIdx := cbOrder[cbIdx]
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
					key := fmt.Sprintf("%d:%d", packet.ResolutionLevel, actualCBIdx)

					existing, ok := cbDataMap[key]

					if ok {
						// Append to existing data
						existing.data = append(existing.data, cbData...)
					} else {
						existing.data = cbData
						existing.resolution = packet.ResolutionLevel
						existing.cbIdx = cbIdx
					}

					dataOffset += cbIncl.DataLength

					// Accumulate PassLengths from each layer (convert to cumulative)
					if cbIncl.PassLengths != nil && len(cbIncl.PassLengths) > 0 {
						if existing.passLengths == nil {
							existing.passLengths = make([]int, len(cbIncl.PassLengths))
							total := 0
							for i, passLen := range cbIncl.PassLengths {
								total += passLen
								existing.passLengths[i] = total
							}
						} else {
							total := existing.passLengths[len(existing.passLengths)-1]
							for _, passLen := range cbIncl.PassLengths {
								total += passLen
								existing.passLengths = append(existing.passLengths, total)
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
				llWidth := subbandDim(comp.width, comp.numLevels, res)
				llHeight := subbandDim(comp.height, comp.numLevels, res)
				numCBX := (llWidth + cbWidth - 1) / cbWidth
				numCBY := (llHeight + cbHeight - 1) / cbHeight

				resolutionLayouts[res] = []subbandLayout{
					{x0: 0, y0: 0, width: llWidth, height: llHeight, numCBX: numCBX, numCBY: numCBY},
				}
			} else {
				// Resolution r: HL, LH, HH subbands
				sbWidth := subbandDim(comp.width, comp.numLevels, res)
				sbHeight := subbandDim(comp.height, comp.numLevels, res)
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
		globalCBIdx := 0
		for res := 0; res <= comp.numLevels; res++ {
			layouts := resolutionLayouts[res]

			// For each subband in this resolution
			for _, layout := range layouts {
				band := 0
				if res > 0 {
					if layout.x0 > 0 && layout.y0 == 0 {
						band = 1 // HL
					} else if layout.x0 == 0 && layout.y0 > 0 {
						band = 2 // LH
					} else if layout.x0 > 0 && layout.y0 > 0 {
						band = 3 // HH
					}
				}
				// For each code-block in this subband
				for cby := 0; cby < layout.numCBY; cby++ {
					for cbx := 0; cbx < layout.numCBX; cbx++ {
						cbIdx := globalCBIdx
						globalCBIdx++

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
						if !cbInfoData.maxBitplaneSet {
							zbp := cbInfoData.zeroBitplanes
							if !cbInfoData.zeroBitplanesSet {
								zbp = 0
							}
							bandNumbps, ok := bandNumbpsFromQCD(td.qcd, comp.numLevels, res, band)
							var maxBP int
							if ok && bandNumbps > 0 {
								const T1_NMSEDEC_FRACBITS = 6
								cblkNumbps := bandNumbps + 1 - zbp
								if cblkNumbps <= 0 {
									maxBP = -1
								} else {
									maxBP = (cblkNumbps - 1) + T1_NMSEDEC_FRACBITS
								}
							} else {
								const T1_NMSEDEC_FRACBITS = 6
								componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
								effectiveBitDepth := componentBitDepth + comp.numLevels + T1_NMSEDEC_FRACBITS
								maxBP = effectiveBitDepth - 1 - zbp
							}
							if maxBP < -1 {
								maxBP = -1
							}
							cbInfoData.maxBitplane = maxBP
							cbInfoData.maxBitplaneSet = true
						}

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
							// Fallback: calculate from maxBitplane (3 passes per bitplane, top bitplane cleanup only)
							const T1_NMSEDEC_FRACBITS = 6
							cblkNumbps := (cbInfoData.maxBitplane + 1) - T1_NMSEDEC_FRACBITS
							if cblkNumbps > 0 {
								numPasses = (cblkNumbps * 3) - 2
							} else if cbInfoData.maxBitplane >= 0 {
								numPasses = 1
							}
						}

						cbd := &CodeBlockDecoder{
							x0:        x0,
							y0:        y0,
							x1:        x1,
							y1:        y1,
							band:      band,
							data:      cbInfoData.data,
							numPasses: numPasses,
							t1Decoder: func() BlockDecoder {
								if td.isHTJ2K && td.blockDecoderFactory != nil {
									return td.blockDecoderFactory(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
								}
								return t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
							}(),
						}
						if orientSetter, ok := cbd.t1Decoder.(interface{ SetOrientation(int) }); ok {
							orientSetter.SetOrientation(band)
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
								// Multi-layer mode
								// Check if this is EBCOT T1 (has DecodeLayeredWithMode) or HTJ2K (use DecodeLayered)
								if t1Dec, ok := cbd.t1Decoder.(interface {
									DecodeLayeredWithMode(data []byte, passLengths []int, maxBitplane int, roishift int, useTERMALL bool, resetContexts bool) error
								}); ok {
									// EBCOT T1 decoder with TERMALL mode support
									resetContexts := (td.cod.CodeBlockStyle & 0x02) != 0
									err = t1Dec.DecodeLayeredWithMode(cbInfoData.data, cbInfoData.passLengths, cbInfoData.maxBitplane, 0, cbInfoData.useTERMALL, resetContexts)
								} else {
									// HTJ2K or other decoder - use standard DecodeLayered
									err = cbd.t1Decoder.DecodeLayered(cbInfoData.data, cbInfoData.passLengths, cbInfoData.maxBitplane, 0)
								}
							} else {
								// Single-layer mode: use DecodeWithBitplane
								err = cbd.t1Decoder.DecodeWithBitplane(cbInfoData.data, cbd.numPasses, cbInfoData.maxBitplane, 0)
								// Error logging trimmed; caller propagates error below
							}

							if err != nil {
								cbd.coeffs = make([]int32, actualWidth*actualHeight)
							} else {
								cbd.coeffs = cbd.t1Decoder.GetData()
								var shiftVal int
								var style byte
								var inside bool
								if td.roi != nil {
									shiftVal, style, inside = td.roi.context(comp.componentIdx, x0, y0, x1, y1)
									if style == 0 && shiftVal > 0 {
										applyInverseMaxShift(cbd.coeffs, shiftVal)
									}
								}

								// Apply inverse T1_NMSEDEC_FRACBITS scaling (right shift by 6)
								// Encoder applies <<6 before T1 encoding, decoder must reverse it
								const T1_NMSEDEC_FRACBITS = 6
								for i := range cbd.coeffs {
									cbd.coeffs[i] >>= T1_NMSEDEC_FRACBITS
								}

								// Inverse General Scaling for ROI blocks (Srgn=1)
								if td.roi != nil && style == 1 && shiftVal > 0 && inside {
									blockMask := td.roi.blockMask(comp.componentIdx, x0, y0, x1, y1)
									if len(blockMask) > 0 && len(blockMask[0]) > 0 {
										applyInverseGeneralScalingMasked(cbd.coeffs, blockMask, shiftVal)
									} else {
										applyInverseGeneralScaling(cbd.coeffs, shiftVal)
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
		}

		// Store code-blocks for assembly
		comp.resolutions = make([]*ResolutionLevel, comp.numLevels+1)
		comp.codeBlocks = codeBlocks
	}

	return nil
}

// buildPrecinctOrder returns the mapping of precinct index -> ordered list of global code-block indices for each resolution.
func (td *TileDecoder) buildPrecinctOrder(comp *ComponentDecoder, cbWidth, cbHeight int) map[int]map[int][]int {
	order := make(map[int]map[int][]int)
	pw, ph := td.precinctSize()
	globalCBIdx := 0

	for res := 0; res <= comp.numLevels; res++ {
		order[res] = make(map[int][]int)
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
			llWidth := subbandDim(comp.width, comp.numLevels, res)
			llHeight := subbandDim(comp.height, comp.numLevels, res)
			numCBX := (llWidth + cbWidth - 1) / cbWidth
			numCBY := (llHeight + cbHeight - 1) / cbHeight

			resWidth := resolutionDim(comp.width, comp.numLevels, res)
			numPrecinctX := (resWidth + pw - 1) / pw
			if numPrecinctX < 1 {
				numPrecinctX = 1
			}

			for cby := 0; cby < numCBY; cby++ {
				for cbx := 0; cbx < numCBX; cbx++ {
					x0 := cbx * cbWidth
					y0 := cby * cbHeight
					px := x0 / pw
					py := y0 / ph
					pIdx := py*numPrecinctX + px
					localX := x0 - px*pw
					localY := y0 - py*ph
					cbxLocal := localX / cbWidth
					cbyLocal := localY / cbHeight
					addEntry(pIdx, 0, cbxLocal, cbyLocal, globalCBIdx)
					globalCBIdx++
				}
			}
		} else {
			sbWidth := subbandDim(comp.width, comp.numLevels, res)
			sbHeight := subbandDim(comp.height, comp.numLevels, res)
			numCBX := (sbWidth + cbWidth - 1) / cbWidth
			numCBY := (sbHeight + cbHeight - 1) / cbHeight

			resWidth := resolutionDim(comp.width, comp.numLevels, res)
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
						x0 := sb.x0 + cbx*cbWidth
						y0 := sb.y0 + cby*cbHeight
						resX, resY := td.toResolutionCoordinates(x0, y0, res, sb.band, sbWidth, sbHeight)
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
						cbxLocal := localX / cbWidth
						cbyLocal := localY / cbHeight
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
				for _, entry := range entries {
					order[res][pIdx] = append(order[res][pIdx], entry.global)
				}
			}
		}
	}

	return order
}

// precinctSize returns the precinct size in pixels (default 2^15 when not set).
func (td *TileDecoder) precinctSize() (int, int) {
	if len(td.cod.PrecinctSizes) > 0 {
		ppx := td.cod.PrecinctSizes[0].PPx
		ppy := td.cod.PrecinctSizes[0].PPy
		return 1 << ppx, 1 << ppy
	}
	return 1 << 15, 1 << 15
}

// toResolutionCoordinates mirrors encoder mapping (per subband).
func (td *TileDecoder) toResolutionCoordinates(globalX, globalY, resolution, band, sbWidth, sbHeight int) (int, int) {
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

// decodeComponent decodes a single component (deprecated - use decodeAllCodeBlocks)
func (td *TileDecoder) decodeComponent(comp *ComponentDecoder) error {
	// Step 1: Parse packets and extract code-block data
	// For MVP, we'll skip detailed packet parsing and assume data is available

	// Step 2: Decode code-blocks using EBCOT Tier-1
	if err := td.decodeCodeBlocks(comp); err != nil {
		return fmt.Errorf("code-block decoding failed: %w", err)
	}

	// Step 3: Assemble code-block coefficients into subband coefficients
	if err := td.assembleSubbands(comp); err != nil {
		return fmt.Errorf("subband assembly failed: %w", err)
	}

	// Step 4: Apply inverse wavelet transform
	if err := td.applyIDWT(comp); err != nil {
		return fmt.Errorf("IDWT failed: %w", err)
	}

	// Step 5: Level shift and convert to samples
	// DISABLED: DC shift should be applied at codec level (decoder.go), not here
	// td.levelShift(comp)

	return nil
}

// decodeCodeBlocks decodes all code-blocks in a component using EBCOT
func (td *TileDecoder) decodeCodeBlocks(comp *ComponentDecoder) error {
	// Parse packets from tile data
	packetDec := NewPacketDecoder(
		td.tile.Data,
		int(td.siz.Csiz),
		int(td.cod.NumberOfLayers),
		comp.numLevels+1, // numResolutions = numLevels + 1
		ProgressionOrder(td.cod.ProgressionOrder),
		td.cod.CodeBlockStyle,
	)

	packets, err := packetDec.DecodePackets()
	if err != nil {
		return fmt.Errorf("failed to decode packets: %w", err)
	}

	// Get code-block size from COD
	cbWidth, cbHeight := td.cod.CodeBlockSize()

	// Calculate number of code-blocks
	numCBX := (comp.width + cbWidth - 1) / cbWidth
	numCBY := (comp.height + cbHeight - 1) / cbHeight

	// Initialize code-block storage
	codeBlocks := make([]*CodeBlockDecoder, 0, numCBX*numCBY)

	// Extract code-block data from packets for this component
	cbDataMap := make(map[int][]byte) // map[cbIndex]data
	maxBitplaneMap := make(map[int]int)

	for _, packet := range packets {
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
					if existing, ok := cbDataMap[cbIdx]; ok {
						// Append to existing data
						cbDataMap[cbIdx] = append(existing, cbData...)
					} else {
						cbDataMap[cbIdx] = cbData
					}
					dataOffset += cbIncl.DataLength

					// Calculate max bitplane from zero bitplanes and QCD band range
					bandNumbps, ok := bandNumbpsFromQCD(td.qcd, int(td.cod.NumberOfDecompositionLevels), 0, 0)
					maxBP := 0
					if ok && bandNumbps > 0 {
						const T1_NMSEDEC_FRACBITS = 6
						cblkNumbps := bandNumbps + 1 - cbIncl.ZeroBitplanes
						if cblkNumbps <= 0 {
							maxBP = -1
						} else {
							maxBP = (cblkNumbps - 1) + T1_NMSEDEC_FRACBITS
						}
					} else {
						const T1_NMSEDEC_FRACBITS = 6
						baseBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
						numLevels := int(td.cod.NumberOfDecompositionLevels)
						effectiveBitDepth := baseBitDepth + numLevels + T1_NMSEDEC_FRACBITS
						maxBP = effectiveBitDepth - 1 - cbIncl.ZeroBitplanes
					}
					if maxBP < -1 {
						maxBP = -1
					}
					if maxBP > maxBitplaneMap[cbIdx] {
						maxBitplaneMap[cbIdx] = maxBP
					}
				}
			}
		}
	}

	// Create and decode code-blocks
	for cby := 0; cby < numCBY; cby++ {
		for cbx := 0; cbx < numCBX; cbx++ {
			// Calculate code-block bounds
			x0 := cbx * cbWidth
			y0 := cby * cbHeight
			x1 := x0 + cbWidth
			y1 := y0 + cbHeight

			// Clip to image bounds
			if x1 > comp.width {
				x1 = comp.width
			}
			if y1 > comp.height {
				y1 = comp.height
			}

			actualWidth := x1 - x0
			actualHeight := y1 - y0
			cbIdx := cby*numCBX + cbx

			// Get code-block data from packets
			cbData, hasData := cbDataMap[cbIdx]
			maxBitplane := maxBitplaneMap[cbIdx]
			numPasses := 1
			if maxBitplane >= 0 {
				const T1_NMSEDEC_FRACBITS = 6
				cblkNumbps := (maxBitplane + 1) - T1_NMSEDEC_FRACBITS
				if cblkNumbps > 0 {
					numPasses = (cblkNumbps * 3) - 2
				}
			}

			// Create code-block decoder - use HTJ2K if enabled, otherwise EBCOT T1
			var blockDecoder BlockDecoder
			if td.isHTJ2K && td.blockDecoderFactory != nil {
				blockDecoder = td.blockDecoderFactory(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
			} else {
				blockDecoder = t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle))
			}

			cbd := &CodeBlockDecoder{
				x0:        x0,
				y0:        y0,
				x1:        x1,
				y1:        y1,
				band:      0,
				data:      cbData,
				numPasses: numPasses,
				t1Decoder: blockDecoder,
			}
			if orientSetter, ok := cbd.t1Decoder.(interface{ SetOrientation(int) }); ok {
				orientSetter.SetOrientation(0)
			}

			// Decode the code-block
			if hasData && len(cbData) > 0 {
				shiftVal := 0
				style := byte(0)
				inside := false
				if td.roi != nil {
					shiftVal, style, inside = td.roi.context(comp.componentIdx, x0, y0, x1, y1)
				}

				// Use DecodeWithBitplane for accurate reconstruction
				err := blockDecoder.DecodeWithBitplane(cbData, cbd.numPasses, maxBitplane, 0)
				if err != nil {
					// If decode fails, use zeros
					cbd.coeffs = make([]int32, actualWidth*actualHeight)
				} else {
					// Get decoded coefficients
					cbd.coeffs = blockDecoder.GetData()
					if style == 0 && shiftVal > 0 {
						applyInverseMaxShift(cbd.coeffs, shiftVal)
					}

					// Apply inverse T1_NMSEDEC_FRACBITS scaling (right shift by 6)
					// Encoder applies <<6 before T1 encoding, decoder must reverse it
					const T1_NMSEDEC_FRACBITS = 6
					for i := range cbd.coeffs {
						cbd.coeffs[i] >>= T1_NMSEDEC_FRACBITS
					}

					// Inverse General Scaling for ROI blocks (Srgn=1)
					if style == 1 && shiftVal > 0 && inside {
						blockMask := td.roi.blockMask(comp.componentIdx, x0, y0, x1, y1)
						if len(blockMask) > 0 && len(blockMask[0]) > 0 {
							applyInverseGeneralScalingMasked(cbd.coeffs, blockMask, shiftVal)
						} else {
							applyInverseGeneralScaling(cbd.coeffs, shiftVal)
						}
					}
				}
			} else {
				// No data - use zeros
				cbd.coeffs = make([]int32, actualWidth*actualHeight)
			}

			codeBlocks = append(codeBlocks, cbd)
		}
	}

	// Store code-blocks for assembly
	comp.resolutions = make([]*ResolutionLevel, comp.numLevels+1)
	// Store code-blocks in ComponentDecoder for assembly
	comp.codeBlocks = codeBlocks

	return nil
}

// assembleSubbands assembles code-block coefficients into subband arrays
func (td *TileDecoder) assembleSubbands(comp *ComponentDecoder) error {
	// Initialize the full coefficient array
	comp.coefficients = make([]int32, comp.width*comp.height)

	if len(comp.codeBlocks) == 0 {
		// No code-blocks decoded - use zeros
		return nil
	}

	// The code-blocks are organized in the same order as they were encoded:
	// - Resolution 0: LL subband (single subband at top-left)
	// - Resolution 1..N: HL, LH, HH subbands for each resolution level
	//
	// The wavelet coefficient array layout after DWT is:
	// For numLevels=1 (one decomposition):
	// +-------+-------+
	// |  LL   |  HL   |
	// +-------+-------+
	// |  LH   |  HH   |
	// +-------+-------+

	// Simply copy all code-blocks at their x0,y0 positions
	// The encoder has already set the correct x0,y0 for each code-block
	for _, cb := range comp.codeBlocks {
		x0 := cb.x0
		y0 := cb.y0
		x1 := cb.x1
		y1 := cb.y1
		actualWidth := x1 - x0
		actualHeight := y1 - y0

		// Copy decoded coefficients from code-block to full array
		for y := 0; y < actualHeight; y++ {
			for x := 0; x < actualWidth; x++ {
				srcIdx := y*actualWidth + x
				dstIdx := (y0+y)*comp.width + (x0 + x)

				if srcIdx < len(cb.coeffs) && dstIdx < len(comp.coefficients) {
					comp.coefficients[dstIdx] = cb.coeffs[srcIdx]
				}
			}
		}
	}

	return nil
}

// applyIDWT applies the inverse discrete wavelet transform
func (td *TileDecoder) applyIDWT(comp *ComponentDecoder) error {
	if comp.numLevels == 0 {
		// No wavelet transform - coefficients are samples
		comp.samples = comp.coefficients
		return nil
	}

	// Check transformation type
	if td.cod.Transformation == 1 {
		// 5/3 reversible wavelet (lossless)
		// Copy coefficients to samples buffer (wavelet transform is in-place)
		comp.samples = make([]int32, len(comp.coefficients))
		copy(comp.samples, comp.coefficients)

		// Apply inverse multilevel wavelet transform
		wavelet.InverseMultilevel(comp.samples, comp.width, comp.height, comp.numLevels)
	} else if td.cod.Transformation == 0 {
		// 9/7 irreversible wavelet (lossy)
		// First, apply dequantization if needed
		dequantized := comp.coefficients
		if td.qcd != nil && td.qcd.QuantizationType() == 2 {
			// Scalar expounded quantization - apply dequantization
			dequantized = td.applyDequantizationBySubband(comp.coefficients, comp.width, comp.height, comp.numLevels)
		}

		// Convert coefficients to float64 for 9/7 transform
		floatCoeffs := wavelet.ConvertInt32ToFloat64(dequantized)

		// Apply inverse multilevel 9/7 wavelet transform
		wavelet.InverseMultilevel97(floatCoeffs, comp.width, comp.height, comp.numLevels)

		// Convert back to int32 with rounding
		comp.samples = wavelet.ConvertFloat64ToInt32(floatCoeffs)
	} else {
		return fmt.Errorf("unsupported wavelet transformation type: %d", td.cod.Transformation)
	}

	return nil
}

// applyDequantizationBySubband applies dequantization to each subband separately
// coeffs: quantized wavelet coefficients in subband layout
// width, height: dimensions of the full image
// numLevels: number of wavelet decomposition levels
func (td *TileDecoder) applyDequantizationBySubband(coeffs []int32, width, height, numLevels int) []int32 {
	if td.qcd == nil || len(td.qcd.SPqcd) == 0 {
		// No dequantization
		return coeffs
	}

	// Get bit depth
	bitDepth := int(td.siz.Components[0].BitDepth())

	// Decode step sizes from QCD
	stepSizes := make([]float64, len(td.qcd.SPqcd)/2)
	for i := 0; i < len(stepSizes); i++ {
		// Each step size is 16-bit
		encoded := uint16(td.qcd.SPqcd[i*2])<<8 | uint16(td.qcd.SPqcd[i*2+1])

		// Decode: bits 11-15 = exponent, bits 0-10 = mantissa
		exponent := int((encoded >> 11) & 0x1F)
		mantissa := float64(encoded & 0x7FF)

		bias := bitDepth - 1
		// stepSize = 2^(exponent - bias) * (1 + mantissa / 2048)
		stepSizes[i] = math.Pow(2.0, float64(exponent-bias)) * (1.0 + mantissa/2048.0)
	}

	dequantized := make([]int32, len(coeffs))
	copy(dequantized, coeffs)

	// Calculate subband dimensions for each level
	subbandIdx := 0

	// Process from coarsest to finest level
	for level := numLevels; level >= 1; level-- {
		// Calculate dimensions at this level
		levelWidth := (width + (1 << level) - 1) >> level
		levelHeight := (height + (1 << level) - 1) >> level

		// At the coarsest level, we also have LL subband
		if level == numLevels {
			// LL subband
			if subbandIdx < len(stepSizes) {
				td.dequantizeSubband(dequantized, 0, 0, levelWidth, levelHeight, width, stepSizes[subbandIdx])
			}
			subbandIdx++
		}

		// HL subband
		if subbandIdx < len(stepSizes) {
			td.dequantizeSubband(dequantized, levelWidth, 0, levelWidth, levelHeight, width, stepSizes[subbandIdx])
		}
		subbandIdx++

		// LH subband
		if subbandIdx < len(stepSizes) {
			td.dequantizeSubband(dequantized, 0, levelHeight, levelWidth, levelHeight, width, stepSizes[subbandIdx])
		}
		subbandIdx++

		// HH subband
		if subbandIdx < len(stepSizes) {
			td.dequantizeSubband(dequantized, levelWidth, levelHeight, levelWidth, levelHeight, width, stepSizes[subbandIdx])
		}
		subbandIdx++
	}

	return dequantized
}

// dequantizeSubband dequantizes a single subband
// data: full coefficient array
// x0, y0: top-left corner of subband
// w, h: dimensions of subband
// stride: row stride (width of full image)
// stepSize: quantization step size
func (td *TileDecoder) dequantizeSubband(data []int32, x0, y0, w, h, stride int, stepSize float64) {
	if stepSize <= 0 {
		return
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y0+y)*stride + (x0 + x)
			if idx < len(data) {
				// Dequantize: coeff * stepSize
				data[idx] = int32(math.Round(float64(data[idx]) * stepSize))
			}
		}
	}
}

// levelShift applies DC level shift to convert coefficients to samples
func (td *TileDecoder) levelShift(comp *ComponentDecoder) {
	// Get bit depth
	bitDepth := td.siz.Components[comp.componentIdx].BitDepth()
	isSigned := td.siz.Components[comp.componentIdx].IsSigned()

	if isSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - add 2^(bitDepth-1)
	shift := int32(1 << (bitDepth - 1))
	for i := range comp.samples {
		comp.samples[i] += shift
	}
}

// GetComponentData returns the decoded data for a specific component
func (td *TileDecoder) GetComponentData(componentIdx int) ([]int32, error) {
	if componentIdx < 0 || componentIdx >= len(td.decodedData) {
		return nil, fmt.Errorf("invalid component index: %d", componentIdx)
	}

	return td.decodedData[componentIdx], nil
}

// GetAllComponentsData returns decoded data for all components
func (td *TileDecoder) GetAllComponentsData() [][]int32 {
	return td.decodedData
}

// applyInverseGeneralScaling divides coefficients by 2^shift in-place.
func applyInverseGeneralScaling(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	factor := int32(1 << shift)
	for i := range data {
		data[i] /= factor
	}
}

// applyInverseGeneralScalingMasked divides only masked coefficients by 2^shift.
func applyInverseGeneralScalingMasked(data []int32, mask [][]bool, shift int) {
	if shift <= 0 || len(mask) == 0 || len(mask[0]) == 0 {
		return
	}
	factor := int32(1 << shift)
	height := len(mask)
	width := len(mask[0])
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if mask[y][x] {
				idx := y*width + x
				if idx >= 0 && idx < len(data) {
					data[idx] /= factor
				}
			}
		}
	}
}

// applyInverseMaxShift reverses MaxShift ROI scaling using the standard threshold rule.
func applyInverseMaxShift(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	if shift >= 31 {
		for i := range data {
			data[i] = 0
		}
		return
	}
	thresh := int32(1 << shift)
	for i, val := range data {
		mag := val
		if mag < 0 {
			mag = -mag
		}
		if mag >= thresh {
			mag >>= shift
			if val < 0 {
				data[i] = -mag
			} else {
				data[i] = mag
			}
		}
	}
}

func maskAllTrue(mask [][]bool) bool {
	if len(mask) == 0 || len(mask[0]) == 0 {
		return false
	}
	for y := 0; y < len(mask); y++ {
		for x := 0; x < len(mask[y]); x++ {
			if !mask[y][x] {
				return false
			}
		}
	}
	return true
}

func maskAnyTrue(mask [][]bool) bool {
	for y := 0; y < len(mask); y++ {
		for x := 0; x < len(mask[y]); x++ {
			if mask[y][x] {
				return true
			}
		}
	}
	return false
}
