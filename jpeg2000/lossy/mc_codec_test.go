package lossy

import (
    "testing"
    "github.com/cocosip/go-dicom-codec/jpeg2000"
    "github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
    "github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
    codecHelpers "github.com/cocosip/go-dicom-codec/codec"
)

func TestLossyCodecWithMCTBindingsWritesMarkers(t *testing.T) {
    w, h, comps := 8, 8, 3
    n := w*h
    src := make([]byte, n*comps)
    for i := 0; i < n; i++ { src[3*i] = byte(i%256); src[3*i+1] = byte((i*3)%256); src[3*i+2] = byte((i*7)%256) }

    frameInfo := &imagetypes.FrameInfo{
        Width:           uint16(w),
        Height:          uint16(h),
        BitsAllocated:   8,
        BitsStored:      8,
        HighBit:         7,
        SamplesPerPixel: uint16(comps),
    }
    pdIn := codecHelpers.NewTestPixelData(frameInfo)
    pdIn.AddFrame(src)
    pdOut := codecHelpers.NewTestPixelData(frameInfo)

    params := NewLossyParameters()
    params.Quality = 90
    b := jpeg2000.MCTBindingParams{ AssocType: 1, ComponentIDs: []uint16{0,1,2}, Matrix: [][]float64{{1,0,0},{0,1,0},{0,0,1}}, Inverse: [][]float64{{1,0,0},{0,1,0},{0,0,1}}, ElementType: 1, MCOPrecision: 0 }
    params.SetParameter("mctBindings", []jpeg2000.MCTBindingParams{b})
    c := NewCodec(90)
    if err := c.Encode(pdIn, pdOut, params); err != nil { t.Fatalf("encode failed: %v", err) }

    encodedData, _ := pdOut.GetFrame(0)
    cs, err := codestream.NewParser(encodedData).Parse()
    if err != nil { t.Fatalf("parse failed: %v", err) }
    if len(cs.MCT) == 0 || len(cs.MCC) == 0 { t.Fatalf("expected MCT/MCC present") }
}
