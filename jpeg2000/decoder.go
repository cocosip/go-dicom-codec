package jpeg2000

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/colorspace"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
)

// Decoder implements JPEG 2000 decoding
type Decoder struct {
	// Codestream
	cs *codestream.Codestream

	// Custom block decoder factory (for HTJ2K support)
	blockDecoderFactory t2.BlockDecoderFactory

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

	// Custom MCT inverse (experimental, parsed from COM)
	mctInverse [][]float64
	// Optional per-component offsets
	mctOffsets []int32
	// MCO precision flags
	mctReversible bool
	bindings      []mctBinding
}

type mctBinding struct {
	compIDs    []int
	matrixF    [][]float64
	matrixI    [][]int32
	offsets    []int32
	normScale  float64
	reversible bool
	rounding   uint8
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

// SetBlockDecoderFactory sets a custom block decoder factory (e.g., for HTJ2K support)
func (d *Decoder) SetBlockDecoderFactory(factory t2.BlockDecoderFactory) {
	d.blockDecoderFactory = factory
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
	d.extractMCTFromMarkers()
	d.extractBindings()

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

// extractMCTFromMarkers parses a custom MCT inverse matrix from MCT marker (experimental)
func (d *Decoder) extractMCTFromMarkers() {
	if d.cs == nil {
		return
	}
	var normScale float64
	if len(d.cs.MCC) > 0 {
		seg := d.cs.MCC[0]
		applyOrder := make([]uint8, len(seg.MCTIndices))
		copy(applyOrder, seg.MCTIndices)
		// Parse MCO options for overrides
		if len(d.cs.MCO) > 0 {
			for _, o := range d.cs.MCO {
				if o.Index != 0 || len(o.Options) == 0 {
					continue
				}
				off := 0
				for off < len(o.Options) {
					t := o.Options[off]
					off++
					switch t {
					case codestream.MCOOptNormScale:
						if off+4 <= len(o.Options) {
							v := uint32(o.Options[off])<<24 | uint32(o.Options[off+1])<<16 | uint32(o.Options[off+2])<<8 | uint32(o.Options[off+3])
							normScale = float64(math.Float32frombits(v))
							off += 4
						} else {
							off = len(o.Options)
						}
					case codestream.MCOOptRecordOrder:
						if off < len(o.Options) {
							count := int(o.Options[off])
							off++
							if off+count <= len(o.Options) {
								applyOrder = make([]uint8, count)
								copy(applyOrder, o.Options[off:off+count])
								off += count
							} else {
								off = len(o.Options)
							}
						}
					default:
						off = len(o.Options)
					}
				}
			}
		}
		// AssocType semantics
		if seg.AssocType == codestream.MCCAssocMatrixThenOffset || seg.AssocType == codestream.MCCAssocOffsetThenMatrix {
			var mats, offs []uint8
			for _, id := range applyOrder {
				for _, m := range d.cs.MCT {
					if m.Index == id {
						if int(m.ArrayType) == 1 {
							mats = append(mats, id)
						} else if int(m.ArrayType) == 2 {
							offs = append(offs, id)
						}
						break
					}
				}
			}
			if seg.AssocType == codestream.MCCAssocMatrixThenOffset {
				applyOrder = append(mats, offs...)
			} else {
				applyOrder = append(offs, mats...)
			}
		}
		for _, idx := range applyOrder {
			for _, m := range d.cs.MCT {
				if m.Index == idx {
					if int(m.ArrayType) == 1 && m.Rows > 0 && m.Cols > 0 {
						rows := int(m.Rows)
						cols := int(m.Cols)
						data := m.Data
						inv := make([][]float64, rows)
						off := 0
						need := rows * cols * 4
						if len(data) >= need {
							for r := 0; r < rows; r++ {
								inv[r] = make([]float64, cols)
								for c := 0; c < cols; c++ {
									if int(m.ElementType) == 1 {
										v := uint32(data[off])<<24 | uint32(data[off+1])<<16 | uint32(data[off+2])<<8 | uint32(data[off+3])
										inv[r][c] = float64(math.Float32frombits(v))
									} else {
										v := int32(uint32(data[off])<<24 | uint32(data[off+1])<<16 | uint32(data[off+2])<<8 | uint32(data[off+3]))
										inv[r][c] = float64(v)
									}
									off += 4
								}
							}
							if normScale != 0 {
								for r := 0; r < rows; r++ {
									for c := 0; c < cols; c++ {
										inv[r][c] /= normScale
									}
								}
							}
							d.mctInverse = inv
						}
					}
					if int(m.ElementType) == 0 && int(m.ArrayType) == 2 && m.Rows > 0 && m.Cols > 0 {
						rows := int(m.Rows)
						cols := int(m.Cols)
						if cols == 1 && rows == d.components {
							data := m.Data
							need := rows * 4
							if len(data) >= need {
								offs := make([]int32, rows)
								off := 0
								for r := 0; r < rows; r++ {
									v := int32(uint32(data[off])<<24 | uint32(data[off+1])<<16 | uint32(data[off+2])<<8 | uint32(data[off+3]))
									off += 4
									offs[r] = v
								}
								d.mctOffsets = offs
							}
						}
					}
				}
			}
		}
		if d.mctInverse != nil || d.mctOffsets != nil {
			return
		}
	}
	if len(d.cs.MCT) > 0 {
		for _, seg := range d.cs.MCT {
			if int(seg.ElementType) == 1 && int(seg.ArrayType) == 1 && seg.Rows > 0 && seg.Cols > 0 {
				rows := int(seg.Rows)
				cols := int(seg.Cols)
				data := seg.Data
				need := rows * cols * 4
				if len(data) >= need {
					inv := make([][]float64, rows)
					off := 0
					for r := 0; r < rows; r++ {
						inv[r] = make([]float64, cols)
						for c := 0; c < cols; c++ {
							v := uint32(data[off])<<24 | uint32(data[off+1])<<16 | uint32(data[off+2])<<8 | uint32(data[off+3])
							off += 4
							inv[r][c] = float64(math.Float32frombits(v))
						}
					}
					d.mctInverse = inv
					break
				}
			}
		}
		if len(d.cs.MCO) > 0 {
			for _, o := range d.cs.MCO {
				if o.Index != 0 || len(o.Options) == 0 {
					continue
				}
				off := 0
				for off < len(o.Options) {
					t := o.Options[off]
					off++
					switch t {
					case codestream.MCOOptNormScale:
						if off+4 <= len(o.Options) {
							v := uint32(o.Options[off])<<24 | uint32(o.Options[off+1])<<16 | uint32(o.Options[off+2])<<8 | uint32(o.Options[off+3])
							normScale = float64(math.Float32frombits(v))
							off += 4
						} else {
							off = len(o.Options)
						}
					case codestream.MCOOptPrecision:
						if off < len(o.Options) {
							d.mctReversible = (o.Options[off] & 0x1) != 0
							off++
						} else {
							off = len(o.Options)
						}
					case codestream.MCOOptRecordOrder:
						// already applied by MCC order and AssocType semantics in this branch
						if off < len(o.Options) {
							count := int(o.Options[off])
							off++
							off += count
						}
					default:
						off = len(o.Options)
					}
				}
			}
			if normScale != 0 && d.mctInverse != nil {
				rows := len(d.mctInverse)
				cols := len(d.mctInverse[0])
				for r := 0; r < rows; r++ {
					for c := 0; c < cols; c++ {
						d.mctInverse[r][c] /= normScale
					}
				}
			}
		}
		if d.mctInverse != nil {
			return
		}
	}
	// Fallback to COM-based payload
	if len(d.cs.COM) > 0 {
		for _, com := range d.cs.COM {
			if len(com.Data) < 7 {
				continue
			}
			if string(com.Data[0:6]) != "JP2MCT" {
				continue
			}
			version := com.Data[6]
			if version != 1 {
				continue
			}
			if len(com.Data) < 11 {
				continue
			}
			rows := int(com.Data[7])<<8 | int(com.Data[8])
			cols := int(com.Data[9])<<8 | int(com.Data[10])
			offset := 11
			if offset >= len(com.Data) {
				continue
			}
			// reversible flag
			offset++
			need := rows * cols * 4
			if offset+need > len(com.Data) {
				continue
			}
			inv := make([][]float64, rows)
			for r := 0; r < rows; r++ {
				inv[r] = make([]float64, cols)
				for c := 0; c < cols; c++ {
					v := uint32(com.Data[offset])<<24 | uint32(com.Data[offset+1])<<16 | uint32(com.Data[offset+2])<<8 | uint32(com.Data[offset+3])
					offset += 4
					inv[r][c] = float64(math.Float32frombits(v))
				}
			}
			d.mctInverse = inv
			return
		}
	}
}

func (d *Decoder) extractBindings() {
	if d.cs == nil {
		return
	}
	for _, seg := range d.cs.MCC {
		compIDs := make([]int, len(seg.ComponentIDs))
		for i := range seg.ComponentIDs {
			compIDs[i] = int(seg.ComponentIDs[i])
		}
		var order []uint8
		order = append(order, seg.MCTIndices...)
		var norm float64
		var prec uint8
		for _, o := range d.cs.MCO {
			if o.Index != seg.Index || len(o.Options) == 0 {
				continue
			}
			off := 0
			for off < len(o.Options) {
				t := o.Options[off]
				off++
				switch t {
				case codestream.MCOOptNormScale:
					if off+4 <= len(o.Options) {
						v := uint32(o.Options[off])<<24 | uint32(o.Options[off+1])<<16 | uint32(o.Options[off+2])<<8 | uint32(o.Options[off+3])
						norm = float64(math.Float32frombits(v))
						off += 4
					} else {
						off = len(o.Options)
					}
				case codestream.MCOOptPrecision:
					if off < len(o.Options) {
						prec = o.Options[off]
						off++
					} else {
						off = len(o.Options)
					}
				case codestream.MCOOptRecordOrder:
					if off < len(o.Options) {
						cnt := int(o.Options[off])
						off++
						if off+cnt <= len(o.Options) {
							order = make([]uint8, cnt)
							copy(order, o.Options[off:off+cnt])
							off += cnt
						} else {
							off = len(o.Options)
						}
					}
				default:
					off = len(o.Options)
				}
			}
		}
		if seg.AssocType == codestream.MCCAssocMatrixThenOffset || seg.AssocType == codestream.MCCAssocOffsetThenMatrix {
			var mats, offs []uint8
			for _, id := range order {
				for _, m := range d.cs.MCT {
					if m.Index == id {
						if int(m.ArrayType) == 1 {
							mats = append(mats, id)
						} else if int(m.ArrayType) == 2 {
							offs = append(offs, id)
						}
						break
					}
				}
			}
			if seg.AssocType == codestream.MCCAssocMatrixThenOffset {
				order = append(mats, offs...)
			} else {
				order = append(offs, mats...)
			}
		}
		var matF [][]float64
		var matI [][]int32
		var offsVals []int32
		for _, id := range order {
			for _, m := range d.cs.MCT {
				if m.Index != id {
					continue
				}
				if int(m.ArrayType) == 1 && m.Rows > 0 && m.Cols > 0 {
					r := int(m.Rows)
					c := int(m.Cols)
					if int(m.ElementType) == 1 {
						need := r * c * 4
						if len(m.Data) >= need {
							matF = make([][]float64, r)
							off := 0
							for i := 0; i < r; i++ {
								matF[i] = make([]float64, c)
								for j := 0; j < c; j++ {
									v := uint32(m.Data[off])<<24 | uint32(m.Data[off+1])<<16 | uint32(m.Data[off+2])<<8 | uint32(m.Data[off+3])
									matF[i][j] = float64(math.Float32frombits(v))
									off += 4
								}
							}
						}
					} else {
						need := r * c * 4
						if len(m.Data) >= need {
							matI = make([][]int32, r)
							off := 0
							for i := 0; i < r; i++ {
								matI[i] = make([]int32, c)
								for j := 0; j < c; j++ {
									v := int32(uint32(m.Data[off])<<24 | uint32(m.Data[off+1])<<16 | uint32(m.Data[off+2])<<8 | uint32(m.Data[off+3]))
									matI[i][j] = v
									off += 4
								}
							}
						}
					}
				} else if int(m.ElementType) == 0 && int(m.ArrayType) == 2 && m.Rows > 0 && m.Cols > 0 {
					r := int(m.Rows)
					c := int(m.Cols)
					if c == 1 && r == len(compIDs) {
						need := r * 4
						if len(m.Data) >= need {
							offsVals = make([]int32, r)
							off := 0
							for i := 0; i < r; i++ {
								v := int32(uint32(m.Data[off])<<24 | uint32(m.Data[off+1])<<16 | uint32(m.Data[off+2])<<8 | uint32(m.Data[off+3]))
								offsVals[i] = v
								off += 4
							}
						}
					}
				}
			}
		}
		b := mctBinding{compIDs: compIDs, matrixF: matF, matrixI: matI, offsets: offsVals, normScale: norm, reversible: (prec & codestream.MCOPrecisionReversibleFlag) != 0, rounding: prec & codestream.MCOPrecisionRoundingMask}
		d.bindings = append(d.bindings, b)
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
		// Check if HTJ2K mode is enabled (bit 6 of Scod, 0x40)
		isHTJ2K := (d.cs.COD.Scod & 0x40) != 0
		// Use injected blockDecoderFactory if HTJ2K is enabled
		var blockDecoderFactory t2.BlockDecoderFactory
		if isHTJ2K && d.blockDecoderFactory != nil {
			blockDecoderFactory = d.blockDecoderFactory
		}
		tileDecoder := t2.NewTileDecoder(tile, d.cs.SIZ, d.cs.COD, d.cs.QCD, roiInfo, isHTJ2K, blockDecoderFactory)

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

	d.data = assembler.GetImageData()
	if len(d.bindings) > 0 {
		width := d.width
		height := d.height
		n := width * height
		for _, b := range d.bindings {
			if len(b.compIDs) == 0 {
				continue
			}
			m := b.matrixF
			useInt := false
			if b.reversible && b.normScale == 0 {
				b.normScale = 1
			}
			if b.reversible && b.normScale == 1 && b.matrixI != nil && len(b.matrixI) == len(b.compIDs) {
				useInt = true
			}
			if useInt {
				r := len(b.matrixI)
				c := len(b.matrixI[0])
				for i := 0; i < n; i++ {
					out := make([]int32, r)
					for rr := 0; rr < r; rr++ {
						var sum int64
						for kk := 0; kk < c; kk++ {
							sum += int64(b.matrixI[rr][kk]) * int64(d.data[b.compIDs[kk]][i])
						}
						out[rr] = int32(sum)
					}
					for rr := 0; rr < r; rr++ {
						d.data[b.compIDs[rr]][i] = out[rr]
					}
				}
			} else if m != nil && len(m) == len(b.compIDs) {
				r := len(m)
				c := len(m[0])
				for i := 0; i < n; i++ {
					out := make([]int32, r)
					for rr := 0; rr < r; rr++ {
						sum := 0.0
						for kk := 0; kk < c; kk++ {
							sum += m[rr][kk] * float64(d.data[b.compIDs[kk]][i])
						}
						switch b.rounding {
						case codestream.MCOPrecisionRoundFloor:
							out[rr] = int32(math.Floor(sum))
						case codestream.MCOPrecisionRoundCeil:
							out[rr] = int32(math.Ceil(sum))
						case codestream.MCOPrecisionRoundTrunc:
							if sum >= 0 {
								out[rr] = int32(math.Floor(sum))
							} else {
								out[rr] = int32(math.Ceil(sum))
							}
						default:
							out[rr] = int32(math.Round(sum))
						}
					}
					for rr := 0; rr < r; rr++ {
						d.data[b.compIDs[rr]][i] = out[rr]
					}
				}
			}
			if b.offsets != nil && len(b.offsets) == len(b.compIDs) {
				for idx, cid := range b.compIDs {
					off := b.offsets[idx]
					if off != 0 {
						for i := 0; i < n; i++ {
							d.data[cid][i] += off
						}
					}
				}
			}
		}
	} else if d.mctInverse != nil && len(d.mctInverse) == d.components {
		// Apply inverse custom MCT
		width := d.width
		height := d.height
		n := width * height
		comps := d.components
		out := make([][]int32, comps)
		for c := 0; c < comps; c++ {
			out[c] = make([]int32, n)
		}
		for i := 0; i < n; i++ {
			for r := 0; r < comps; r++ {
				sum := 0.0
				for k := 0; k < comps; k++ {
					sum += d.mctInverse[r][k] * float64(d.data[k][i])
				}
				out[r][i] = int32(math.Round(sum))
			}
		}
		d.data = out
		if d.mctOffsets != nil && len(d.mctOffsets) == d.components {
			for c := 0; c < comps; c++ {
				off := d.mctOffsets[c]
				if off != 0 {
					for i := 0; i < n; i++ {
						d.data[c][i] += off
					}
				}
			}
		}
	} else if d.cs != nil && d.cs.COD != nil && d.components == 3 {
		if d.cs.COD.MultipleComponentTransform == 1 {
			if d.cs.COD.Transformation == 1 {
				r, g, b := colorspace.ApplyInverseRCTToComponents(d.data[0], d.data[1], d.data[2])
				d.data[0], d.data[1], d.data[2] = r, g, b
			} else {
				r, g, b := colorspace.ConvertComponentsYCbCrToRGB(d.data[0], d.data[1], d.data[2])
				d.data[0], d.data[1], d.data[2] = r, g, b
			}
		}
	}

	// DEBUG: Print first few values before inverse DC level shift
	if len(d.data) > 0 && len(d.data[0]) > 0 {
		fmt.Printf("DEBUG decoder: Before inverse DC shift, first 10 values: ")
		for i := 0; i < 10 && i < len(d.data[0]); i++ {
			fmt.Printf("%d ", d.data[0][i])
		}
		fmt.Printf("\n")
		fmt.Printf("DEBUG decoder: BitDepth=%d, IsSigned=%v\n", d.bitDepth, d.isSigned)
	}

	// Apply inverse DC level shift for unsigned data
	d.applyInverseDCLevelShift()

	// DEBUG: Print first few values after inverse DC level shift
	if len(d.data) > 0 && len(d.data[0]) > 0 {
		fmt.Printf("DEBUG decoder: After inverse DC shift, first 10 values: ")
		for i := 0; i < 10 && i < len(d.data[0]); i++ {
			fmt.Printf("%d ", d.data[0][i])
		}
		fmt.Printf("\n")
	}

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
