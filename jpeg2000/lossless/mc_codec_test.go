package lossless

import (
    "testing"
    "github.com/cocosip/go-dicom-codec/jpeg2000"
    "github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func TestLosslessCodecWithMCTBindingsWritesMarkers(t *testing.T) {
    w, h, comps := 8, 8, 2
    n := w*h
    src := make([]byte, n*comps)
    for i := 0; i < n; i++ { src[2*i] = byte(i%256); src[2*i+1] = byte((i*3)%256) }
    pdIn := &codec.PixelData{ Data: src, Width: uint16(w), Height: uint16(h), BitsAllocated: 8, BitsStored: 8, HighBit: 7, SamplesPerPixel: uint16(comps) }
    pdOut := &codec.PixelData{}
    params := NewLosslessParameters()
    b := jpeg2000.MCTBindingParams{ AssocType: 2, ComponentIDs: []uint16{0,1}, Matrix: [][]float64{{1,0},{0,1}}, Inverse: [][]float64{{1,0},{0,1}}, Offsets: []int32{5,-5}, ElementType: 1, MCOPrecision: 1 }
    params.SetParameter("mctBindings", []jpeg2000.MCTBindingParams{b})
    c := NewCodec()
    if err := c.Encode(pdIn, pdOut, params); err != nil { t.Fatalf("encode failed: %v", err) }
    cs, err := codestream.NewParser(pdOut.Data).Parse()
    if err != nil { t.Fatalf("parse failed: %v", err) }
    if len(cs.MCT) == 0 || len(cs.MCC) == 0 { t.Fatalf("expected MCT/MCC present") }
}
