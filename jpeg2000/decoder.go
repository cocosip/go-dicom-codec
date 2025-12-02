package jpeg2000

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
)

// Decoder implements JPEG 2000 decoding
type Decoder struct {
	// Codestream
	cs *codestream.Codestream
	// ROI
	roi       *ROIParams
	roiConfig *ROIConfig
	roiShifts []int
	roiRects  [][]roiRect // per-component rectangles
	roiSrgn   []byte      // per-component ROI style (Srgn)
	roiMasks  []*roiMask  // per-component ROI mask

	// Decoded image data
	width      int
	height     int
	components int
	bitDepth   int
	isSigned   bool

	// Decoded pixel data per component
	data [][]int32
}

// NewDecoder creates a new JPEG 2000 decoder
func NewDecoder() *Decoder {
	return &Decoder{}
}

// SetROI sets the ROI rectangle for decoding (required if ROI is used in the codestream).
func (d *Decoder) SetROI(roi *ROIParams) {
	d.roi = roi
}

// SetROIConfig sets ROI configuration (MVP: multiple rectangles, MaxShift).
func (d *Decoder) SetROIConfig(cfg *ROIConfig) {
	d.roiConfig = cfg
}

// Decode decodes a JPEG 2000 codestream
func (d *Decoder) Decode(data []byte) error {
	// Parse codestream
	parser := codestream.NewParser(data)
	cs, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse codestream: %w", err)
	}

	d.cs = cs

	// Extract image parameters
	if err := d.extractImageParameters(); err != nil {
		return fmt.Errorf("failed to extract image parameters: %w", err)
	}

	// Capture ROI shift values from RGN segments
	d.captureROIShifts()

	// Extract ROI geometry from COM marker (if present)
	d.extractROIFromCOM()

	// Resolve ROI geometry (legacy ROI or ROIConfig)
	if err := d.resolveROI(); err != nil {
		return fmt.Errorf("invalid ROI configuration: %w", err)
	}

	// Decode all tiles
	if err := d.decodeTiles(); err != nil {
		return fmt.Errorf("failed to decode tiles: %w", err)
	}

	return nil
}

// extractImageParameters extracts image parameters from SIZ segment
func (d *Decoder) extractImageParameters() error {
	if d.cs.SIZ == nil {
		return fmt.Errorf("missing SIZ segment")
	}

	siz := d.cs.SIZ

	d.width = int(siz.Xsiz - siz.XOsiz)
	d.height = int(siz.Ysiz - siz.YOsiz)
	d.components = int(siz.Csiz)

	if d.components == 0 {
		return fmt.Errorf("invalid number of components: %d", d.components)
	}

	// Use first component's parameters
	d.bitDepth = siz.Components[0].BitDepth()
	d.isSigned = siz.Components[0].IsSigned()

	return nil
}

// captureROIShifts builds per-component ROI shift table from RGN segments.
// If no RGN is present, shifts remain zero.
func (d *Decoder) captureROIShifts() {
	if d.cs == nil || d.cs.SIZ == nil {
		return
	}
	d.roiShifts = make([]int, d.components)
	d.roiSrgn = make([]byte, d.components)
	for _, rgn := range d.cs.RGN {
		if int(rgn.Crgn) < len(d.roiShifts) {
			d.roiShifts[int(rgn.Crgn)] = int(rgn.SPrgn)
			d.roiSrgn[int(rgn.Crgn)] = rgn.Srgn
		}
	}
}

// extractROIFromCOM extracts ROI geometry from COM marker (private metadata).
// This allows automatic ROI reconstruction without external parameters.
func (d *Decoder) extractROIFromCOM() {
	if d.cs == nil || len(d.cs.COM) == 0 {
		return
	}

	// If user already provided ROIConfig, don't override
	if d.roiConfig != nil && !d.roiConfig.IsEmpty() {
		return
	}

	// Search for our private ROI COM marker
	for _, com := range d.cs.COM {
		// Check for magic string "JP2ROI"
		if len(com.Data) < 7 {
			continue
		}
		if string(com.Data[0:6]) != "JP2ROI" {
			continue
		}

		// Parse version
		version := com.Data[6]
		if version != 1 {
			continue // Unknown version
		}

		// Parse ROI configuration
		cfg, err := parseROIFromCOMData(com.Data[7:])
		if err != nil {
			// Invalid format, skip
			continue
		}

		// Use this ROI configuration
		d.roiConfig = cfg
		return
	}
}

// resolveROI normalizes ROI inputs (legacy ROI or ROIConfig) into internal rectangles.
func (d *Decoder) resolveROI() error {
	d.roiRects = nil

	// ROIConfig takes priority when present
	if d.roiConfig != nil && !d.roiConfig.IsEmpty() {
		if err := d.roiConfig.Validate(d.width, d.height); err != nil {
			return err
		}
		srgn, shifts, rects, err := d.roiConfig.ResolveRectangles(d.width, d.height, d.components)
		if err != nil {
			return err
		}
		d.roiShifts = shifts
		d.roiRects = rects
		d.roiMasks = buildMasksFromConfig(d.width, d.height, d.components, rects, d.roiConfig)
		if len(shifts) > 0 {
			d.roiSrgn = make([]byte, len(shifts))
			for i := range d.roiSrgn {
				d.roiSrgn[i] = srgn
			}
		}
		return nil
	}

	// Legacy single-rectangle ROI
	if d.roi != nil {
		if !d.roi.IsValid(d.width, d.height) {
			return fmt.Errorf("invalid ROI parameters for decoded image: %+v", *d.roi)
		}
		d.roiShifts = make([]int, d.components)
		d.roiSrgn = make([]byte, d.components)
		d.roiRects = make([][]roiRect, d.components)
		d.roiMasks = make([]*roiMask, d.components)
		for c := 0; c < d.components; c++ {
			d.roiShifts[c] = d.roi.Shift
			d.roiSrgn[c] = 0
			d.roiRects[c] = []roiRect{{
				x0: d.roi.X0,
				y0: d.roi.Y0,
				x1: d.roi.X0 + d.roi.Width,
				y1: d.roi.Y0 + d.roi.Height,
			}}
			d.roiMasks[c] = newROIMask(d.width, d.height)
			d.roiMasks[c].setRect(d.roi.X0, d.roi.Y0, d.roi.X0+d.roi.Width, d.roi.Y0+d.roi.Height)
		}
	}

	return nil
}

// decodeTiles decodes all tiles in the codestream
func (d *Decoder) decodeTiles() error {
	if len(d.cs.Tiles) == 0 {
		return fmt.Errorf("no tiles found in codestream")
	}

	// Create tile assembler
	assembler := NewTileAssembler(d.cs.SIZ)

	// Build ROI info for tile decoders
	var roiInfo *t2.ROIInfo
	if len(d.roiRects) > 0 && len(d.roiShifts) == d.components {
		rectsByComp := make([][]t2.ROIRect, len(d.roiRects))
		for comp := range d.roiRects {
			rects := d.roiRects[comp]
			rectsByComp[comp] = make([]t2.ROIRect, len(rects))
			for i, r := range rects {
				rectsByComp[comp][i] = t2.ROIRect{
					X0: r.x0,
					Y0: r.y0,
					X1: r.x1,
					Y1: r.y1,
				}
			}
		}
		roiInfo = &t2.ROIInfo{
			RectsByComponent: rectsByComp,
			Shifts:           d.roiShifts,
			Styles:           d.roiSrgn,
		}
		roiInfo.Masks = make([]*t2.ROIMask, len(d.roiMasks))
		for i := range d.roiMasks {
			if d.roiMasks[i] != nil {
				roiInfo.Masks[i] = &t2.ROIMask{
					Width:  d.roiMasks[i].width,
					Height: d.roiMasks[i].height,
					Data:   d.roiMasks[i].data,
				}
			}
		}
	}

	// Decode all tiles
	for tileIdx, tile := range d.cs.Tiles {
		// Create tile decoder
		tileDecoder := t2.NewTileDecoder(tile, d.cs.SIZ, d.cs.COD, d.cs.QCD, roiInfo)

		// Decode tile
		tileData, err := tileDecoder.Decode()
		if err != nil {
			return fmt.Errorf("failed to decode tile %d: %w", tileIdx, err)
		}

		// Assemble tile into image
		err = assembler.AssembleTile(tileIdx, tileData)
		if err != nil {
			return fmt.Errorf("failed to assemble tile %d: %w", tileIdx, err)
		}
	}

	// Get assembled image data
	d.data = assembler.GetImageData()

	// Note: Inverse DC level shift is already applied in tile decoder
	// Do not apply it again here to avoid double shifting

	return nil
}

// GetImageData returns the decoded image data for all components
func (d *Decoder) GetImageData() [][]int32 {
	return d.data
}

// GetComponentData returns the decoded data for a specific component
func (d *Decoder) GetComponentData(componentIdx int) ([]int32, error) {
	if componentIdx < 0 || componentIdx >= len(d.data) {
		return nil, fmt.Errorf("invalid component index: %d", componentIdx)
	}
	return d.data[componentIdx], nil
}

// Width returns the image width
func (d *Decoder) Width() int {
	return d.width
}

// Height returns the image height
func (d *Decoder) Height() int {
	return d.height
}

// Components returns the number of components
func (d *Decoder) Components() int {
	return d.components
}

// BitDepth returns the bit depth
func (d *Decoder) BitDepth() int {
	return d.bitDepth
}

// IsSigned returns whether the data is signed
func (d *Decoder) IsSigned() bool {
	return d.isSigned
}

// GetPixelData returns interleaved pixel data in a byte array
// Suitable for use with the Codec interface
func (d *Decoder) GetPixelData() []byte {
	if d.components == 1 {
		// Grayscale
		return d.getGrayscalePixelData()
	}
	// RGB or multi-component
	return d.getInterleavedPixelData()
}

// getGrayscalePixelData returns grayscale pixel data
func (d *Decoder) getGrayscalePixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit
		result := make([]byte, numPixels)
		for i := 0; i < numPixels; i++ {
			val := d.data[0][i]
			if val < 0 {
				val = 0
			} else if val > 255 {
				val = 255
			}
			result[i] = byte(val)
		}
		return result
	}

	// 16-bit (or 12-bit stored as 16-bit)
	result := make([]byte, numPixels*2)
	for i := 0; i < numPixels; i++ {
		val := d.data[0][i]
		if val < 0 {
			val = 0
		}
		maxVal := (1 << d.bitDepth) - 1
		if val > int32(maxVal) {
			val = int32(maxVal)
		}
		// Little-endian
		result[i*2] = byte(val)
		result[i*2+1] = byte(val >> 8)
	}
	return result
}

// getInterleavedPixelData returns interleaved RGB/multi-component pixel data
func (d *Decoder) getInterleavedPixelData() []byte {
	numPixels := d.width * d.height

	if d.bitDepth <= 8 {
		// 8-bit per component
		result := make([]byte, numPixels*d.components)
		for i := 0; i < numPixels; i++ {
			for c := 0; c < d.components; c++ {
				val := d.data[c][i]
				if val < 0 {
					val = 0
				} else if val > 255 {
					val = 255
				}
				result[i*d.components+c] = byte(val)
			}
		}
		return result
	}

	// 16-bit per component
	result := make([]byte, numPixels*d.components*2)
	for i := 0; i < numPixels; i++ {
		for c := 0; c < d.components; c++ {
			val := d.data[c][i]
			if val < 0 {
				val = 0
			}
			maxVal := (1 << d.bitDepth) - 1
			if val > int32(maxVal) {
				val = int32(maxVal)
			}
			idx := (i*d.components + c) * 2
			result[idx] = byte(val)
			result[idx+1] = byte(val >> 8)
		}
	}
	return result
}

// applyInverseDCLevelShift applies inverse DC level shift for unsigned data
// For unsigned data: add 2^(bitDepth-1) to convert back from signed range
func (d *Decoder) applyInverseDCLevelShift() {
	if d.isSigned {
		// Signed data - no level shift needed
		return
	}

	// Unsigned data - add 2^(bitDepth-1)
	shift := int32(1 << (d.bitDepth - 1))

	for c := 0; c < d.components; c++ {
		for i := 0; i < len(d.data[c]); i++ {
			d.data[c][i] += shift
		}
	}
}

// parseROIFromCOMData parses ROI configuration from COM marker data.
func parseROIFromCOMData(data []byte) (*ROIConfig, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("COM data too short")
	}

	// Read number of ROI regions (2 bytes)
	numRegions := int(data[0])<<8 | int(data[1])
	offset := 2

	cfg := &ROIConfig{
		ROIs: make([]ROIRegion, 0, numRegions),
	}

	for i := 0; i < numRegions; i++ {
		if offset >= len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}

		// Read shape type (1 byte)
		shapeType := data[offset]
		offset++

		// Read number of components (1 byte)
		if offset >= len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}
		numComps := int(data[offset])
		offset++

		// Read component indices
		if offset+numComps > len(data) {
			return nil, fmt.Errorf("unexpected end of COM data")
		}
		comps := make([]int, numComps)
		for j := 0; j < numComps; j++ {
			comps[j] = int(data[offset])
			offset++
		}

		roi := ROIRegion{
			Components: comps,
		}

		// Parse geometry based on shape type
		switch shapeType {
		case 0: // Rectangle
			if offset+16 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			x0 := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			y0 := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
			x1 := int(data[offset+8])<<24 | int(data[offset+9])<<16 | int(data[offset+10])<<8 | int(data[offset+11])
			y1 := int(data[offset+12])<<24 | int(data[offset+13])<<16 | int(data[offset+14])<<8 | int(data[offset+15])
			offset += 16
			roi.Rect = &ROIParams{X0: x0, Y0: y0, Width: x1 - x0, Height: y1 - y0}
			roi.Shape = ROIShapeRectangle

		case 1: // Polygon
			if offset+2 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			numPoints := int(data[offset])<<8 | int(data[offset+1])
			offset += 2
			if offset+numPoints*8 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			points := make([]Point, numPoints)
			for j := 0; j < numPoints; j++ {
				x := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
				y := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
				points[j] = Point{X: x, Y: y}
				offset += 8
			}
			roi.Polygon = points
			roi.Shape = ROIShapePolygon

		case 2: // Mask (placeholder only - actual mask data not stored)
			if offset+8 > len(data) {
				return nil, fmt.Errorf("unexpected end of COM data")
			}
			width := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			height := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])
			offset += 8
			// Create empty mask data structure (decoder still needs external mask)
			roi.MaskWidth = width
			roi.MaskHeight = height
			roi.Shape = ROIShapeMask
			// Note: MaskData not populated from COM (too large to store)

		default:
			return nil, fmt.Errorf("unknown shape type: %d", shapeType)
		}

		cfg.ROIs = append(cfg.ROIs, roi)
	}

	return cfg, nil
}
