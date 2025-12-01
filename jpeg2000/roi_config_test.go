package jpeg2000

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
)

func TestROIConfigResolveSingleRect(t *testing.T) {
	cfg := &ROIConfig{
		DefaultShift: 2,
		ROIs: []ROIRegion{
			{
				Rect: &ROIParams{X0: 1, Y0: 1, Width: 4, Height: 4},
			},
		},
	}

	shifts, rects, err := cfg.ResolveMaxShiftRectangles(8, 8, 1)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(shifts) != 1 || shifts[0] != 2 {
		t.Fatalf("unexpected shifts: %+v", shifts)
	}
	if len(rects) != 1 || len(rects[0]) != 1 {
		t.Fatalf("expected 1 rect, got %v", rects)
	}
	r := rects[0][0]
	if r.x0 != 1 || r.y0 != 1 || r.x1 != 5 || r.y1 != 5 {
		t.Fatalf("unexpected rect: %+v", r)
	}
}

func TestROIConfigMixedShiftError(t *testing.T) {
	cfg := &ROIConfig{
		ROIs: []ROIRegion{
			{Rect: &ROIParams{X0: 0, Y0: 0, Width: 4, Height: 4}, Shift: 2},
			{Rect: &ROIParams{X0: 4, Y0: 4, Width: 4, Height: 4}, Shift: 3},
		},
	}

	if _, _, err := cfg.ResolveMaxShiftRectangles(8, 8, 1); err == nil {
		t.Fatalf("expected mixed shift error, got nil")
	}
}

func TestEncodeWithROIConfigMultiRect(t *testing.T) {
	width, height := 16, 16
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.ROI = nil
	params.ROIConfig = &ROIConfig{
		ROIs: []ROIRegion{
			{Rect: &ROIParams{X0: 0, Y0: 0, Width: 4, Height: 4}, Shift: 3},
			{Rect: &ROIParams{X0: 8, Y0: 8, Width: 4, Height: 4}, Shift: 3},
		},
	}

	numPixels := width * height
	componentData := [][]int32{make([]int32, numPixels)}
	for i := 0; i < numPixels; i++ {
		componentData[0][i] = int32(i % 256)
	}

	enc := NewEncoder(params)
	stream, err := enc.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	parser := codestream.NewParser(stream)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse codestream failed: %v", err)
	}

	if len(cs.RGN) != params.Components {
		t.Fatalf("expected %d RGN segments, got %d", params.Components, len(cs.RGN))
	}
	if cs.RGN[0].Srgn != 0 || int(cs.RGN[0].SPrgn) != 3 {
		t.Fatalf("unexpected RGN values: %+v", cs.RGN[0])
	}

	dec := NewDecoder()
	dec.SetROIConfig(params.ROIConfig)
	if err := dec.Decode(stream); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if dec.Width() != width || dec.Height() != height {
		t.Fatalf("decoded dimensions mismatch: got %dx%d", dec.Width(), dec.Height())
	}
}

func TestROIConfigComponentFilter(t *testing.T) {
	cfg := &ROIConfig{
		DefaultShift: 2,
		ROIs: []ROIRegion{
			{
				Rect:       &ROIParams{X0: 0, Y0: 0, Width: 4, Height: 4},
				Components: []int{1},
			},
		},
	}

	shifts, rects, err := cfg.ResolveMaxShiftRectangles(8, 8, 3)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(shifts) != 3 {
		t.Fatalf("unexpected shifts len: %d", len(shifts))
	}
	if shifts[0] != 0 || shifts[1] != 2 || shifts[2] != 0 {
		t.Fatalf("unexpected shifts: %+v", shifts)
	}
	if len(rects) != 3 || len(rects[1]) != 1 {
		t.Fatalf("unexpected rects: %+v", rects)
	}
}
