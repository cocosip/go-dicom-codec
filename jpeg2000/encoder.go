package jpeg2000

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/colorspace"
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

	// Precinct parameters (0 = use default size of 2^15 = 32768)
	PrecinctWidth  int // Precinct width (power of 2, e.g., 128, 256, 512)
	PrecinctHeight int // Precinct height (power of 2, e.g., 128, 256, 512)

	// Lossy compression quality (1-100, only used when Lossless=false)
	// Higher values = better quality, lower compression
	// 100 = minimal quantization (near-lossless)
	// 50 = balanced quality/compression
	// 1 = maximum compression, lower quality
	Quality int // Default: 80

	// CustomQuantSteps allows caller to override quantization step sizes per subband (lossy only).
	// Length must be 3*NumLevels+1 when provided. Values are floating quant steps.
	CustomQuantSteps []float64

	// TargetRatio optionally requests a target compression ratio (orig_size / compressed_size).
	// Used by rate-distortion truncation when >0.
	TargetRatio float64

	// Progression order
	ProgressionOrder uint8 // 0=LRCP, 1=RLCP, 2=RPCL, 3=PCRL, 4=CPRL

	// Layer parameters
	NumLayers int // Number of quality layers (default 1)

	// PCRD options
	UsePCRDOpt          bool
	LayerBudgetStrategy string
	LambdaTolerance     float64

	// AppendLosslessLayer will append a final lossless layer (rate=0) after target-rate layers.
	AppendLosslessLayer bool

	// Region of Interest (ROI)
	ROI *ROIParams // Optional single-rectangle ROI with MaxShift
	// ROIConfig supports multiple ROI entries (MVP: multiple rectangles, MaxShift only)
	ROIConfig *ROIConfig

	// Custom multi-component transform (Part 2 style, experimental)
	// If provided, overrides default RCT/ICT for multi-component images.
	MCTMatrix        [][]float64
	InverseMCTMatrix [][]float64
	MCTReversible    bool
	EnableMCT        bool // when false, skip RCT/ICT and custom MCT

	// Optional per-component offsets (Part 2 offset array)
	MCTOffsets           []int32
	MCTNormScale         float64
	MCTAssocType         uint8
	MCTMatrixElementType uint8 // 0=int32, 1=float32 (default)
	MCOPrecision         uint8
	MCORecordOrder       []uint8
	MCTBindings          []MCTBindingParams

	// Block encoder factory (for HTJ2K support)
	// If nil, defaults to EBCOT T1 encoder
	BlockEncoderFactory func(width, height int) BlockEncoder

}

// BlockEncoder is an interface for T1 block encoders (EBCOT or HTJ2K)
type BlockEncoder interface {
	Encode(coeffs []int32, numPasses int, roiShift int) ([]byte, error)
}

type MCTBindingParams struct {
	AssocType      uint8
	ComponentIDs   []uint16
	MCTRecordOrder []uint8
	MCOPrecision   uint8
	MCONormScale   float64
	Matrix         [][]float64
	Inverse        [][]float64
	Offsets        []int32
	ElementType    uint8
}

// DefaultEncodeParams returns default encoding parameters for lossless encoding
func DefaultEncodeParams(width, height, components, bitDepth int, isSigned bool) *EncodeParams {
	return &EncodeParams{
		Width:               width,
		Height:              height,
		Components:          components,
		BitDepth:            bitDepth,
		IsSigned:            isSigned,
		TileWidth:           0, // Single tile
		TileHeight:          0, // Single tile
		NumLevels:           5, // 5 DWT levels
		Lossless:            true,
		Quality:             80, // Default quality for lossy mode
		CodeBlockWidth:      64,
		CodeBlockHeight:     64,
		PrecinctWidth:       0, // Default (2^15)
		PrecinctHeight:      0, // Default (2^15)
		TargetRatio:         0,
		ProgressionOrder:    0, // LRCP
		NumLayers:           1,
		UsePCRDOpt:          false,
		LayerBudgetStrategy: "EXPONENTIAL",
		LambdaTolerance:     0.01,
		AppendLosslessLayer: false,
		ROI:                 nil,
		EnableMCT:           true,
	}
}

// Encoder implements JPEG 2000 encoding
type Encoder struct {
	params    *EncodeParams
	data      [][]int32 // [component][pixel]
	roiShifts []int
	roiRects  [][]roiRect // per-component rectangles
	roiStyles []byte      // per-component Srgn value: 0=MaxShift, 1=GeneralScaling
	roiMasks  []*roiMask  // per-component ROI mask (full-res)
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

	// Apply DC level shift BEFORE MCT (to match OpenJPEG order)
	// OpenJPEG: DC shift -> MCT -> DWT -> T1
	e.applyDCLevelShift()

	if e.params.EnableMCT {
		if e.params.MCTMatrix != nil && len(e.params.MCTMatrix) == e.params.Components {
			e.applyCustomMCT()
		} else if e.params.Components == 3 {
			if e.params.Lossless {
				y, cb, cr := colorspace.ApplyRCTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			} else {
				y, cb, cr := colorspace.ApplyICTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			}
		}
	}

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

	// Apply DC level shift BEFORE MCT (to match OpenJPEG order)
	// OpenJPEG: DC shift -> MCT -> DWT -> T1
	e.applyDCLevelShift()

	if e.params.EnableMCT {
		if e.params.MCTMatrix != nil && len(e.params.MCTMatrix) == e.params.Components {
			e.applyCustomMCT()
		} else if e.params.Components == 3 {
			if e.params.Lossless {
				y, cb, cr := colorspace.ApplyRCTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			} else {
				y, cb, cr := colorspace.ApplyICTToComponents(e.data[0], e.data[1], e.data[2])
				e.data[0], e.data[1], e.data[2] = y, cb, cr
			}
		}
	}

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

	if p.NumLayers < 1 {
		return fmt.Errorf("invalid number of layers: %d (must be > 0)", p.NumLayers)
	}

	if p.ROIConfig != nil && !p.ROIConfig.IsEmpty() {
		if err := p.ROIConfig.Validate(p.Width, p.Height); err != nil {
			return fmt.Errorf("invalid ROIConfig: %w", err)
		}
	}

	if p.ROI != nil {
		if !p.ROI.IsValid(p.Width, p.Height) {
			return fmt.Errorf("invalid ROI parameters: %+v", *p.ROI)
		}
		if p.ROI.Shift > 255 {
			return fmt.Errorf("invalid ROI shift: %d (must be <=255)", p.ROI.Shift)
		}
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
	// Resolve ROI (supports legacy ROI and ROIConfig)
	if err := e.resolveROI(); err != nil {
		return nil, fmt.Errorf("failed to resolve ROI: %w", err)
	}

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

	// Write RGN (ROI) if present
	if err := e.writeRGN(buf); err != nil {
		return nil, fmt.Errorf("failed to write RGN: %w", err)
	}

	// Write COM (private ROI metadata) if ROI is enabled
	if err := e.writeCOM(buf); err != nil {
		return nil, fmt.Errorf("failed to write COM: %w", err)
	}

	// Write MCT/MCC (Part 2-style) if provided
	if err := e.writeMCTAndMCC(buf); err != nil {
		return nil, fmt.Errorf("failed to write MCT/MCC: %w", err)
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

// applyCustomMCT applies a custom multi-component transform from params.MCTMatrix
func (e *Encoder) applyCustomMCT() {
	p := e.params
	if p.MCTMatrix == nil || len(p.MCTMatrix) != p.Components {
		return
	}
	width := p.Width
	height := p.Height
	n := width * height
	comps := p.Components
	out := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		out[c] = make([]int32, n)
	}
	if p.MCTMatrixElementType == 0 && p.MCTReversible && p.MCTNormScale == 1 {
		im := make([][]int32, comps)
		for r := 0; r < comps; r++ {
			im[r] = make([]int32, comps)
			for k := 0; k < comps; k++ {
				im[r][k] = int32(math.Round(p.MCTMatrix[r][k]))
			}
		}
		for i := 0; i < n; i++ {
			for r := 0; r < comps; r++ {
				var sum int64
				for k := 0; k < comps; k++ {
					sum += int64(im[r][k]) * int64(e.data[k][i])
				}
				out[r][i] = int32(sum)
			}
		}
	} else {
		scale := p.MCTNormScale
		if scale == 0 {
			scale = 1
		}
		for i := 0; i < n; i++ {
			for r := 0; r < comps; r++ {
				sum := 0.0
				for k := 0; k < comps; k++ {
					sum += (p.MCTMatrix[r][k] * scale) * float64(e.data[k][i])
				}
				out[r][i] = int32(math.Round(sum))
			}
		}
	}
	e.data = out
}

// writeMCTAndMCC writes Part 2 MCT/MCC markers carrying the custom inverse matrix (experimental)
func (e *Encoder) writeMCTAndMCC(buf *bytes.Buffer) error {
	p := e.params
	if len(p.MCTBindings) > 0 {
		nextID := uint8(0)
		for idx, b := range p.MCTBindings {
			rows := len(b.Matrix)
			cols := len(b.Matrix[0])
			mct := &bytes.Buffer{}
			mct.WriteByte(nextID)
			et := b.ElementType
			if et == 0 {
				et = 1
			}
			mct.WriteByte(et)
			mct.WriteByte(1)
			_ = binary.Write(mct, binary.BigEndian, uint16(rows))
			_ = binary.Write(mct, binary.BigEndian, uint16(cols))
			rev := byte(0)
			mct.WriteByte(rev)
			inv := b.Inverse
			if inv == nil || len(inv) != rows {
				inv = make([][]float64, rows)
				for r := 0; r < rows; r++ {
					inv[r] = make([]float64, cols)
					for c := 0; c < cols; c++ {
						if r == c {
							inv[r][c] = 1
						}
					}
				}
			}
			if et == 1 {
				for r := 0; r < rows; r++ {
					for c := 0; c < cols; c++ {
						_ = binary.Write(mct, binary.BigEndian, math.Float32bits(float32(inv[r][c])))
					}
				}
			} else {
				scale := b.MCONormScale
				if scale == 0 {
					scale = 1
				}
				for r := 0; r < rows; r++ {
					for c := 0; c < cols; c++ {
						val := int32(math.Round(inv[r][c] * scale))
						_ = binary.Write(mct, binary.BigEndian, uint32(val))
					}
				}
			}
			if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCT); err != nil {
				return err
			}
			mctLen := uint16(2 + mct.Len())
			if err := binary.Write(buf, binary.BigEndian, mctLen); err != nil {
				return err
			}
			if _, err := buf.Write(mct.Bytes()); err != nil {
				return err
			}
			mcc := &bytes.Buffer{}
			mcc.WriteByte(uint8(idx))
			assoc := b.AssocType
			if assoc == 0 {
				assoc = 1
			}
			mcc.WriteByte(assoc)
			_ = binary.Write(mcc, binary.BigEndian, uint16(len(b.ComponentIDs)))
			for _, cid := range b.ComponentIDs {
				_ = binary.Write(mcc, binary.BigEndian, cid)
			}
			var order []uint8
			if len(b.MCTRecordOrder) > 0 {
				order = b.MCTRecordOrder
			} else if b.Offsets != nil && len(b.Offsets) == len(b.ComponentIDs) {
				order = []uint8{nextID, nextID + 1}
			} else {
				order = []uint8{nextID}
			}
			mcc.WriteByte(uint8(len(order)))
			for _, id := range order {
				mcc.WriteByte(id)
			}
			if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCC); err != nil {
				return err
			}
			mccLen := uint16(2 + mcc.Len())
			if err := binary.Write(buf, binary.BigEndian, mccLen); err != nil {
				return err
			}
			if _, err := buf.Write(mcc.Bytes()); err != nil {
				return err
			}
			if b.Offsets != nil && len(b.Offsets) == len(b.ComponentIDs) {
				off := &bytes.Buffer{}
				off.WriteByte(nextID + 1)
				off.WriteByte(0)
				off.WriteByte(2)
				_ = binary.Write(off, binary.BigEndian, uint16(len(b.ComponentIDs)))
				_ = binary.Write(off, binary.BigEndian, uint16(1))
				off.WriteByte(0)
				for i := 0; i < len(b.ComponentIDs); i++ {
					_ = binary.Write(off, binary.BigEndian, uint32(b.Offsets[i]))
				}
				if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCT); err != nil {
					return err
				}
				offLen := uint16(2 + off.Len())
				if err := binary.Write(buf, binary.BigEndian, offLen); err != nil {
					return err
				}
				if _, err := buf.Write(off.Bytes()); err != nil {
					return err
				}
			}
			if b.MCONormScale != 0 || b.MCOPrecision != 0 || len(b.MCTRecordOrder) > 0 {
				mco := &bytes.Buffer{}
				mco.WriteByte(uint8(idx))
				if b.MCONormScale != 0 {
					mco.WriteByte(codestream.MCOOptNormScale)
					_ = binary.Write(mco, binary.BigEndian, math.Float32bits(float32(b.MCONormScale)))
				}
				if b.MCOPrecision != 0 {
					mco.WriteByte(codestream.MCOOptPrecision)
					mco.WriteByte(b.MCOPrecision)
				}
				if len(b.MCTRecordOrder) > 0 {
					mco.WriteByte(codestream.MCOOptRecordOrder)
					mco.WriteByte(uint8(len(b.MCTRecordOrder)))
					for _, id := range b.MCTRecordOrder {
						mco.WriteByte(id)
					}
				}
				if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCO); err != nil {
					return err
				}
				mcoLen := uint16(2 + mco.Len())
				if err := binary.Write(buf, binary.BigEndian, mcoLen); err != nil {
					return err
				}
				if _, err := buf.Write(mco.Bytes()); err != nil {
					return err
				}
			}
			nextID += 2
		}
		return nil
	}
	if p.MCTMatrix == nil || len(p.MCTMatrix) != p.Components {
		return nil
	}
	rows := len(p.MCTMatrix)
	cols := len(p.MCTMatrix[0])
	for _, row := range p.MCTMatrix {
		if len(row) != cols {
			return nil
		}
	}
	// Build MCT payload aligned to Part 2 (simplified header)
	// Zmct(1) Ymct(1=FLOAT32) Xmct(1=DECORRELATION) rows(2) cols(2) rev(1) matrix
	mct := &bytes.Buffer{}
	mct.WriteByte(0) // Zmct index (decorrelation matrix)
	et := p.MCTMatrixElementType
	if et == 0 {
		et = 1
	}
	mct.WriteByte(et) // Ymct element type
	mct.WriteByte(1)  // Xmct array type: decorrelation matrix
	_ = binary.Write(mct, binary.BigEndian, uint16(rows))
	_ = binary.Write(mct, binary.BigEndian, uint16(cols))
	rev := byte(0)
	if p.MCTReversible {
		rev = 1
	}
	mct.WriteByte(rev)
	inv := p.InverseMCTMatrix
	if inv == nil || len(inv) != rows {
		inv = make([][]float64, rows)
		for r := 0; r < rows; r++ {
			inv[r] = make([]float64, cols)
			for c := 0; c < cols; c++ {
				if r == c {
					inv[r][c] = 1
				}
			}
		}
	}
	if et == 1 { // float32
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				_ = binary.Write(mct, binary.BigEndian, math.Float32bits(float32(inv[r][c])))
			}
		}
	} else { // int32
		scale := p.MCTNormScale
		if scale == 0 {
			scale = 1
		}
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				val := int32(math.Round(inv[r][c] * scale))
				_ = binary.Write(mct, binary.BigEndian, uint32(val))
			}
		}
	}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCT); err != nil {
		return err
	}
	mctLen := uint16(2 + mct.Len())
	if err := binary.Write(buf, binary.BigEndian, mctLen); err != nil {
		return err
	}
	if _, err := buf.Write(mct.Bytes()); err != nil {
		return err
	}

	// Build MCC payload aligned to Part 2 (simplified association)
	// Zmcc(1) Ymcc(1=simple decorrelation) Ncomp(2) compIds(2*N) mctIndex(1)
	mcc := &bytes.Buffer{}
	mcc.WriteByte(0) // Zmcc index
	assoc := p.MCTAssocType
	if assoc == 0 {
		assoc = 1
	}
	mcc.WriteByte(assoc) // Ymcc type
	_ = binary.Write(mcc, binary.BigEndian, uint16(p.Components))
	for i := 0; i < p.Components; i++ {
		_ = binary.Write(mcc, binary.BigEndian, uint16(i))
	}
	// Write record indices
	if len(p.MCORecordOrder) > 0 {
		mcc.WriteByte(uint8(len(p.MCORecordOrder)))
		for _, id := range p.MCORecordOrder {
			mcc.WriteByte(id)
		}
	} else if p.MCTOffsets != nil && len(p.MCTOffsets) == p.Components {
		// Default order [0,1]: matrix then offset
		mcc.WriteByte(2)
		mcc.WriteByte(0)
		mcc.WriteByte(1)
	} else {
		mcc.WriteByte(1)
		mcc.WriteByte(0)
	}
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCC); err != nil {
		return err
	}
	mccLen := uint16(2 + mcc.Len())
	if err := binary.Write(buf, binary.BigEndian, mccLen); err != nil {
		return err
	}
	if _, err := buf.Write(mcc.Bytes()); err != nil {
		return err
	}

	// Optional offset record
	if p.MCTOffsets != nil && len(p.MCTOffsets) == p.Components {
		off := &bytes.Buffer{}
		off.WriteByte(1) // Zmct index (offset array)
		off.WriteByte(0) // Ymct element type: int32
		off.WriteByte(2) // Xmct array type: offset
		_ = binary.Write(off, binary.BigEndian, uint16(p.Components))
		_ = binary.Write(off, binary.BigEndian, uint16(1))
		off.WriteByte(0) // rev flag not used
		for i := 0; i < p.Components; i++ {
			_ = binary.Write(off, binary.BigEndian, uint32(p.MCTOffsets[i]))
		}
		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCT); err != nil {
			return err
		}
		offLen := uint16(2 + off.Len())
		if err := binary.Write(buf, binary.BigEndian, offLen); err != nil {
			return err
		}
		if _, err := buf.Write(off.Bytes()); err != nil {
			return err
		}
		// We skip writing MCC for offsets (decoder applies when present)
	}

	if p.MCTNormScale != 0 || p.MCOPrecision != 0 || len(p.MCORecordOrder) > 0 {
		mco := &bytes.Buffer{}
		mco.WriteByte(0) // Index
		if p.MCTNormScale != 0 {
			mco.WriteByte(codestream.MCOOptNormScale)
			_ = binary.Write(mco, binary.BigEndian, math.Float32bits(float32(p.MCTNormScale)))
		}
		if p.MCOPrecision != 0 {
			mco.WriteByte(codestream.MCOOptPrecision)
			mco.WriteByte(p.MCOPrecision)
		}
		if len(p.MCORecordOrder) > 0 {
			mco.WriteByte(codestream.MCOOptRecordOrder)
			mco.WriteByte(uint8(len(p.MCORecordOrder)))
			for _, id := range p.MCORecordOrder {
				mco.WriteByte(id)
			}
		}
		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerMCO); err != nil {
			return err
		}
		mcoLen := uint16(2 + mco.Len())
		if err := binary.Write(buf, binary.BigEndian, mcoLen); err != nil {
			return err
		}
		if _, err := buf.Write(mco.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// resolveROI normalizes ROI inputs (legacy ROI or ROIConfig) into internal slices.
func (e *Encoder) resolveROI() error {
	e.roiShifts = nil
	e.roiRects = nil
	e.roiStyles = nil
	e.roiMasks = nil

	// ROIConfig takes priority when present
	if e.params.ROIConfig != nil && !e.params.ROIConfig.IsEmpty() {
		style, shifts, rectsByComp, err := e.params.ROIConfig.ResolveRectangles(e.params.Width, e.params.Height, e.params.Components)
		if err != nil {
			return err
		}
		e.roiShifts = shifts
		e.roiRects = rectsByComp
		e.roiMasks = buildMasksFromConfig(e.params.Width, e.params.Height, e.params.Components, rectsByComp, e.params.ROIConfig)
		if len(shifts) > 0 {
			e.roiStyles = make([]byte, len(shifts))
			for i := range e.roiStyles {
				e.roiStyles[i] = style
			}
		}
		return nil
	}

	// Legacy single-rectangle ROI
	if e.params.ROI != nil {
		if !e.params.ROI.IsValid(e.params.Width, e.params.Height) {
			return fmt.Errorf("invalid ROI parameters: %+v", *e.params.ROI)
		}
		e.roiShifts = make([]int, e.params.Components)
		e.roiRects = make([][]roiRect, e.params.Components)
		e.roiStyles = make([]byte, e.params.Components)
		e.roiMasks = make([]*roiMask, e.params.Components)
		for c := 0; c < e.params.Components; c++ {
			e.roiShifts[c] = e.params.ROI.Shift
			e.roiStyles[c] = 0
			e.roiRects[c] = []roiRect{{
				x0: e.params.ROI.X0,
				y0: e.params.ROI.Y0,
				x1: e.params.ROI.X0 + e.params.ROI.Width,
				y1: e.params.ROI.Y0 + e.params.ROI.Height,
			}}
			e.roiMasks[c] = newROIMask(e.params.Width, e.params.Height)
			e.roiMasks[c].setRect(e.params.ROI.X0, e.params.ROI.Y0, e.params.ROI.X0+e.params.ROI.Width, e.params.ROI.Y0+e.params.ROI.Height)
		}
	}

	return nil
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
	// Bit 0: Precinct defined (1 if custom precinct sizes are used)
	// Bit 6 (0x40): HTJ2K mode (JPEG 2000 Part 15)
	scod := uint8(0)
	if p.PrecinctWidth > 0 || p.PrecinctHeight > 0 {
		scod |= 0x01 // Enable precinct sizes
	}
	// Set HTJ2K bit if using HTJ2K block encoder
	if p.BlockEncoderFactory != nil {
		scod |= 0x40 // Enable HTJ2K mode
	}
	_ = binary.Write(codData, binary.BigEndian, scod)

	// SGcod - Progression order and layers
	_ = binary.Write(codData, binary.BigEndian, p.ProgressionOrder)
	_ = binary.Write(codData, binary.BigEndian, uint16(p.NumLayers))

	// MCT - Multiple component transformation (1 for RGB, 0 for grayscale)
	mct := uint8(0)
	if p.Components >= 3 || (p.MCTMatrix != nil && len(p.MCTMatrix) == p.Components) {
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
	// Enable TERMALL for layered encoding (NumLayers>1) and target-ratio encoding.
	// This is required because encoder writes PassLengths metadata for layered modes.
	codeBlockStyle := uint8(0)
	if p.NumLayers > 1 || p.TargetRatio > 0 {
		codeBlockStyle |= 0x04
	}
	_ = binary.Write(codData, binary.BigEndian, codeBlockStyle)

	// Transformation (0 = 9/7 irreversible, 1 = 5/3 reversible)
	transform := uint8(1)
	if !p.Lossless {
		transform = 0
	}
	_ = binary.Write(codData, binary.BigEndian, transform)

	// Write precinct sizes if enabled (Scod bit 0 = 1)
	if scod&0x01 != 0 {
		// One precinct size per resolution level (numLevels + 1)
		numResolutions := p.NumLevels + 1
		for r := 0; r < numResolutions; r++ {
			// Calculate precinct size for this resolution level
			// Default precinct size is 2^15 (32768) if not specified
			ppx, ppy := e.getPrecinctSizeExponents(r)

			// Pack PPx and PPy into single byte: PPy (high 4 bits) | PPx (low 4 bits)
			ppxppy := (ppy << 4) | ppx
			_ = binary.Write(codData, binary.BigEndian, ppxppy)
		}
	}

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerCOD))
	_ = binary.Write(buf, binary.BigEndian, uint16(codData.Len()+2))
	buf.Write(codData.Bytes())

	return nil
}

// getPrecinctSize returns the actual precinct dimensions (in pixels) for a given resolution level
func (e *Encoder) getPrecinctSize(resolutionLevel int) (width, height int) {
	ppx, ppy := e.getPrecinctSizeExponents(resolutionLevel)
	return 1 << ppx, 1 << ppy
}

// calculatePrecinctIndex calculates the precinct index for a code-block
// based on its position within the resolution level
// In JPEG 2000, precincts are defined at the resolution level, and all subbands
// at the same resolution share the same precinct partitioning
// cbX0, cbY0 are in the **resolution reference grid** (not global wavelet space)
func (e *Encoder) calculatePrecinctIndex(cbX0, cbY0, resolutionLevel int) int {
	// Get precinct dimensions for this resolution
	precinctWidth, precinctHeight := e.getPrecinctSize(resolutionLevel)

	// Get resolution dimensions
	resWidth, _ := e.getResolutionDimensions(resolutionLevel)

	// Calculate precinct grid position
	px := cbX0 / precinctWidth
	py := cbY0 / precinctHeight

	// Calculate number of precincts in X direction based on resolution dimensions
	numPrecinctX := (resWidth + precinctWidth - 1) / precinctWidth

	// Linear precinct index
	return py*numPrecinctX + px
}

// toResolutionCoordinates converts global wavelet coordinates to resolution reference grid coordinates
// For resolution 0 (LL subband), the coordinates are already in the correct space
// For resolution > 0, we need to map based on the subband type:
//
//	HL (band=1): coordinates are at offset (subbandWidth, 0)
//	LH (band=2): coordinates are at offset (0, subbandHeight)
//	HH (band=3): coordinates are at offset (subbandWidth, subbandHeight)
func (e *Encoder) toResolutionCoordinates(globalX, globalY, resolutionLevel, band int) (int, int) {
	if resolutionLevel == 0 {
		// LL subband - coordinates are already correct
		return globalX, globalY
	}

	// For resolution > 0, get subband dimensions
	subbandWidth, subbandHeight := e.getSubbandDimensions(resolutionLevel)

	// Map coordinates based on subband type
	// In the wavelet transform, subbands are laid out as:
	// +----+----+
	// | LL | HL |
	// +----+----+
	// | LH | HH |
	// +----+----+
	// So we need to subtract the subband offset to get resolution-local coordinates
	resX := globalX
	resY := globalY

	switch band {
	case 0: // LL (shouldn't happen for res > 0)
		// Already correct
	case 1: // HL (high-low) - right of LL
		resX = globalX - subbandWidth
	case 2: // LH (low-high) - below LL
		resY = globalY - subbandHeight
	case 3: // HH (high-high) - diagonal from LL
		resX = globalX - subbandWidth
		resY = globalY - subbandHeight
	}

	return resX, resY
}

// getSubbandDimensions returns the dimensions of a subband at a resolution level
func (e *Encoder) getSubbandDimensions(resolutionLevel int) (width, height int) {
	// For resolution level r:
	// - r=0: LL subband dimensions = image / (2^numLevels)
	// - r>0: HL/LH/HH subband dimensions = image / (2^(numLevels - r + 1))
	//
	// Simplified: All subbands at any resolution have the same calculation
	// subbandWidth = imageWidth >> (numLevels - resolutionLevel + 1) for res > 0
	// subbandWidth = imageWidth >> numLevels for res == 0

	if resolutionLevel == 0 {
		// LL subband
		width = ceilDivPow2(e.params.Width, e.params.NumLevels)
		height = ceilDivPow2(e.params.Height, e.params.NumLevels)
	} else {
		// HL/LH/HH subbands
		// These come from decomposition level (numLevels - resolutionLevel + 1)
		level := e.params.NumLevels - resolutionLevel + 1
		if level < 0 {
			level = 0
		}
		width = ceilDivPow2(e.params.Width, level)
		height = ceilDivPow2(e.params.Height, level)
	}

	// Ensure minimum size of 1
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return width, height
}

// getResolutionDimensions returns the dimensions of a resolution level
func (e *Encoder) getResolutionDimensions(resolutionLevel int) (width, height int) {
	// For resolution level r:
	// - r=0: LL subband (lowest resolution) = image / (2^numLevels)
	// - r=1: includes HL/LH/HH at decomposition level (numLevels-1) = image / (2^(numLevels-1))
	// - r=numLevels: highest resolution = full image
	//
	// Formula: width = imageWidth / (2^(numLevels - resolutionLevel))

	divisor := e.params.NumLevels - resolutionLevel
	if divisor < 0 {
		divisor = 0
	}

	width = ceilDivPow2(e.params.Width, divisor)
	height = ceilDivPow2(e.params.Height, divisor)

	// Ensure minimum size of 1
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return width, height
}

// ceilDivPow2 computes ceil(n / 2^pow) for pow >= 0.
func ceilDivPow2(n, pow int) int {
	if pow <= 0 {
		return n
	}
	divisor := 1 << pow
	return (n + divisor - 1) / divisor
}

// getPrecinctSizeExponents returns the precinct size exponents (PPx, PPy) for a given resolution level
// PPx and PPy are stored as log2 values (e.g., PPx=7 means width=2^7=128)
// ISO/IEC 15444-1 specifies that precinct sizes should be adjusted per resolution level
func (e *Encoder) getPrecinctSizeExponents(resolutionLevel int) (ppx, ppy uint8) {
	p := e.params

	// Default precinct size is 2^15 (32768) if not specified
	precinctWidth := p.PrecinctWidth
	precinctHeight := p.PrecinctHeight

	if precinctWidth == 0 {
		precinctWidth = 1 << 15 // 32768
	}
	if precinctHeight == 0 {
		precinctHeight = 1 << 15 // 32768
	}

	// Calculate exponents (log2)
	ppx = uint8(log2(precinctWidth))
	ppy = uint8(log2(precinctHeight))

	// For lower resolution levels, precincts should be smaller
	// Clamp to resolution size to avoid precincts larger than the resolution
	// At resolution level r, the image dimensions are divided by 2^(numLevels - r)
	if resolutionLevel < p.NumLevels {
		// Lower resolution levels - reduce precinct size
		maxPPx := uint8(15) // Max allowed by standard
		maxPPy := uint8(15)

		// Clamp to reasonable values
		if ppx > maxPPx {
			ppx = maxPPx
		}
		if ppy > maxPPy {
			ppy = maxPPy
		}
	}

	// PPx and PPy must be at least 0 (meaning 2^0 = 1 pixel minimum)
	// and at most 15 (meaning 2^15 = 32768 pixels maximum)
	if ppx > 15 {
		ppx = 15
	}
	if ppy > 15 {
		ppy = 15
	}

	return ppx, ppy
}

// writeQCD writes the QCD (Quantization Default) segment
func (e *Encoder) writeQCD(buf *bytes.Buffer) error {
	p := e.params

	qcdData := &bytes.Buffer{}

	if p.Lossless {
		// Lossless mode: no quantization (style 0)
		// Sqcd - bits 0-4: quantization type (0 = no quantization), bits 5-7: guard bits (2)
		// Match OpenJPEG: qntsty + (numgbits << 5) = 0 + (2 << 5) = 0x40
		sqcd := uint8(2<<5 | 0) // 2 guard bits in upper 3 bits, no quantization in lower 5 bits = 0x40
		_ = binary.Write(qcdData, binary.BigEndian, sqcd)

		// SPqcd - Quantization step size for each subband
		// For lossless: exponent only (8 bits), no mantissa
		// Exponent varies by subband due to DWT filter gains (5/3 reversible):
		//   LL band: bitDepth + 0
		//   HL, LH bands: bitDepth + 1
		//   HH band: bitDepth + 2
		// Values are shifted left by 3 bits when encoded
		for res := 0; res <= p.NumLevels; res++ {
			numBands := 1
			if res == 0 {
				numBands = 1 // LL band only for lowest resolution
			} else {
				numBands = 3 // HL, LH, HH for each resolution
			}
			for band := 0; band < numBands; band++ {
				var log2Gain int
				if res == 0 {
					log2Gain = 0 // LL band
				} else if band == 2 {
					log2Gain = 2 // HH band
				} else {
					log2Gain = 1 // HL, LH bands
				}
				expn := uint8((p.BitDepth + log2Gain) << 3)
				_ = binary.Write(qcdData, binary.BigEndian, expn)
			}
		}
	} else {
		// Lossy mode: scalar expounded quantization (style 2)
		encodedSteps := make([]uint16, 0)
		guardBits := uint8(2)

		if len(p.CustomQuantSteps) > 0 {
			encodedSteps = encodeQuantStepsFromFloats(p.CustomQuantSteps, p.BitDepth)
		} else {
			// Calculate quantization parameters based on quality
			quantParams := CalculateQuantizationParams(p.Quality, p.NumLevels, p.BitDepth)
			guardBits = uint8(quantParams.GuardBits)
			encodedSteps = quantParams.EncodedSteps
		}

		// Sqcd - bits 0-4: guard bits, bits 5-7: quantization type (2 = scalar expounded)
		sqcd := uint8(2<<5 | (guardBits & 0x1F))
		_ = binary.Write(qcdData, binary.BigEndian, sqcd)

		// SPqcd - Quantization step sizes for each subband
		// For scalar expounded: 16-bit value per subband (5-bit exponent, 11-bit mantissa)
		for _, encodedStep := range encodedSteps {
			_ = binary.Write(qcdData, binary.BigEndian, encodedStep)
		}
	}

	// Write marker and length
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerQCD))
	_ = binary.Write(buf, binary.BigEndian, uint16(qcdData.Len()+2))
	buf.Write(qcdData.Bytes())

	return nil
}

// writeRGN writes ROI (Region of Interest) marker segments (main header) if ROI is enabled.
// Baseline: MaxShift, one RGN per component, Crgn fits in 1 byte.
func (e *Encoder) writeRGN(buf *bytes.Buffer) error {
	if len(e.roiShifts) == 0 {
		return nil
	}

	for comp := 0; comp < e.params.Components; comp++ {
		shift := 0
		if comp < len(e.roiShifts) {
			shift = e.roiShifts[comp]
		}
		if shift <= 0 {
			continue
		}

		style := byte(0)
		if comp < len(e.roiStyles) {
			style = e.roiStyles[comp]
		}

		segment := &bytes.Buffer{}
		// Lrgn = 5 (length includes itself)
		_ = binary.Write(segment, binary.BigEndian, uint16(5))
		segment.WriteByte(byte(comp))  // Crgn
		segment.WriteByte(style)       // Srgn: 0 = MaxShift, 1 = General Scaling
		segment.WriteByte(byte(shift)) // SPrgn

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerRGN); err != nil {
			return err
		}
		if _, err := buf.Write(segment.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// writeCOM writes a COM (Comment) marker with private ROI metadata.
// This allows the decoder to reconstruct ROI geometry without external parameters.
// Format: Magic("JP2ROI") + Version(1) + ROI count + ROI geometries
func (e *Encoder) writeCOM(buf *bytes.Buffer) error {
	// Only write COM if we have ROI configuration
	if e.params.ROIConfig == nil || e.params.ROIConfig.IsEmpty() {
		return nil
	}

	data := &bytes.Buffer{}

	// Magic string: "JP2ROI" (6 bytes)
	data.WriteString("JP2ROI")

	// Version: 1 (1 byte)
	data.WriteByte(1)

	// Number of ROI regions (2 bytes)
	_ = binary.Write(data, binary.BigEndian, uint16(len(e.params.ROIConfig.ROIs)))

	// Encode each ROI region
	for _, roi := range e.params.ROIConfig.ROIs {
		// Shape type: 0=Rectangle, 1=Polygon, 2=Mask (1 byte)
		var shapeType byte
		if roi.Rect != nil {
			shapeType = 0
		} else if len(roi.Polygon) > 0 {
			shapeType = 1
		} else if roi.MaskData != nil {
			shapeType = 2
		}
		data.WriteByte(shapeType)

		// Number of components (1 byte)
		numComps := len(roi.Components)
		if numComps == 0 {
			numComps = e.params.Components // All components
		}
		data.WriteByte(byte(numComps))

		// Component indices
		if len(roi.Components) > 0 {
			for _, comp := range roi.Components {
				data.WriteByte(byte(comp))
			}
		} else {
			// All components
			for c := 0; c < e.params.Components; c++ {
				data.WriteByte(byte(c))
			}
		}

		// Geometry data based on shape type
		switch shapeType {
		case 0: // Rectangle
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.X0))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.Y0))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.X0+roi.Rect.Width))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.Rect.Y0+roi.Rect.Height))
		case 1: // Polygon
			_ = binary.Write(data, binary.BigEndian, uint16(len(roi.Polygon)))
			for _, pt := range roi.Polygon {
				_ = binary.Write(data, binary.BigEndian, uint32(pt.X))
				_ = binary.Write(data, binary.BigEndian, uint32(pt.Y))
			}
		case 2: // Mask (don't store raw mask, too large - store dimensions only as placeholder)
			_ = binary.Write(data, binary.BigEndian, uint32(roi.MaskWidth))
			_ = binary.Write(data, binary.BigEndian, uint32(roi.MaskHeight))
			// Note: Actual mask data not stored in COM (too large)
			// Decoder needs external mask or should use rectangle/polygon instead
		}
	}

	// Write COM marker
	if err := binary.Write(buf, binary.BigEndian, codestream.MarkerCOM); err != nil {
		return err
	}

	// Write length (2 bytes for length itself + 2 bytes for Rcom + data)
	length := uint16(2 + 2 + data.Len())
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return err
	}

	// Write Rcom (Registration value): 0x0001 for binary data (ISO/IEC 8859-15)
	// We use 0x0000 for private binary format
	if err := binary.Write(buf, binary.BigEndian, uint16(0x0000)); err != nil {
		return err
	}

	// Write data
	if _, err := buf.Write(data.Bytes()); err != nil {
		return err
	}

	return nil
}

// writeTileRGN writes ROI marker segments in tile-part header.
// This allows tile-specific ROI information (optional enhancement).
func (e *Encoder) writeTileRGN(buf *bytes.Buffer) error {
	// For now, write the same RGN as main header
	// In the future, this could support tile-specific ROI regions
	if len(e.roiShifts) == 0 {
		return nil
	}

	for comp := 0; comp < e.params.Components; comp++ {
		shift := 0
		if comp < len(e.roiShifts) {
			shift = e.roiShifts[comp]
		}
		if shift <= 0 {
			continue
		}

		style := byte(0)
		if comp < len(e.roiStyles) {
			style = e.roiStyles[comp]
		}

		segment := &bytes.Buffer{}
		// Lrgn = 5 (length includes itself)
		_ = binary.Write(segment, binary.BigEndian, uint16(5))
		segment.WriteByte(byte(comp))  // Crgn
		segment.WriteByte(style)       // Srgn: 0 = MaxShift, 1 = General Scaling
		segment.WriteByte(byte(shift)) // SPrgn

		if err := binary.Write(buf, binary.BigEndian, codestream.MarkerRGN); err != nil {
			return err
		}
		if _, err := buf.Write(segment.Bytes()); err != nil {
			return err
		}
	}

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

	// Encode tile data
	tileBytes := e.encodeTileData(transformedData, actualWidth, actualHeight)

	// Build tile-part header (e.g., RGN) to compute Psot correctly
	tileHeader := &bytes.Buffer{}
	if err := e.writeTileRGN(tileHeader); err != nil {
		return fmt.Errorf("failed to write tile-part RGN: %w", err)
	}

	// Write SOT (Start of Tile)
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSOT))
	_ = binary.Write(buf, binary.BigEndian, uint16(10)) // Lsot

	_ = binary.Write(buf, binary.BigEndian, uint16(tileIdx)) // Isot
	tilePartLength := len(tileBytes) + tileHeader.Len() + 14 // SOT(12) + header + SOD(2) + data
	_ = binary.Write(buf, binary.BigEndian, uint32(tilePartLength))
	_ = binary.Write(buf, binary.BigEndian, uint8(0)) // TPsot
	_ = binary.Write(buf, binary.BigEndian, uint8(1)) // TNsot

	// Write tile-part header (e.g., RGN)
	if _, err := buf.Write(tileHeader.Bytes()); err != nil {
		return err
	}

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
		if e.params.Components >= 3 && len(quantParams.StepSizes) > 0 {
			if e.params.Quality >= 95 {
				quantParams.StepSizes[0] = quantParams.StepSizes[0] * 0.6
			} else if e.params.Quality >= 85 {
				quantParams.StepSizes[0] = quantParams.StepSizes[0] * 0.8
			}
		}

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
	allBlocks := make([]*t2.PrecinctCodeBlock, 0)

	// Debug counters
	cbCountByRes := make([]int, e.params.NumLevels+1)

	// Process each component
	for comp := 0; comp < e.params.Components; comp++ {
		// Global code-block index across all resolutions
		globalCBIdx := 0

		// Process each resolution level
		// Resolution 0 = LL subband (lowest frequency)
		// Resolution 1+ = HL, LH, HH subbands
		for res := 0; res <= e.params.NumLevels; res++ {
			// Get subband dimensions for this resolution
			subbands := e.getSubbandsForResolution(tileData[comp], width, height, res)

			// Process each subband
			for _, subband := range subbands {
				// Partition subband into code-blocks
				codeBlocks := e.partitionIntoCodeBlocks(subband, comp)

				// Encode each code-block with T1
				for _, cb := range codeBlocks {
					encodedCB := e.encodeCodeBlock(cb, globalCBIdx)

					// Set the code-block index correctly
					encodedCB.Index = globalCBIdx
					// Calculate precinct index based on code-block position
					// Convert from global wavelet space to resolution reference grid
					resX0, resY0 := e.toResolutionCoordinates(encodedCB.X0, encodedCB.Y0, res, subband.band)
					precinctIdx := e.calculatePrecinctIndex(resX0, resY0, res)
					precinctWidth := e.params.PrecinctWidth
					precinctHeight := e.params.PrecinctHeight
					if precinctWidth == 0 {
						precinctWidth = 1 << 15
					}
					if precinctHeight == 0 {
						precinctHeight = 1 << 15
					}
					px := resX0 / precinctWidth
					py := resY0 / precinctHeight
					localX := resX0 - px*precinctWidth
					localY := resY0 - py*precinctHeight
					encodedCB.CBX = localX / e.params.CodeBlockWidth
					encodedCB.CBY = localY / e.params.CodeBlockHeight

					// Add to T2 packet encoder
					packetEnc.AddCodeBlock(comp, res, precinctIdx, encodedCB)
					allBlocks = append(allBlocks, encodedCB)
					globalCBIdx++

					// Count codeblocks per resolution
					cbCountByRes[res]++
				}
			}
		}
	}


	// Apply rate-distortion optimized allocation (PCRD) if layered or TargetRatio is requested.
	if e.params.NumLayers > 1 || e.params.TargetRatio > 0 {
		origBytes := e.params.Width * e.params.Height * e.params.Components * ((e.params.BitDepth + 7) / 8)
		if e.params.UsePCRDOpt && e.params.TargetRatio > 0 {
			midRefine := e.params.TargetRatio >= 7.5 && e.params.TargetRatio <= 8.5
			if !midRefine && e.params.TargetRatio <= 8.0 {
				e.applyRateDistortion(allBlocks, origBytes)
			} else {
				targetTotal := float64(origBytes) / e.params.TargetRatio
				// Estimate fixed codestream overhead (main header + per tile markers)
				fixed := float64(e.estimateFixedOverhead())
				if targetTotal > fixed {
					targetData := targetTotal - fixed
					// Iteratively refine allocation based on actual packet bytes
					budget := targetData
					maxIter := 6
					minScale := 0.5
					maxScale := 1.5
					if midRefine {
						maxIter = 12
						minScale = 0.8
						maxScale = 1.2
					}
					for iter := 0; iter < maxIter; iter++ {
						e.applyRateDistortionWithBudget(allBlocks, budget)
						packets, err := packetEnc.EncodePackets()
						if err != nil {
							break
						}
						pktBytes := 0
						for _, p := range packets {
							pktBytes += stuffedLen(p.Header) + stuffedLen(p.Body)
						}
						if pktBytes == 0 {
							break
						}
						errPct := math.Abs(float64(pktBytes)-targetData) / targetData
						if errPct <= 0.05 {
							break
						}
						scale := targetData / float64(pktBytes)
						if scale < minScale {
							scale = minScale
						}
						if scale > maxScale {
							scale = maxScale
						}
						budget = budget * scale
					}
				} else {
					e.applyRateDistortion(allBlocks, origBytes)
				}
			}
		} else {
			e.applyRateDistortion(allBlocks, origBytes)
		}
	}

	// Generate packets
	packets, err := packetEnc.EncodePackets()
	if err != nil {
		// Fallback to empty packet on error
		return []byte{0x00}
	}

	// Write packets to bitstream
	// OpenJPEG applies bit-stuffing only to packet headers (handled during header encoding).
	buf := &bytes.Buffer{}
	for _, packet := range packets {
		// Header already contains OpenJPEG-style bit stuffing.
		buf.Write(packet.Header)
		// Body is raw code-block data (no byte stuffing).
		buf.Write(packet.Body)
	}

	return buf.Bytes()
}

// estimateFixedOverhead builds main header segments to estimate constant bytes (excluding tile packet data).
func (e *Encoder) estimateFixedOverhead() int {
	buf := &bytes.Buffer{}
	// SOC
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerSOC))
	// SIZ, COD, QCD, RGN, COM
	_ = e.writeSIZ(buf)
	_ = e.writeCOD(buf)
	_ = e.writeQCD(buf)
	_ = e.writeRGN(buf)
	_ = e.writeCOM(buf)
	// Assume single tile overhead: SOT(12) + SOD(2) without data
	// We still include tile-part RGN if ROI present
	tile := &bytes.Buffer{}
	_ = binary.Write(tile, binary.BigEndian, uint16(codestream.MarkerSOT))
	_ = binary.Write(tile, binary.BigEndian, uint16(10))
	_ = binary.Write(tile, binary.BigEndian, uint16(0))
	_ = binary.Write(tile, binary.BigEndian, uint32(14))
	_ = binary.Write(tile, binary.BigEndian, uint8(0))
	_ = binary.Write(tile, binary.BigEndian, uint8(1))
	_ = e.writeTileRGN(tile)
	_ = binary.Write(tile, binary.BigEndian, uint16(codestream.MarkerSOD))
	// Append tile overhead
	buf.Write(tile.Bytes())
	// EOC
	_ = binary.Write(buf, binary.BigEndian, uint16(codestream.MarkerEOC))
	// Byte-stuffing applies to packet data; headers here are marker-coded, do not stuff
	return buf.Len()
}

// applyRateDistortionWithBudget performs allocation using a target total packet-data budget in bytes.
func (e *Encoder) applyRateDistortionWithBudget(blocks []*t2.PrecinctCodeBlock, targetBudget float64) {
	numLayers := e.params.NumLayers
	if numLayers <= 0 {
		numLayers = 1
	}
	appendLossless := e.params.AppendLosslessLayer && numLayers > 1
	if e.params.Lossless && numLayers > 1 {
		appendLossless = true
	}
	passesPerBlock := make([][]t1.PassData, 0, len(blocks))
	totalRate := 0.0
	for _, cb := range blocks {
		passesPerBlock = append(passesPerBlock, cb.Passes)
		if len(cb.Passes) > 0 {
			last := cb.Passes[len(cb.Passes)-1]
			bytes := last.ActualBytes
			if bytes == 0 {
				bytes = last.Rate
			}
			totalRate += float64(bytes)
		}
	}
	budget := targetBudget
	if budget <= 0 || budget > totalRate {
		budget = totalRate
	}
	var alloc *LayerAllocation
	if e.params.UsePCRDOpt && e.params.TargetRatio > 8.0 {
		layerBudgets := ComputeLayerBudgets(budget, numLayers, e.params.LayerBudgetStrategy)
		alloc = AllocateLayersWithLambda(passesPerBlock, numLayers, layerBudgets, e.params.LambdaTolerance)
	} else {
		alloc = AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, budget)
	}
	for idx, cb := range blocks {
		if len(cb.Passes) == 0 || cb.CompleteData == nil {
			continue
		}
		if len(cb.PassLengths) == 0 {
			cb.PassLengths = make([]int, len(cb.Passes))
			for i, p := range cb.Passes {
				cb.PassLengths[i] = p.ActualBytes
			}
		}
		cb.LayerPasses = make([]int, numLayers)
		cb.LayerData = make([][]byte, numLayers)
		prevEnd := 0
		for layer := 0; layer < numLayers; layer++ {
			passCount := alloc.GetPassesForLayer(idx, layer)
			if passCount > len(cb.Passes) {
				passCount = len(cb.Passes)
			}
			cb.LayerPasses[layer] = passCount
			end := prevEnd
			if passCount > 0 {
				end = cb.Passes[passCount-1].ActualBytes
				if end == 0 {
					end = cb.Passes[passCount-1].Rate
				}
			}
			if end < prevEnd {
				end = prevEnd
			}
			if end > len(cb.CompleteData) {
				end = len(cb.CompleteData)
			}
			cb.LayerData[layer] = cb.CompleteData[prevEnd:end]
			prevEnd = end
		}
		if appendLossless && len(cb.Passes) > 0 {
			last := numLayers - 1
			cb.LayerPasses[last] = len(cb.Passes)
			if prevEnd < 0 {
				prevEnd = 0
			}
			if prevEnd > len(cb.CompleteData) {
				prevEnd = len(cb.CompleteData)
			}
			cb.LayerData[last] = cb.CompleteData[prevEnd:]
		}
		cb.Data = cb.CompleteData
		cb.UseTERMALL = numLayers > 1 // Only use TERMALL for multi-layer
	}
}

// applyRateDistortion truncates/allocates passes across layers using PCRD-style allocation.
func (e *Encoder) applyRateDistortion(blocks []*t2.PrecinctCodeBlock, origBytes int) {
	numLayers := e.params.NumLayers
	if numLayers <= 0 {
		numLayers = 1
	}
	appendLossless := e.params.AppendLosslessLayer && numLayers > 1
	if e.params.Lossless && numLayers > 1 {
		appendLossless = true
	}
	if len(blocks) == 0 {
		return
	}

	passesPerBlock := make([][]t1.PassData, 0, len(blocks))
	totalRate := 0.0
	passBytes := func(passes []t1.PassData, count int) int {
		if count <= 0 {
			return 0
		}
		if count > len(passes) {
			count = len(passes)
		}
		b := passes[count-1].ActualBytes
		if b == 0 {
			b = passes[count-1].Rate
		}
		return b
	}

	for _, cb := range blocks {
		passesPerBlock = append(passesPerBlock, cb.Passes)
		if len(cb.Passes) > 0 {
			last := cb.Passes[len(cb.Passes)-1]
			bytes := last.ActualBytes
			if bytes == 0 {
				bytes = last.Rate
			}
			totalRate += float64(bytes)
		}
	}

	budget := totalRate
	if e.params.TargetRatio > 0 && origBytes > 0 {
		target := float64(origBytes) / e.params.TargetRatio
		if target < budget {
			budget = target
		}
	}

	var alloc *LayerAllocation
	if e.params.UsePCRDOpt {
		layerBudgets := ComputeLayerBudgets(budget, numLayers, e.params.LayerBudgetStrategy)
		alloc = AllocateLayersWithLambda(passesPerBlock, numLayers, layerBudgets, e.params.LambdaTolerance)
	} else {
		alloc = AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, budget)
	}

	// Enforce global budget proportionally when budget is less than full rate.
	if budget > 0 && budget < totalRate {
		for idx, passes := range passesPerBlock {
			fullBytes := passBytes(passes, len(passes))
			if fullBytes == 0 {
				continue
			}
			allowed := int(math.Floor(budget * float64(fullBytes) / totalRate))
			if allowed <= 0 && len(passes) > 0 {
				allowed = passBytes(passes, 1)
			}
			passCount := 0
			for i := 0; i < len(passes); i++ {
				if passBytes(passes, i+1) <= allowed {
					passCount = i + 1
				} else {
					break
				}
			}
			if passCount == 0 && len(passes) > 0 {
				passCount = 1
			}
			for layer := 0; layer < numLayers; layer++ {
				frac := float64(layer+1) / float64(numLayers)
				layerPass := int(math.Ceil(frac * float64(passCount)))
				if layerPass > passCount {
					layerPass = passCount
				}
				if layer > 0 && layerPass < alloc.CodeBlockPasses[idx][layer-1] {
					layerPass = alloc.CodeBlockPasses[idx][layer-1]
				}
				alloc.CodeBlockPasses[idx][layer] = layerPass
			}
		}
	}

	for idx, cb := range blocks {
		if len(cb.Passes) == 0 || cb.CompleteData == nil {
			// Fallback: keep existing data
			continue
		}

		if len(cb.PassLengths) == 0 {
			cb.PassLengths = make([]int, len(cb.Passes))
			for i, p := range cb.Passes {
				cb.PassLengths[i] = p.ActualBytes
			}
		}

		cb.LayerPasses = make([]int, numLayers)
		cb.LayerData = make([][]byte, numLayers)

		prevEnd := 0
		for layer := 0; layer < numLayers; layer++ {
			passCount := alloc.GetPassesForLayer(idx, layer)
			if passCount > len(cb.Passes) {
				passCount = len(cb.Passes)
			}
			cb.LayerPasses[layer] = passCount

			end := prevEnd
			if passCount > 0 {
				end = cb.Passes[passCount-1].ActualBytes
				if end == 0 {
					end = cb.Passes[passCount-1].Rate
				}
			}

			if end < prevEnd {
				end = prevEnd
			}
			if end > len(cb.CompleteData) {
				end = len(cb.CompleteData)
			}

			cb.LayerData[layer] = cb.CompleteData[prevEnd:end]
			prevEnd = end
		}

		if appendLossless && len(cb.Passes) > 0 {
			last := numLayers - 1
			cb.LayerPasses[last] = len(cb.Passes)
			if prevEnd < 0 {
				prevEnd = 0
			}
			if prevEnd > len(cb.CompleteData) {
				prevEnd = len(cb.CompleteData)
			}
			cb.LayerData[last] = cb.CompleteData[prevEnd:]
		}

		// Keep full data for compatibility
		cb.Data = cb.CompleteData
		cb.UseTERMALL = numLayers > 1 // Only use TERMALL for multi-layer
	}
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

func stuffedLen(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := len(data)
	for _, b := range data {
		if b == 0xFF {
			n++
		}
	}
	return n
}

// subbandInfo represents a wavelet subband
type subbandInfo struct {
	data   []int32 // Coefficient data
	x0, y0 int     // Subband origin
	width  int     // Subband width
	height int     // Subband height
	band   int     // Band type: 0=LL, 1=HL, 2=LH, 3=HH
	res    int     // Resolution level (0=LL)
	scale  int     // Scale factor to map subband coords to full resolution (power of two)
}

// getSubbandsForResolution extracts subbands for a specific resolution level
func (e *Encoder) getSubbandsForResolution(data []int32, width, height, resolution int) []subbandInfo {
	// Resolution 0 contains only LL subband (approximation)
	// Resolution r > 0 contains HL, LH, HH subbands from decomposition level r

	var subbands []subbandInfo

	if resolution == 0 {
		// LL subband (top-left quadrant after all decompositions)
		// Use ceiling division to match JPEG2000 standard and OpenJPEG
		divisor := 1 << e.params.NumLevels
		llWidth := (width + divisor - 1) / divisor   // Ceiling division
		llHeight := (height + divisor - 1) / divisor // Ceiling division
		scale := divisor

		llData := make([]int32, llWidth*llHeight)
		for y := 0; y < llHeight; y++ {
			for x := 0; x < llWidth; x++ {
				srcIdx := y*width + x
				if srcIdx < len(data) && y < height && x < width {
					llData[y*llWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}

		sb := subbandInfo{
			data:   llData,
			x0:     0,
			y0:     0,
			width:  llWidth,
			height: llHeight,
			band:   0, // LL
			res:    0,
			scale:  scale,
		}
		subbands = append(subbands, sb)
	} else {
		// For resolution r, extract HL, LH, HH subbands
		// OpenJPEG uses: levelDiff = numResolutions - 1 - resno
		// Since numResolutions = NumLevels + 1:
		// levelDiff = (NumLevels + 1) - 1 - resolution = NumLevels - resolution
		level := e.params.NumLevels - resolution
		if level < 0 {
			level = 0
		}

		// CRITICAL FIX: OpenJPEG uses (level_no + 1) for HL/LH/HH subband dimensions
		// See tcd.c: opj_int64_ceildivpow2(..., (OPJ_INT32)(l_level_no + 1))
		// This means we divide by 2^(level+1) instead of 2^level
		divisor := 1 << (level + 1)
		sbWidth := (width + divisor - 1) / divisor   // Ceiling division
		sbHeight := (height + divisor - 1) / divisor // Ceiling division
		scale := divisor

		// HL (high-low): right half of top half
		hlData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				srcIdx := y*width + (sbWidth + x)
				if srcIdx < len(data) && sbWidth+x < width {
					hlData[y*sbWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hlData,
			x0:     sbWidth,
			y0:     0,
			width:  sbWidth,
			height: sbHeight,
			band:   1, // HL
			res:    resolution,
			scale:  scale,
		})

		// LH (low-high): left half of bottom half
		lhData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				srcIdx := (sbHeight+y)*width + x
				if srcIdx < len(data) && sbHeight+y < height {
					lhData[y*sbWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   lhData,
			x0:     0,
			y0:     sbHeight,
			width:  sbWidth,
			height: sbHeight,
			band:   2, // LH
			res:    resolution,
			scale:  scale,
		})

		// HH (high-high): right half of bottom half
		hhData := make([]int32, sbWidth*sbHeight)
		for y := 0; y < sbHeight; y++ {
			for x := 0; x < sbWidth; x++ {
				srcIdx := (sbHeight+y)*width + (sbWidth + x)
				if srcIdx < len(data) && sbHeight+y < height && sbWidth+x < width {
					hhData[y*sbWidth+x] = data[srcIdx]
				}
				// else: pad with zero (already initialized to 0)
			}
		}
		subbands = append(subbands, subbandInfo{
			data:   hhData,
			x0:     sbWidth,
			y0:     sbHeight,
			width:  sbWidth,
			height: sbHeight,
			band:   3, // HH
			res:    resolution,
			scale:  scale,
		})
	}

	return subbands
}

type codeBlockInfo struct {
	compIdx  int
	data     []int32
	width    int
	height   int
	globalX0 int // Global X position in coefficient array
	globalY0 int // Global Y position in coefficient array
	cbx      int // Code-block X index within subband
	cby      int // Code-block Y index within subband
	scale    int // Downsampling factor from full resolution (reserved)
	resLevel int // Resolution level (0=LL)
	band     int // Subband identifier (0=LL,1=HL,2=LH,3=HH)
	mask     [][]bool
}

// partitionIntoCodeBlocks partitions a subband into code-blocks
func (e *Encoder) partitionIntoCodeBlocks(subband subbandInfo, compIdx int) []codeBlockInfo {
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

			var mask [][]bool
			if compIdx < len(e.roiMasks) && e.roiMasks[compIdx] != nil {
				// Use subband.scale to map subband coords to full resolution
				step := max(1, subband.scale)
				fullX0 := (subband.x0 + x0) * step
				fullY0 := (subband.y0 + y0) * step
				fullX1 := (subband.x0 + x1) * step
				fullY1 := (subband.y0 + y1) * step
				mask = e.roiMasks[compIdx].downsample(fullX0, fullY0, fullX1, fullY1, step)
			}

			codeBlocks = append(codeBlocks, codeBlockInfo{
				compIdx:  compIdx,
				data:     cbData,
				width:    actualWidth,
				height:   actualHeight,
				globalX0: globalX0,
				globalY0: globalY0,
				cbx:      cbx,
				cby:      cby,
				scale:    subband.scale,
				resLevel: subband.res,
				band:     subband.band,
				mask:     mask,
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

	// Apply T1_NMSEDEC_FRACBITS scaling (left shift 6 bits)
	// This matches OpenJPEG's representation for lossless encoding
	// OpenJPEG applies this shift in t1.c before T1 encoding
	const T1_NMSEDEC_FRACBITS = 6
	for i := range cbData {
		cbData[i] <<= T1_NMSEDEC_FRACBITS
	}

	// Calculate max bitplane from scaled data
	rawMaxBitplane := calculateMaxBitplane(cbData)

	// Adjust maxBitplane by adding 1 then subtracting T1_NMSEDEC_FRACBITS
	// OpenJPEG does: numbps = (floorlog2(max) + 1) - T1_NMSEDEC_FRACBITS
	// The +1 is critical - it converts from bit position to number of bits
	numbps := rawMaxBitplane
	if numbps >= 0 {
		numbps = (numbps + 1) - T1_NMSEDEC_FRACBITS
	}

	// Calculate number of coding passes
	// OpenJPEG sequencing: first pass is cleanup on the top bit-plane,
	// then 3 passes per remaining bit-plane.
	numPasses := 1
	if numbps > 0 {
		numPasses = (numbps * 3) - 2
	}
	if numbps < 0 {
		// All zeros - still need at least 1 pass for valid packet header
		numPasses = 1
	}

	// Calculate zero bit-planes
	// ZeroBitPlanes = number of MSB bit-planes that are all zero
	// Formula: effectiveBitDepth - 1 - rawMaxBitplane
	// IMPORTANT: Use rawMaxBitplane (before T1_NMSEDEC_FRACBITS adjustment) because
	// effectiveBitDepth already includes T1_NMSEDEC_FRACBITS
	// Note: After wavelet transform, coefficients may need extra bits
	// 5/3 reversible wavelet adds 1 bit per decomposition level
	effectiveBitDepth := e.params.BitDepth + e.params.NumLevels + T1_NMSEDEC_FRACBITS

	zeroBitPlanes := 0
	if rawMaxBitplane < 0 {
		// All data is zero, all bit-planes are zero
		zeroBitPlanes = effectiveBitDepth
	} else {
		zeroBitPlanes = effectiveBitDepth - 1 - rawMaxBitplane
	}

	// Create block encoder (EBCOT T1 or HTJ2K)
	var blockEnc BlockEncoder
	if e.params.BlockEncoderFactory != nil {
		blockEnc = e.params.BlockEncoderFactory(actualWidth, actualHeight)
	} else {
		t1Enc := t1.NewT1Encoder(actualWidth, actualHeight, 0)
		t1Enc.SetOrientation(cb.band)
		blockEnc = t1Enc
	}

	// ROI handling: determine style/shift/inside and apply scaling/roishift
	style, roiShift, inside := e.roiContext(cb)
	roishift := 0
	if roiShift > 0 {
		if style == 1 {
			// General Scaling: scale ROI blocks before coding, background unchanged
			if inside {
				if cb.mask != nil && len(cb.mask) > 0 && len(cb.mask[0]) > 0 {
					applyGeneralScalingMasked(cbData, cb.mask, roiShift)
				} else {
					applyGeneralScaling(cbData, roiShift)
				}
			}
		} else {
			// MaxShift: shift background blocks
			if !inside {
				roishift = roiShift
			}
		}
	}

	// Create PrecinctCodeBlock structure
	pcb := &t2.PrecinctCodeBlock{
		Index:          0, // Will be set by caller if needed
		X0:             cb.globalX0,
		Y0:             cb.globalY0,
		X1:             cb.globalX0 + actualWidth,
		Y1:             cb.globalY0 + actualHeight,
		CBX:            cb.cbx,
		CBY:            cb.cby,
		Band:           cb.band,
		Included:       false, // First inclusion in packet
		NumPassesTotal: numPasses,
		ZeroBitPlanes:  zeroBitPlanes,
	}

	useLayered := e.params.NumLayers > 1 || e.params.TargetRatio > 0

	if useLayered {
		// Force TERMALL style termination so each pass boundary is byte-aligned for PCRD.
		layerBoundaries := []int{numPasses}
		if e.params.NumLayers > 1 {
			layerAlloc := AllocateLayersSimple(numPasses, e.params.NumLayers, 1)
			layerBoundaries = make([]int, e.params.NumLayers)
			for layer := 0; layer < e.params.NumLayers; layer++ {
				layerBoundaries[layer] = layerAlloc.GetPassesForLayer(0, layer)
			}
		} else {
			// Two boundaries are enough to force termination for all passes.
			layerBoundaries = []int{1, numPasses}
		}

		// Try EncodeLayered if available (T1Encoder), otherwise fallback to simple Encode
		var passes []t1.PassData
		var completeData []byte
		var err error

		if t1Enc, ok := blockEnc.(*t1.T1Encoder); ok {
			// Calculate code-block style flags (match writeCOD logic)
			cblksty := uint8(0)
			if e.params.NumLayers > 1 || e.params.TargetRatio > 0 {
				cblksty |= 0x04
			}
			// Use layered encoding for T1Encoder
			passes, completeData, err = t1Enc.EncodeLayered(cbData, numPasses, roishift, layerBoundaries, cblksty)
		} else {
			// Fallback to simple encoding for HTJ2K
			completeData, err = blockEnc.Encode(cbData, numPasses, roishift)
			// Create single pass data
			if err == nil {
				passes = []t1.PassData{{ActualBytes: len(completeData)}}
			}
		}

		if err != nil || len(passes) == 0 {
			encodedData := []byte{0x00}
			pcb.Data = encodedData
			pcb.LayerData = [][]byte{encodedData}
			pcb.LayerPasses = []int{1}
			return pcb
		}

		// Store per-pass metadata for later PCRD allocation.
		pcb.PassLengths = make([]int, len(passes))
		for i, pass := range passes {
			pcb.PassLengths[i] = pass.ActualBytes
		}
		pcb.Passes = passes
		pcb.CompleteData = completeData
		pcb.Data = completeData
		pcb.UseTERMALL = e.params.NumLayers > 1 || e.params.TargetRatio > 0

		return pcb
	} else {
		// Single layer: use block encoder
		encodedData, err := blockEnc.Encode(cbData, numPasses, roishift)
		if err != nil {
			// Return minimal code-block on error
			encodedData = []byte{0x00}
			numPasses = 1
			zeroBitPlanes = effectiveBitDepth
			pcb.NumPassesTotal = numPasses
			pcb.ZeroBitPlanes = zeroBitPlanes
		}
		pcb.Data = encodedData
	}

	return pcb
}

// roiShiftForCodeBlock returns the MaxShift value for the given code-block.
// For MaxShift: shift is applied to background blocks (non-ROI).
// For General Scaling: shift is reported for ROI blocks (background 0) but caller may apply scaling separately.
func (e *Encoder) roiShiftForCodeBlock(cb codeBlockInfo) int {
	_, shift, inside := e.roiContext(cb)
	style := byte(0)
	if cb.compIdx >= 0 && cb.compIdx < len(e.roiStyles) {
		style = e.roiStyles[cb.compIdx]
	}
	if style == 1 && cb.mask != nil {
		// General Scaling uses explicit coefficient scaling; no roishift when mask is present.
		return 0
	}
	return shiftIfApplicable(e.roiStyles, cb.compIdx, shift, inside)
}

// roiContext returns ROI style, shift, and whether the block intersects ROI.
func (e *Encoder) roiContext(cb codeBlockInfo) (byte, int, bool) {
	if cb.compIdx < 0 || cb.compIdx >= len(e.roiShifts) {
		return 0, 0, false
	}
	style := byte(0)
	if cb.compIdx < len(e.roiStyles) {
		style = e.roiStyles[cb.compIdx]
	}
	shift := e.roiShifts[cb.compIdx]
	if shift <= 0 {
		return style, 0, false
	}
	x0 := cb.globalX0
	y0 := cb.globalY0
	x1 := cb.globalX0 + cb.width
	y1 := cb.globalY0 + cb.height

	inside := false
	hasMask := cb.mask != nil && len(cb.mask) > 0 && len(cb.mask[0]) > 0
	if hasMask {
		inside = maskAnyTrue(cb.mask)
	} else {
		rects := e.roiRects[cb.compIdx]
		for _, rect := range rects {
			if rect.intersects(x0, y0, x1, y1) {
				inside = true
				break
			}
		}
	}
	return style, shift, inside
}

func shiftIfApplicable(styles []byte, compIdx, shift int, inside bool) int {
	style := byte(0)
	if compIdx >= 0 && compIdx < len(styles) {
		style = styles[compIdx]
	}
	if shift <= 0 {
		return 0
	}
	if style == 1 {
		// General Scaling uses explicit coefficient scaling; roishift not used.
		return 0
	}
	if inside {
		return 0
	}
	return shift
}

// applyGeneralScaling multiplies coefficients in-place by 2^shift.
func applyGeneralScaling(data []int32, shift int) {
	if shift <= 0 {
		return
	}
	factor := int32(1 << shift)
	for i := range data {
		data[i] *= factor
	}
}

// applyGeneralScalingMasked multiplies only coefficients covered by mask by 2^shift.
func applyGeneralScalingMasked(data []int32, mask [][]bool, shift int) {
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
					data[idx] *= factor
				}
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// encodeQuantStepsFromFloats converts floating quantization steps to encoded form (5-bit exponent, 11-bit mantissa).
func encodeQuantStepsFromFloats(steps []float64, bitDepth int) []uint16 {
	if len(steps) == 0 {
		return nil
	}
	encoded := make([]uint16, len(steps))
	bias := bitDepth - 1
	for i, stepSize := range steps {
		if stepSize <= 0 {
			stepSize = 0.0001
		}
		exp := int(math.Floor(math.Log2(stepSize))) + bias
		mantissa := stepSize / math.Pow(2.0, float64(exp-bias))
		if exp < 0 {
			exp = 0
		}
		if exp > 31 {
			exp = 31
		}
		mantissaInt := int((mantissa - 1.0) * 2048.0)
		if mantissaInt < 0 {
			mantissaInt = 0
		}
		if mantissaInt > 2047 {
			mantissaInt = 2047
		}
		encoded[i] = uint16((exp << 11) | mantissaInt)
	}
	return encoded
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
