package jpeg2000

import "fmt"

// ROIStyle defines how a ROI is coded.
type ROIStyle int

const (
	// ROIStyleMaxShift uses MaxShift (background bit-plane shift).
	ROIStyleMaxShift ROIStyle = iota
	// ROIStyleGeneralScaling follows General Scaling (not yet implemented).
	ROIStyleGeneralScaling
)

// ROIShape describes the geometry of a ROI.
type ROIShape int

const (
	// ROIShapeRectangle is the only shape currently supported.
	ROIShapeRectangle ROIShape = iota
	// ROIShapePolygon is planned (not yet implemented).
	ROIShapePolygon
	// ROIShapeMask accepts an external mask/bitmap (not yet implemented).
	ROIShapeMask
)

// roiRect is an internal axis-aligned rectangle helper.
type roiRect struct {
	x0, y0 int
	x1, y1 int
}

// ROIRegion describes one ROI entry. Only rectangular MaxShift is supported for now.
type ROIRegion struct {
	ID string

	Style ROIStyle
	Shape ROIShape

	// Rectangular ROI data (required for rectangle shape).
	Rect *ROIParams

	// Shift overrides Rect.Shift for MaxShift if >0.
	Shift int

	// Scale is an alias for General Scaling k (same semantics as Shift).
	// If >0 it overrides Shift.
	Scale int

	// Polygon vertices (optional). If provided, polygon area is added to mask.
	Polygon []Point

	// Mask bitmap (optional). If provided, must match image size.
	MaskWidth  int
	MaskHeight int
	MaskData   []bool // row-major MaskWidth*MaskHeight

	// Components limits ROI to certain component indices (empty = all).
	Components []int
}

// ROIConfig groups multiple ROI definitions and defaults.
type ROIConfig struct {
	ROIs []ROIRegion

	// Defaults used when individual ROI fields are not set.
	DefaultStyle ROIStyle
	DefaultShift int
}

// IsEmpty reports whether the config has no ROI entries.
func (cfg *ROIConfig) IsEmpty() bool {
	return cfg == nil || len(cfg.ROIs) == 0
}

// Validate ensures current MVP constraints: rectangle + MaxShift/GeneralScaling only, with valid geometry and shift.
func (cfg *ROIConfig) Validate(imgWidth, imgHeight int) error {
	if cfg == nil || len(cfg.ROIs) == 0 {
		return nil
	}

	for i := range cfg.ROIs {
		roi := &cfg.ROIs[i]

		hasPolygon := len(roi.Polygon) >= 3
		hasMask := len(roi.MaskData) > 0 && roi.MaskWidth > 0 && roi.MaskHeight > 0

		style := roi.Style
		if style == ROIStyle(0) {
			style = cfg.DefaultStyle
		}
		if style == ROIStyle(0) {
			style = ROIStyleMaxShift
		}
		if style != ROIStyleMaxShift && style != ROIStyleGeneralScaling {
			return fmt.Errorf("ROI[%d]: style %v not supported (must be MaxShift or GeneralScaling)", i, style)
		}

		shape := roi.Shape
		if shape == ROIShape(0) {
			shape = ROIShapeRectangle
		}
		if shape != ROIShapeRectangle {
			return fmt.Errorf("ROI[%d]: shape %v not supported (only rectangle MVP)", i, shape)
		}

		rect := roi.Rect
		if rect == nil && hasPolygon {
			b := boundingRect(roi.Polygon)
			rect = &ROIParams{X0: b.x0, Y0: b.y0, Width: b.x1 - b.x0, Height: b.y1 - b.y0}
		} else if rect == nil && hasMask {
			if b, ok := boundingRectFromMask(roi.MaskWidth, roi.MaskHeight, roi.MaskData); ok {
				rect = &ROIParams{X0: b.x0, Y0: b.y0, Width: b.x1 - b.x0, Height: b.y1 - b.y0}
			} else {
				rect = &ROIParams{X0: 0, Y0: 0, Width: imgWidth, Height: imgHeight}
			}
		}
		if rect == nil && !hasPolygon && !hasMask {
			return fmt.Errorf("ROI[%d]: rectangle or polygon/mask required for shape rectangle", i)
		}

		shift := roi.Shift
		if roi.Scale > 0 {
			shift = roi.Scale
		}
		if shift <= 0 && rect != nil {
			shift = rect.Shift
		}
		if shift <= 0 {
			shift = cfg.DefaultShift
		}

		if rect != nil && shift > 0 {
			rect = &ROIParams{
				X0:     rect.X0,
				Y0:     rect.Y0,
				Width:  rect.Width,
				Height: rect.Height,
				Shift:  shift,
			}
			roi.Rect = rect
		}

		if rect != nil && !rect.IsValid(imgWidth, imgHeight) {
			return fmt.Errorf("ROI[%d]: invalid rectangle %+v", i, roi.Rect)
		}

		if shift <= 0 && !hasMask {
			return fmt.Errorf("ROI[%d]: missing MaxShift value", i)
		}
		if shift > 255 {
			return fmt.Errorf("ROI[%d]: shift %d exceeds 255", i, shift)
		}
	}

	return nil
}

// ResolveRectangles returns Srgn style, per-component MaxShift/Scaling values, and rectangle lists.
// MVP: supports Srgn 0 (MaxShift) or 1 (General Scaling) with rectangular geometry.
func (cfg *ROIConfig) ResolveRectangles(imgWidth, imgHeight, components int) (byte, []int, [][]roiRect, error) {
	if cfg == nil || len(cfg.ROIs) == 0 {
		return 0, nil, nil, nil
	}

	if err := cfg.Validate(imgWidth, imgHeight); err != nil {
		return 0, nil, nil, err
	}

	if components <= 0 {
		return 0, nil, nil, fmt.Errorf("invalid component count: %d", components)
	}

	shifts := make([]int, components)
	rectsByComp := make([][]roiRect, components)
	srgn := byte(0) // default MaxShift
	styleSet := false

	for i := range cfg.ROIs {
		roi := cfg.ROIs[i]
		hasPolygon := len(roi.Polygon) >= 3
		hasMask := len(roi.MaskData) > 0 && roi.MaskWidth > 0 && roi.MaskHeight > 0

		style := roi.Style
		if style == ROIStyle(0) {
			style = cfg.DefaultStyle
		}
		if style == ROIStyle(0) {
			style = ROIStyleMaxShift
		}
		if style != ROIStyleMaxShift && style != ROIStyleGeneralScaling {
			return 0, nil, nil, fmt.Errorf("ROI[%d]: unsupported style %v", i, style)
		}

		if hasMask {
			style = ROIStyleMaxShift
		}

		if !styleSet {
			srgn = byte(style)
			styleSet = true
		} else if srgn != byte(style) {
			return 0, nil, nil, fmt.Errorf("ROI[%d]: mixed ROI styles not supported (got %d vs %d)", i, style, srgn)
		}

		roiShift := roi.Shift
		if roi.Scale > 0 {
			roiShift = roi.Scale
		}
		if roiShift <= 0 && roi.Rect != nil {
			roiShift = roi.Rect.Shift
		}
		if roiShift <= 0 {
			roiShift = cfg.DefaultShift
		}
		if hasMask {
			roiShift = 0
		}
		if roiShift <= 0 && !hasMask {
			return 0, nil, nil, fmt.Errorf("ROI[%d]: missing ROI shift/scaling value after defaults", i)
		}
		if roiShift > 255 {
			return 0, nil, nil, fmt.Errorf("ROI[%d]: shift %d exceeds 255", i, roiShift)
		}

		rect := roi.Rect
		if rect == nil && hasPolygon {
			b := boundingRect(roi.Polygon)
			rect = &ROIParams{X0: b.x0, Y0: b.y0, Width: b.x1 - b.x0, Height: b.y1 - b.y0}
		} else if rect == nil && hasMask {
			if b, ok := boundingRectFromMask(roi.MaskWidth, roi.MaskHeight, roi.MaskData); ok {
				rect = &ROIParams{X0: b.x0, Y0: b.y0, Width: b.x1 - b.x0, Height: b.y1 - b.y0}
			} else {
				rect = &ROIParams{X0: 0, Y0: 0, Width: imgWidth, Height: imgHeight}
			}
		}
		if rect == nil {
			return 0, nil, nil, fmt.Errorf("ROI[%d]: missing ROI geometry", i)
		}

		targetComponents := roi.Components
		if len(targetComponents) == 0 {
			targetComponents = make([]int, components)
			for c := 0; c < components; c++ {
				targetComponents[c] = c
			}
		}

		for _, comp := range targetComponents {
			if comp < 0 || comp >= components {
				return 0, nil, nil, fmt.Errorf("ROI[%d]: component index %d out of range", i, comp)
			}
			if shifts[comp] != 0 && shifts[comp] != roiShift {
				return 0, nil, nil, fmt.Errorf("ROI[%d]: mixed shifts for component %d (%d vs %d)", i, comp, shifts[comp], roiShift)
			}
			shifts[comp] = roiShift
			rectsByComp[comp] = append(rectsByComp[comp], roiRect{
				x0: roi.Rect.X0,
				y0: roi.Rect.Y0,
				x1: roi.Rect.X0 + roi.Rect.Width,
				y1: roi.Rect.Y0 + roi.Rect.Height,
			})
		}
	}

	return srgn, shifts, rectsByComp, nil
}

func (r roiRect) intersects(x0, y0, x1, y1 int) bool {
	return r.x0 < x1 && x0 < r.x1 && r.y0 < y1 && y0 < r.y1
}
