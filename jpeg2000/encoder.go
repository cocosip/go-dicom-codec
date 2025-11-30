package jpeg2000

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

// EncodeParams contains parameters for JPEG 2000 encoding
type EncodeParams struct {
	// Image parameters
	Width      int
	Height     int
	Components int
	BitDepth   int
	IsSigned   bool

	// Tile parameters
	TileWidth  int // 0 means single tile (entire image)
	TileHeight int // 0 means single tile (entire image)

	// Coding parameters
	NumLevels       int  // Number of wavelet decomposition levels (0-6)
	Lossless        bool // true for lossless (5/3 wavelet), false for lossy (9/7 wavelet)
	CodeBlockWidth  int  // Code-block width (power of 2, typically 64)
	CodeBlockHeight int  // Code-block height (power of 2, typically 64)

	// Lossy compression quality (1-100, only used when Lossless=false)
	// Higher values = better quality, lower compression
	// 100 = minimal quantization (near-lossless)
	// 50 = balanced quality/compression
	// 1 = maximum compression, lower quality
	Quality int // Default: 80

	// Progression order
	ProgressionOrder uint8 // 0=LRCP, 1=RLCP, 2=RPCL, 3=PCRL, 4=CPRL

	// Layer parameters
	NumLayers int // Number of quality layers (default 1)
}

// DefaultEncodeParams returns default encoding parameters for lossless encoding
func DefaultEncodeParams(width, height, components, bitDepth int, isSigned bool) *EncodeParams {
	return &EncodeParams{
		Width:            width,
		Height:           height,
		Components:       components,
		BitDepth:         bitDepth,
		IsSigned:         isSigned,
		TileWidth:        0, // Single tile
		TileHeight:       0, // Single tile
		NumLevels:        5, // 5 DWT levels
		Lossless:         true,
		Quality:          80, // Default quality for lossy mode
		CodeBlockWidth:   64,
		CodeBlockHeight:  64,
		ProgressionOrder: 0, // LRCP
		NumLayers:        1,
	}
}

// Encoder implements JPEG 2000 encoding
type Encoder struct {
	params *EncodeParams
	data   [][]int32 // [component][pixel]
}

// NewEncoder creates a new JPEG 2000 encoder
func NewEncoder(params *EncodeParams) *Encoder {
	return &Encoder{
		params: params,
	}
}

// Encode encodes pixel data to JPEG 2000 format
// pixelData: raw pixel data (interleaved for multi-component, planar format as [][]int32 also supported)
func (e *Encoder) Encode(pixelData []byte) ([]byte, error) {
	// Validate parameters
	if err := e.validateParams(); err != nil {
		return nil, fmt.Errorf("invalid encoding parameters: %w", err)
	}

	// Convert pixel data to component arrays
	if err := e.convertPixelData(pixelData); err != nil {
		return nil, fmt.Errorf("failed to convert pixel data: %w", err)
	}

	// Apply DC level shift for unsigned data
	e.applyDCLevelShift()

	// Build codestream
	codestream, err := e.buildCodestream()
	if err != nil {
		return nil, fmt.Errorf("failed to build codestream: %w", err)
	}

	return codestream, nil
}

// EncodeComponents encodes component data directly (for testing)
func (e *Encoder) EncodeComponents(componentData [][]int32) ([]byte, error) {
	// Validate parameters
	if err := e.validateParams(); err != nil {
		return nil, fmt.Errorf("invalid encoding parameters: %w", err)
	}

	// Validate component data
	if len(componentData) != e.params.Components {
		return nil, fmt.Errorf("expected %d components, got %d", e.params.Components, len(componentData))
	}

	expectedSize := e.params.Width * e.params.Height
	for i, comp := range componentData {
		if len(comp) != expectedSize {
			return nil, fmt.Errorf("component %d: expected %d pixels, got %d", i, expectedSize, len(comp))
		}
	}

	// Copy component data (we need to modify it for DC level shift)
	e.data = make([][]int32, len(componentData))
	for i := range componentData {
		e.data[i] = make([]int32, len(componentData[i]))
		copy(e.data[i], componentData[i])
	}

	// Apply DC level shift for unsigned data
	e.applyDCLevelShift()

	// Build codestream
	codestream, err := e.buildCodestream()
	if err != nil {
		return nil, fmt.Errorf("failed to build codestream: %w", err)
	}

	return codestream, nil
}

// validateParams validates encoding parameters
func (e *Encoder) validateParams() error {
	p := e.params

	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("invalid dimensions: %dx%d", p.Width, p.Height)
	}

	if p.Components <= 0 || p.Components > 4 {
		return fmt.Errorf("invalid number of components: %d (must be 1-4)", p.Components)
	}

	if p.BitDepth < 1 || p.BitDepth > 16 {
		return fmt.Errorf("invalid bit depth: %d (must be 1-16)", p.BitDepth)
	}

	if p.NumLevels < 0 || p.NumLevels > 6 {
		return fmt.Errorf("invalid decomposition levels: %d (must be 0-6)", p.NumLevels)
	}

	if p.CodeBlockWidth < 4 || p.CodeBlockWidth > 1024 || !isPowerOfTwo(p.CodeBlockWidth) {
		return fmt.Errorf("invalid code-block width: %d (must be power of 2, 4-1024)", p.CodeBlockWidth)
	}

	if p.CodeBlockHeight < 4 || p.CodeBlockHeight > 1024 || !isPowerOfTwo(p.CodeBlockHeight) {
		return fmt.Errorf("invalid code-block height: %d (must be power of 2, 4-1024)", p.CodeBlockHeight)
	}

	if p.NumLayers <= 0 {
		return fmt.Errorf("invalid number of layers: %d (must be > 0)", p.NumLayers)
	}

	return nil
}

// convertPixelData converts byte array to component arrays
func (e *Encoder) convertPixelData(pixelData []byte) error {
	p := e.params
	numPixels := p.Width * p.Height
	expectedBytes := numPixels * p.Components * ((p.BitDepth + 7) / 8)

	if len(pixelData) < expectedBytes {
		return fmt.Errorf("insufficient pixel data: got %d bytes, need %d", len(pixelData), expectedBytes)
	}

	// Initialize component arrays
	e.data = make([][]int32, p.Components)
	for i := range e.data {
		e.data[i] = make([]int32, numPixels)
	}

	// Convert based on bit depth
	if p.BitDepth <= 8 {
		// 8-bit data
		for i := 0; i < numPixels; i++ {
			for c := 0; c < p.Components; c++ {
				val := int32(pixelData[i*p.Components+c])
				if p.IsSigned && val >= 128 {
					val -= 256
				}
				e.data[c][i] = val
			}
		}
	} else {
		// 16-bit data (little-endian)
		for i := 0; i < numPixels; i++ {
			for c := 0; c < p.Components; c++ {
				idx := (i*p.Components + c) * 2
				val := int32(pixelData[idx]) | (int32(pixelData[idx+1]) << 8)
				if p.IsSigned && val >= (1<<(p.BitDepth-1)) {
					val -= (1 << p.BitDepth)
				}
				e.data[c][i] = val
			}
		}
	}

	return nil
}

// buildCodestream builds the JPEG 2000 codestream
func (e *Encoder) buildCodestream() ([]byte, error) {
	buf := &bytes.Buffer{}

	// Write SOC (Start of Codestream)
	if err := binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSOC)); err != nil {
		return nil, err
	}

	// Write SIZ (Image and Tile Size)
	if err := e.writeSIZ(buf); err != nil {
		return nil, fmt.Errorf("failed to write SIZ: %w", err)
	}

	// Write COD (Coding Style Default)
	if err := e.writeCOD(buf); err != nil {
		return nil, fmt.Errorf("failed to write COD: %w", err)
	}

	// Write QCD (Quantization Default)
	if err := e.writeQCD(buf); err != nil {
		return nil, fmt.Errorf("failed to write QCD: %w", err)
	}

	// Write tiles
	if err := e.writeTiles(buf); err != nil {
		return nil, fmt.Errorf("failed to write tiles: %w", err)
	}

	// Write EOC (End of Codestream)
	if err := binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerEOC)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeSIZ writes the SIZ (Image and Tile Size) segment
func (e *Encoder) writeSIZ(buf *bytes.Buffer) error {
	p := e.params

	sizData := &bytes.Buffer{}

	// Rsiz - Capabilities (0 = baseline)
	_ = binary.Write(sizData, binary.BigEndian, uint16(0))

	// Xsiz, Ysiz - Image size
	_ = binary.Write(sizData, binary.BigEndian, uint32(p.Width))
	_ = binary.Write(sizData, binary.BigEndian, uint32(p.Height))

	// XOsiz, YOsiz - Image offset
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))

	// XTsiz, YTsiz - Tile size
	tileWidth := p.TileWidth
	tileHeight := p.TileHeight
	if tileWidth == 0 {
		tileWidth = p.Width
	}
	if tileHeight == 0 {
		tileHeight = p.Height
	}
	_ = binary.Write(sizData, binary.BigEndian, uint32(tileWidth))
	_ = binary.Write(sizData, binary.BigEndian, uint32(tileHeight))

	// XTOsiz, YTOsiz - Tile offset
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))
	_ = binary.Write(sizData, binary.BigEndian, uint32(0))

	// Csiz - Number of components
	_ = binary.Write(sizData, binary.BigEndian, uint16(p.Components))

	// Component information
	ssiz := uint8(p.BitDepth - 1)
	if p.IsSigned {
		ssiz |= 0x80
	}
	for i := 0; i < p.Components; i++ {
		_ = binary.Write(sizData, binary.BigEndian, ssiz)
		_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // XRsiz - horizontal separation
		_ = binary.Write(sizData, binary.BigEndian, uint8(1)) // YRsiz - vertical separation
	}

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSIZ))
	_ = binary.Write(buf, binary.BigEndian, uint16(sizData.Len()+2))
	buf.Write(sizData.Bytes())

	return nil
}

// writeCOD writes the COD (Coding Style Default) segment
func (e *Encoder) writeCOD(buf *bytes.Buffer) error {
	p := e.params

	codData := &bytes.Buffer{}

	// Scod - Coding style parameters
	scod := uint8(0)
	_ = binary.Write(codData, binary.BigEndian, scod)

	// SGcod - Progression order and layers
	_ = binary.Write(codData, binary.BigEndian, p.ProgressionOrder)
	_ = binary.Write(codData, binary.BigEndian, uint16(p.NumLayers))

	// MCT - Multiple component transformation (1 for RGB, 0 for grayscale)
	mct := uint8(0)
	if p.Components == 3 {
		mct = 1
	}
	_ = binary.Write(codData, binary.BigEndian, mct)

	// SPcod - Decomposition levels and code-block size
	_ = binary.Write(codData, binary.BigEndian, uint8(p.NumLevels))

	// Code-block size (log2(width) - 2, log2(height) - 2)
	cbWidthExp := uint8(log2(p.CodeBlockWidth) - 2)
	cbHeightExp := uint8(log2(p.CodeBlockHeight) - 2)
	_ = binary.Write(codData, binary.BigEndian, cbWidthExp)
	_ = binary.Write(codData, binary.BigEndian, cbHeightExp)

	// Code-block style
	// Bit 2 (0x04): Termination on each coding pass (TERMALL mode)
	// Enable TERMALL for ALL multi-layer encoding (both lossless and lossy)
	// This is required because encoder writes PassLengths metadata for all multi-layer
	codeBlockStyle := uint8(0)
	if p.NumLayers > 1 {
		codeBlockStyle |= 0x04 // Enable TERMALL mode for all multi-layer encoding
	}
	_ = binary.Write(codData, binary.BigEndian, codeBlockStyle)

	// Transformation (0 = 9/7 irreversible, 1 = 5/3 reversible)
	transform := uint8(1)
	if !p.Lossless {
		transform = 0
	}
	_ = binary.Write(codData, binary.BigEndian, transform)

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerCOD))
	_ = binary.Write(buf, binary.BigEndian, uint16(codData.Len()+2))
	buf.Write(codData.Bytes())

	return nil
}

// writeQCD writes the QCD (Quantization Default) segment
func (e *Encoder) writeQCD(buf *bytes.Buffer) error {
	p := e.params

	qcdData := &bytes.Buffer{}

	if p.Lossless {
		// Lossless mode: no quantization (style 0)
		// Sqcd - bits 0-4: guard bits (2), bits 5-7: quantization type (0 = no quantization)
		sqcd := uint8(0<<5 | 2) // No quantization, 2 guard bits
		_ = binary.Write(qcdData, binary.BigEndian, sqcd)

		// SPqcd - Quantization step size for each subband
		// For lossless: exponent only (8 bits), no mantissa
		numSubbands := 3*p.NumLevels + 1
		for i := 0; i < numSubbands; i++ {
			// Exponent = bitDepth (shifted left by 3 bits)
			expn := uint8(p.BitDepth << 3)
			_ = binary.Write(qcdData, binary.BigEndian, expn)
		}
	} else {
		// Lossy mode: scalar expounded quantization (style 2)
		// Calculate quantization parameters based on quality
		quantParams := CalculateQuantizationParams(p.Quality, p.NumLevels, p.BitDepth)

		// Sqcd - bits 0-4: guard bits, bits 5-7: quantization type (2 = scalar expounded)
		sqcd := uint8(2<<5 | quantParams.GuardBits)
		_ = binary.Write(qcdData, binary.BigEndian, sqcd)

		// SPqcd - Quantization step sizes for each subband
		// For scalar expounded: 16-bit value per subband (5-bit exponent, 11-bit mantissa)
		for _, encodedStep := range quantParams.EncodedSteps {
			_ = binary.Write(qcdData, binary.BigEndian, encodedStep)
		}
	}

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerQCD))
	_ = binary.Write(buf, binary.BigEndian, uint16(qcdData.Len()+2))
	buf.Write(qcdData.Bytes())

	return nil
}

// writeTiles writes all tile data
func (e *Encoder) writeTiles(buf *bytes.Buffer) error {
	p := e.params

	// Calculate tile dimensions
	tileWidth := p.TileWidth
	tileHeight := p.TileHeight
	if tileWidth == 0 {
		tileWidth = p.Width
	}
	if tileHeight == 0 {
		tileHeight = p.Height
	}

	numTilesX := (p.Width + tileWidth - 1) / tileWidth
	numTilesY := (p.Height + tileHeight - 1) / tileHeight
	numTiles := numTilesX * numTilesY

	// Write each tile
	for tileIdx := 0; tileIdx < numTiles; tileIdx++ {
		if err := e.writeTile(buf, tileIdx, tileWidth, tileHeight, numTilesX); err != nil {
			return fmt.Errorf("failed to write tile %d: %w", tileIdx, err)
		}
	}

	return nil
}

// writeTile writes a single tile
func (e *Encoder) writeTile(buf *bytes.Buffer, tileIdx, tileWidth, tileHeight, numTilesX int) error {
	// Calculate tile bounds
	tileX := tileIdx % numTilesX
	tileY := tileIdx / numTilesX

	x0 := tileX * tileWidth
	y0 := tileY * tileHeight
	x1 := x0 + tileWidth
	y1 := y0 + tileHeight

	if x1 > e.params.Width {
		x1 = e.params.Width
	}
	if y1 > e.params.Height {
		y1 = e.params.Height
	}

	actualWidth := x1 - x0
	actualHeight := y1 - y0

	// Extract tile data
	tileData := make([][]int32, e.params.Components)
	for c := 0; c < e.params.Components; c++ {
		tileData[c] = make([]int32, actualWidth*actualHeight)
		for ty := 0; ty < actualHeight; ty++ {
			srcIdx := (y0+ty)*e.params.Width + x0
			dstIdx := ty * actualWidth
			copy(tileData[c][dstIdx:dstIdx+actualWidth], e.data[c][srcIdx:srcIdx+actualWidth])
		}
	}

	// Apply wavelet transform
	transformedData, err := e.applyWaveletTransform(tileData, actualWidth, actualHeight)
	if err != nil {
		return fmt.Errorf("wavelet transform failed: %w", err)
	}

	// Encode tile data (simplified - just write placeholder)
	tileBytes := e.encodeTileData(transformedData, actualWidth, actualHeight)

	// Write SOT (Start of Tile)
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSOT))
	_ = binary.Write(buf, binary.BigEndian, uint16(10)) // Lsot

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx)) // Isot
	tilePartLength := len(tileBytes) + 14                    // SOT(12) + SOD(2) + data
	_ = binary.Write(buf, binary.BigEndian, uint32(tilePartLength))
	_ = binary.Write(buf, binary.BigEndian, uint8(0)) // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1)) // TNsot

	// Write SOD (Start of Data)
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSOD))

	// Write tile data
	buf.Write(tileBytes)

	return nil
}

// applyWaveletTransform applies wavelet transform to tile data
func (e *Encoder) applyWaveletTransform(tileData [][]int32, width, height int) ([][]int32, error) {
	if e.params.NumLevels == 0 {
		// No transform
		return tileData, nil
	}

	if e.params.Lossless {
		// Apply 5/3 reversible wavelet transform (lossless)
		transformed := make([][]int32, len(tileData))
		for c := 0; c < len(tileData); c++ {
			// Copy component data
			transformed[c] = make([]int32, len(tileData[c]))
			copy(transformed[c], tileData[c])

			// Apply forward multilevel DWT
			wavelet.ForwardMultilevel(transformed[c], width, height, e.params.NumLevels)
		}
		return transformed, nil
	} else {
		// Apply 9/7 irreversible wavelet transform (lossy)
		transformed := make([][]int32, len(tileData))

		// Calculate quantization parameters based on quality
		quantParams := CalculateQuantizationParams(e.params.Quality, e.params.NumLevels, e.params.BitDepth)

		for c := 0; c < len(tileData); c++ {
			// Convert to float64 for 9/7 transform
			floatData := wavelet.ConvertInt32ToFloat64(tileData[c])

			// Apply forward multilevel 9/7 DWT
			wavelet.ForwardMultilevel97(floatData, width, height, e.params.NumLevels)

			// Convert to int32 first
			coeffs := wavelet.ConvertFloat64ToInt32(floatData)

			// Apply quantization per subband
			transformed[c] = e.applyQuantizationBySubband(coeffs, width, height, quantParams.StepSizes)
		}
		return transformed, nil
	}
}

// applyQuantizationBySubband applies quantization to each subband separately
// coeffs: wavelet coefficients in subband layout
// width, height: dimensions of the full image
// stepSizes: quantization step sizes for each subband (LL, HL1, LH1, HH1, HL2, ...)
func (e *Encoder) applyQuantizationBySubband(coeffs []int32, width, height int, stepSizes []float64) []int32 {
	if len(stepSizes) == 0 || e.params.NumLevels == 0 {
		// No quantization
		return coeffs
	}

	quantized := make([]int32, len(coeffs))
	copy(quantized, coeffs)

	// Calculate subband dimensions for each level
	// After multilevel DWT, subbands are arranged as:
	// [LL_n] [HL_n] [LH_n] [HH_n] ... [HL_1] [LH_1] [HH_1]
	// where n = numLevels

	currentWidth := width
	currentHeight := height
	numLevels := e.params.NumLevels

	// Track which subband we're processing
	subbandIdx := 0

	// Process from coarsest to finest level
	for level := numLevels; level >= 1; level-- {
		// Calculate dimensions at this level
		levelWidth := (currentWidth + (1 << level) - 1) >> level
		levelHeight := (currentHeight + (1 << level) - 1) >> level

		// At the coarsest level, we also have LL subband
		if level == numLevels {
			// LL subband (low-pass both directions)
			stepSize := stepSizes[subbandIdx]
			e.quantizeSubband(quantized, 0, 0, levelWidth, levelHeight, currentWidth, stepSize)
			subbandIdx++
		}

		// HL subband (high-pass horizontal, low-pass vertical)
		stepSize := stepSizes[subbandIdx]
		e.quantizeSubband(quantized, levelWidth, 0, levelWidth, levelHeight, currentWidth, stepSize)
		subbandIdx++

		// LH subband (low-pass horizontal, high-pass vertical)
		stepSize = stepSizes[subbandIdx]
		e.quantizeSubband(quantized, 0, levelHeight, levelWidth, levelHeight, currentWidth, stepSize)
		subbandIdx++

		// HH subband (high-pass both directions)
		stepSize = stepSizes[subbandIdx]
		e.quantizeSubband(quantized, levelWidth, levelHeight, levelWidth, levelHeight, currentWidth, stepSize)
		subbandIdx++
	}

	return quantized
}

// quantizeSubband quantizes a single subband
// data: full coefficient array
// x0, y0: top-left corner of subband
// w, h: dimensions of subband
// stride: row stride (width of full image)
// stepSize: quantization step size
func (e *Encoder) quantizeSubband(data []int32, x0, y0, w, h, stride int, stepSize float64) {
	if stepSize <= 0 {
		return
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y0+y)*stride + (x0 + x)
			if idx < len(data) {
				// Quantize: round(coeff / stepSize)
				data[idx] = int32(math.Round(float64(data[idx]) / stepSize))
			}
		}
	}
}

// encodeTileData encodes tile data using T1 and T2 encoding
func (e *Encoder) encodeTileData(tileData [][]int32, width, height int) []byte {
	// Step 1: Partition into subbands and code-blocks
	// Step 2: Apply T1 EBCOT encoding to each code-block
	// Step 3: Collect code-blocks into T2 packet encoder
	// Step 4: Generate packets and write to bitstream

	// Initialize T2 packet encoder
	packetEnc := t2.NewPacketEncoder(
		e.params.Components,
		e.params.NumLayers,
		e.params.NumLevels+1,                           // numResolutions = numLevels + 1
		t2.ProgressionOrder(e.params.ProgressionOrder), // Cast uint8 to ProgressionOrder
	)

	// Process each component
	for comp := 0; comp < e.params.Components; comp++ {
		// Process each resolution level
		// Resolution 0 = LL subband (lowest frequency)
		// Resolution 1+ = HL, LH, HH subbands
		for res := 0; res <= e.params.NumLevels; res++ {
			// Get subband dimensions for this resolution
			subbands := e.getSubbandsForResolution(tileData[comp], width, height, res)

			// Process each subband
			globalCBIdx := 0
			for subbandIdx, subband := range subbands {
				// Partition subband into code-blocks
				codeBlocks := e.partitionIntoCodeBlocks(subband)

				// Encode each code-block with T1
				for cbIdx, cb := range codeBlocks {
					encodedCB := e.encodeCodeBlock(cb, cbIdx)

					// DEBUG: Track LH subband (res=1, subbandIdx=1 which is LH)
					if res == 1 && subbandIdx == 1 && cbIdx == 0 {
						// This is LH subband at resolution 1
						if e.params.NumLayers > 1 && encodedCB.LayerData != nil {
							fmt.Printf("[ENCODER LH CB] res=%d, NumLayers=%d, totalDataLen=%d\n",
								res, len(encodedCB.LayerData), len(encodedCB.Data))
							for layer := 0; layer < len(encodedCB.LayerData); layer++ {
								fmt.Printf("[ENCODER LH CB] Layer %d: data len=%d\n",
									layer, len(encodedCB.LayerData[layer]))
							}
						}
					}

					// Add to T2 packet encoder
					packetEnc.AddCodeBlock(comp, res, 0, encodedCB) // precinctIdx=0 (single precinct)
					globalCBIdx++
				}
			}
		}
	}

	// Generate packets
	packets, err := packetEnc.EncodePackets()
	if err != nil {
		// Fallback to empty packet on error
		return []byte{0x00}
	}

	// Write packets to bitstream with byte-stuffing
	buf := &bytes.Buffer{}
	for _, packet := range packets {
		// Write packet header with byte-stuffing
		writeWithByteStuffing(buf, packet.Header)
		// Write packet body with byte-stuffing
		writeWithByteStuffing(buf, packet.Body)

	}

	return buf.Bytes()
}

// writeWithByteStuffing writes data with JPEG 2000 byte-stuffing
// Any 0xFF byte must be followed by 0x00 to distinguish it from markers
func writeWithByteStuffing(buf *bytes.Buffer, data []byte) {
	for _, b := range data {
		buf.WriteByte(b)
		if b == 0xFF {
			buf.WriteByte(0x00) // Stuff byte
		}
	}
}

// subbandInfo represents a wavelet subband
type subbandInfo struct {
	data   []int32 // Coefficient data
	x0, y0 int     // Subband origin
	width  int     // Subband width
	height int     // Subband height
	band   int     // Band type: 0=LL, 1=HL, 2=LH, 3=HH
}

// getSubbandsForResolution extracts subbands for a specific resolution level
func (e *Encoder) getSubbandsForResolution(data []int32, width, height, resolution int) []subbandInfo {
	// Resolution 0 contains only LL subband (approximation)
	// Resolution r > 0 contains HL, LH, HH subbands from decomposition level r

	var subbands []subbandInfo

	if resolution == 0 {
		// LL subband (top-left quadrant after all decompositions)
		llWidth := width >> e.params.NumLevels
		llHeight := height >> e.params.NumLevels

		llData := make([]int32, llWidth*llHeight)
		for y := 0; y < llHeight; y++ {
			for x := 0; x < llWidth; x++ {
				llData[y*llWidth+x] = data[y*width+x]
			}
		}

		subbands = append(subbands, subbandInfo{
			data:   llData,
			x0:     0,
			y0:     0,
			width:  llWidth,
			height: llHeight,
			band:   0, // LL
		})
	} else {
		// For resolution r, extract HL, LH, HH from decomposition level (numLevels - r + 1)
		level := e.params.NumLevels - resolution + 1

		sbWidth := width >> level
		sbHeight := height >> level

		// HL (high-low): right half of top half
		hlData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				hlData[y*sbWidth+x] = data[y*width+(sbWidth+x)]
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hlData,
			x0:     sbWidth,
			y0:     0,
			width:  sbWidth,
			height: sbHeight,
			band:   1, // HL
		})

		// LH (low-high): left half of bottom half
		lhData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				lhData[y*sbWidth+x] = data[(sbHeight+y)*width+x]
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   lhData,
			x0:     0,
			y0:     sbHeight,
			width:  sbWidth,
			height: sbHeight,
			band:   2, // LH
		})

		// HH (high-high): right half of bottom half
		hhData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				hhData[y*sbWidth+x] = data[(sbHeight+y)*width+(sbWidth+x)]
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hhData,
			x0:     sbWidth,
			y0:     sbHeight,
			width:  sbWidth,
			height: sbHeight,
			band:   3, // HH
		})
	}

	return subbands
}

type codeBlockInfo struct {
	data     []int32
	width    int
	height   int
	globalX0 int // Global X position in coefficient array
	globalY0 int // Global Y position in coefficient array
}

// partitionIntoCodeBlocks partitions a subband into code-blocks
func (e *Encoder) partitionIntoCodeBlocks(subband subbandInfo) []codeBlockInfo {
	cbWidth := e.params.CodeBlockWidth
	cbHeight := e.params.CodeBlockHeight

	numCBX := (subband.width + cbWidth - 1) / cbWidth
	numCBY := (subband.height + cbHeight - 1) / cbHeight

	codeBlocks := make([]codeBlockInfo, 0, numCBX*numCBY)

	for cby := 0; cby < numCBY; cby++ {
		for cbx := 0; cbx < numCBX; cbx++ {
			// Calculate code-block bounds
			x0 := cbx * cbWidth
			y0 := cby * cbHeight
			x1 := x0 + cbWidth
			y1 := y0 + cbHeight

			if x1 > subband.width {
				x1 = subband.width
			}
			if y1 > subband.height {
				y1 = subband.height
			}

			actualWidth := x1 - x0
			actualHeight := y1 - y0

			// Extract code-block data
			cbData := make([]int32, actualWidth*actualHeight)
			for y := 0; y < actualHeight; y++ {
				for x := 0; x < actualWidth; x++ {
					srcIdx := (y0+y)*subband.width + (x0 + x)
					dstIdx := y*actualWidth + x
					cbData[dstIdx] = subband.data[srcIdx]
				}
			}

			// Store code-block with its dimensions and global position
			globalX0 := subband.x0 + x0
			globalY0 := subband.y0 + y0

			codeBlocks = append(codeBlocks, codeBlockInfo{
				data:     cbData,
				width:    actualWidth,
				height:   actualHeight,
				globalX0: globalX0,
				globalY0: globalY0,
			})
		}
	}

	return codeBlocks
}

// encodeCodeBlock encodes a single code-block using T1 EBCOT encoder
func (e *Encoder) encodeCodeBlock(cb codeBlockInfo, cbIdx int) *t2.PrecinctCodeBlock {
	// Use provided dimensions
	actualWidth := cb.width
	actualHeight := cb.height
	cbData := cb.data

	// Calculate max bitplane from data
	maxBitplane := calculateMaxBitplane(cbData)

	// Calculate number of coding passes
	// For lossless: encode all bit-planes
	// Each bit-plane has 3 passes: SPP, MRP, CP
	numPasses := (maxBitplane + 1) * 3
	if maxBitplane < 0 {
		// All zeros - still need at least 1 pass for valid packet header
		numPasses = 1
	}

	// Calculate zero bit-planes
	// ZeroBitPlanes = number of MSB bit-planes that are all zero
	// Formula: effectiveBitDepth - 1 - maxBitplane
	// Note: After wavelet transform, coefficients may need extra bits
	// 5/3 reversible wavelet adds 1 bit per decomposition level
	effectiveBitDepth := e.params.BitDepth + e.params.NumLevels

	zeroBitPlanes := 0
	if maxBitplane < 0 {
		// All data is zero, all bit-planes are zero
		zeroBitPlanes = effectiveBitDepth
	} else {
		zeroBitPlanes = effectiveBitDepth - 1 - maxBitplane
	}

	// Create T1 encoder
	t1Enc := t1.NewT1Encoder(actualWidth, actualHeight, 0)

	// Create PrecinctCodeBlock structure
	pcb := &t2.PrecinctCodeBlock{
		Index:          0, // Will be set by caller if needed
		X0:             cb.globalX0,
		Y0:             cb.globalY0,
		X1:             cb.globalX0 + actualWidth,
		Y1:             cb.globalY0 + actualHeight,
		Included:       false, // First inclusion in packet
		NumPassesTotal: numPasses,
		ZeroBitPlanes:  zeroBitPlanes,
	}

	// Check if we need multi-layer encoding
	if e.params.NumLayers > 1 {
		// Allocate passes to layers using simple algorithm
		layerAlloc := AllocateLayersSimple(numPasses, e.params.NumLayers, 1)

		// Compute layer boundaries (pass indices that end each layer)
		layerBoundaries := make([]int, e.params.NumLayers)
		for layer := 0; layer < e.params.NumLayers; layer++ {
			layerBoundaries[layer] = layerAlloc.GetPassesForLayer(0, layer)
		}

		// DEBUG: Track LH subband layer allocation
		if cb.globalX0 == 0 && cb.globalY0 == 8 {
			fmt.Printf("[ENCODER LH ALLOC] numPasses=%d, numLayers=%d\n", numPasses, e.params.NumLayers)
			for layer := 0; layer < e.params.NumLayers; layer++ {
				fmt.Printf("[ENCODER LH ALLOC]   layer[%d]: boundary=%d passes\n",
					layer, layerBoundaries[layer])
			}
		}

		// Multi-layer encoding: use layered encoder with layer boundaries
		// Pass actual lossless parameter to control context resets
		// TERMALL mode (bit 0x04 in COD) controls pass termination separately
		passes, completeData, err := t1Enc.EncodeLayered(cbData, numPasses, 0, layerBoundaries, e.params.Lossless)
		if err != nil || len(passes) == 0 {
			// Fallback to single layer on error
			encodedData := []byte{0x00}
			pcb.Data = encodedData
			pcb.LayerData = [][]byte{encodedData}
			pcb.LayerPasses = []int{1}

			// DEBUG: Track errors
			if cb.globalX0 == 0 && cb.globalY0 == 8 {
				fmt.Printf("[ENCODER LH ERROR] EncodeLayered failed: err=%v, len(passes)=%d\n", err, len(passes))
			}
			return pcb
		}

		// DEBUG: Track LH subband passes
		if cb.globalX0 == 0 && cb.globalY0 == 8 {
			fmt.Printf("[ENCODER LH PASSES] numPasses=%d, len(passes)=%d, len(completeData)=%d\n",
				numPasses, len(passes), len(completeData))
			for i, p := range passes {
				fmt.Printf("[ENCODER LH PASSES]   pass[%d]: ActualBytes=%d\n", i, p.ActualBytes)
			}
		}

		// Build layer metadata using pass Rate information
		// Following OpenJPEG: layers share complete data, metadata specifies byte ranges
		pcb.LayerData = make([][]byte, e.params.NumLayers)
		pcb.LayerPasses = make([]int, e.params.NumLayers)

		// Build per-pass length information (for TERMALL mode decoding)
		pcb.PassLengths = make([]int, len(passes))
		for i, pass := range passes {
			pcb.PassLengths[i] = pass.ActualBytes
		}

		prevByteEnd := 0
		for layer := 0; layer < e.params.NumLayers; layer++ {
			// Get cumulative passes for this layer
			totalPassesForLayer := layerAlloc.GetPassesForLayer(0, layer)
			pcb.LayerPasses[layer] = totalPassesForLayer

			// DEBUG: Track LH subband layer passes
			if cb.globalX0 == 0 && cb.globalY0 == 8 {
				fmt.Printf("[ENCODER LH LAYERPASSES] layer[%d]: totalPassesForLayer=%d\n",
					layer, totalPassesForLayer)
			}

			// Use ActualBytes from passes to determine byte end
			// ActualBytes is the actual position in the buffer (not including rate_extra_bytes estimate)
			var currentByteEnd int
			if totalPassesForLayer > 0 && totalPassesForLayer <= len(passes) {
				currentByteEnd = passes[totalPassesForLayer-1].ActualBytes
			} else {
				currentByteEnd = len(completeData)
			}

			// Clamp
			if currentByteEnd > len(completeData) {
				currentByteEnd = len(completeData)
			}
			if currentByteEnd < prevByteEnd {
				currentByteEnd = prevByteEnd
			}

			// Store incremental data for this layer (from prevByteEnd to currentByteEnd)
			pcb.LayerData[layer] = completeData[prevByteEnd:currentByteEnd]

			// DEBUG: Track LH subband layer data
			if cb.globalX0 == 0 && cb.globalY0 == 8 {
				fmt.Printf("[ENCODER LH LAYERDATA] layer[%d]: prevByteEnd=%d, currentByteEnd=%d, layerDataLen=%d\n",
					layer, prevByteEnd, currentByteEnd, len(pcb.LayerData[layer]))
			}

			prevByteEnd = currentByteEnd
		}

		// For backward compatibility, set Data to all passes
		pcb.Data = completeData

		// Set TERMALL flag for all multi-layer encoding
		pcb.UseTERMALL = e.params.NumLayers > 1

	} else {
		// Single layer: use original encoder
		encodedData, err := t1Enc.Encode(cbData, numPasses, 0)
		if err != nil {
			// Return minimal code-block on error
			encodedData = []byte{0x00}
			numPasses = 1
			maxBitplane = 0
			zeroBitPlanes = effectiveBitDepth
			pcb.NumPassesTotal = numPasses
			pcb.ZeroBitPlanes = zeroBitPlanes
		}
		pcb.Data = encodedData
	}

	return pcb
}

// calculateMaxBitplane finds the highest bit-plane that contains a '1' bit
func calculateMaxBitplane(data []int32) int {
	maxAbs := int32(0)
	for _, val := range data {
		absVal := val
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal > maxAbs {
			maxAbs = absVal
		}
	}

	if maxAbs == 0 {
		return -1
	}

	// Find highest bit set
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}

	return bitplane - 1
}

// Helper functions

func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

func log2(n int) int {
	result := 0
	for n > 1 {
		n >>= 1
		result++
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// applyDCLevelShift applies DC level shift for unsigned data
// For unsigned data: subtract 2^(bitDepth-1) to convert to signed range
func (e *Encoder) applyDCLevelShift() {
	if e.params.IsSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - subtract 2^(bitDepth-1)
	shift := int32(1 << (e.params.BitDepth - 1))
	for comp := range e.data {
		for i := range e.data[comp] {
			e.data[comp][i] -= shift
		}
	}
}
