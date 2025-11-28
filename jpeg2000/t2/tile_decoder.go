package t2

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

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
	data      []byte // Compressed data
	numPasses int
	t1Decoder *t1.T1Decoder
	coeffs    []int32 // Decoded coefficients
}

// NewTileDecoder creates a new tile decoder
func NewTileDecoder(
	tile *codestream.Tile,
	siz *codestream.SIZSegment,
	cod *codestream.CODSegment,
	qcd *codestream.QCDSegment,
) *TileDecoder {
	td := &TileDecoder{
		tile: tile,
		siz:  siz,
		cod:  cod,
		qcd:  qcd,
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

	packets, err := packetDec.DecodePackets()
	if err != nil {
		return nil, fmt.Errorf("failed to decode packets: %w", err)
	}

	// Decode code-blocks for all components from the parsed packets
	if err := td.decodeAllCodeBlocksFixed(packets); err != nil {
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

		// Level shift
		td.levelShift(comp)

		td.decodedData[i] = comp.samples
	}

	return td.decodedData, nil
}

// decodeAllCodeBlocks decodes code-blocks for all components from packets
func (td *TileDecoder) decodeAllCodeBlocks(packets []Packet) error {
	cbWidth, cbHeight := td.cod.CodeBlockSize()

	// Process each component
	for _, comp := range td.components {
		// Calculate number of code-blocks for this component
		numCBX := (comp.width + cbWidth - 1) / cbWidth
		numCBY := (comp.height + cbHeight - 1) / cbHeight

		// Accumulate code-block data from packets for this component
		cbDataMap := make(map[int][]byte) // map[cbIndex]data
		cbPassesMap := make(map[int]int)  // map[cbIndex]total passes
		maxBitplaneMap := make(map[int]int)
		cbPassLengthsMap := make(map[int][]int) // map[cbIndex]passLengths (for TERMALL)
		cbUseTERMALLMap := make(map[int]bool)   // map[cbIndex]useTERMALL flag

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
						if existing, ok := cbDataMap[cbIdx]; ok {
							// Append to existing data
							cbDataMap[cbIdx] = append(existing, cbData...)
						} else {
							cbDataMap[cbIdx] = cbData
						}
						dataOffset += cbIncl.DataLength

						// Accumulate passes from packet header
						// cbIncl.NumPasses is the NEW passes in this packet (for this layer)
						cbPassesMap[cbIdx] += cbIncl.NumPasses

						// Store PassLengths and UseTERMALL (from first packet that provides it)
						if cbIncl.PassLengths != nil && len(cbIncl.PassLengths) > 0 {
							if _, ok := cbPassLengthsMap[cbIdx]; !ok {
								cbPassLengthsMap[cbIdx] = cbIncl.PassLengths
							}
						}
						if cbIncl.UseTERMALL {
							cbUseTERMALLMap[cbIdx] = true
						}

						// Calculate max bitplane from zero bitplanes
						// maxBitplane = bitDepth - 1 - zeroBitplanes
						componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
						maxBP := componentBitDepth - 1 - cbIncl.ZeroBitplanes
						if maxBP > maxBitplaneMap[cbIdx] {
							maxBitplaneMap[cbIdx] = maxBP
						}
					}
				}
			}
		}

		// Create and decode code-blocks for this component
		codeBlocks := make([]*CodeBlockDecoder, 0, numCBX*numCBY)

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

				// Create code-block decoder
				// Use accumulated passes from packet headers if available
				numPasses, hasPasses := cbPassesMap[cbIdx]
				if !hasPasses || numPasses == 0 {
					// Fallback: calculate from bitplane (for backward compatibility)
					numPasses = (maxBitplane + 1) * 3
				}

				cbd := &CodeBlockDecoder{
					x0:        x0,
					y0:        y0,
					x1:        x1,
					y1:        y1,
					data:      cbData,
					numPasses: numPasses,
					t1Decoder: t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle)),
				}

				// Decode the code-block
				if hasData && len(cbData) > 0 {
					// Check if TERMALL mode is enabled
					useTERMALL := cbUseTERMALLMap[cbIdx]
					passLengths, hasPassLengths := cbPassLengthsMap[cbIdx]

					var err error
					if useTERMALL && hasPassLengths && len(passLengths) > 0 {
						// TERMALL mode: use DecodeLayered with per-pass lengths
						err = cbd.t1Decoder.DecodeLayered(cbData, passLengths, maxBitplane, 0)
					} else {
						// Normal mode: use DecodeWithBitplane
						err = cbd.t1Decoder.DecodeWithBitplane(cbData, cbd.numPasses, maxBitplane, 0)
					}

					if err != nil {
						// If decode fails, use zeros
						cbd.coeffs = make([]int32, actualWidth*actualHeight)
					} else {
						// Get decoded coefficients
						cbd.coeffs = cbd.t1Decoder.GetData()
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
	}

	return nil
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
	td.levelShift(comp)

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

					// Calculate max bitplane from zero bitplanes
					// maxBitplane = bitDepth - 1 - zeroBitplanes
					componentBitDepth := int(td.siz.Components[comp.componentIdx].Ssiz&0x7F) + 1
					maxBP := componentBitDepth - 1 - cbIncl.ZeroBitplanes
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

			// Create code-block decoder
			cbd := &CodeBlockDecoder{
				x0:        x0,
				y0:        y0,
				x1:        x1,
				y1:        y1,
				data:      cbData,
				numPasses: (maxBitplane + 1) * 3,
				t1Decoder: t1.NewT1Decoder(actualWidth, actualHeight, int(td.cod.CodeBlockStyle)),
			}

			// Decode the code-block
			if hasData && len(cbData) > 0 {
				// Use DecodeWithBitplane for accurate reconstruction
				err := cbd.t1Decoder.DecodeWithBitplane(cbData, cbd.numPasses, maxBitplane, 0)
				if err != nil {
					// If decode fails, use zeros
					cbd.coeffs = make([]int32, actualWidth*actualHeight)
				} else {
					// Get decoded coefficients
					cbd.coeffs = cbd.t1Decoder.GetData()
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
	cbCount := 0
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
		cbCount++
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
