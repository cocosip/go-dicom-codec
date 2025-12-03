package jpeg2000

import (
    "testing"
    "github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
)

func TestMCOPrecisionReversibleFlag(t *testing.T) {
    w, h, comps := 16, 16, 3
    n := w * h
    src := make([][]int32, comps)
    for c := 0; c < comps; c++ {
        src[c] = make([]int32, n)
        for i := 0; i < n; i++ { src[c][i] = int32((i + c) % 200) }
    }
    p := DefaultEncodeParams(w, h, comps, 8, true)
    I := make([][]float64, comps)
    for i := 0; i < comps; i++ { I[i] = make([]float64, comps); for j := 0; j < comps; j++ { if i == j { I[i][j] = 1 } } }
    p.MCTMatrix = I
    p.InverseMCTMatrix = I
    p.MCTReversible = true
    p.MCTMatrixElementType = 0
    p.MCTNormScale = 1.0
    p.MCOPrecision = 1 // reversible flag
    p.NumLevels = 0
    data, err := NewEncoder(p).EncodeComponents(src)
    if err != nil { t.Fatalf("encode failed: %v", err) }
    cs, err := codestream.NewParser(data).Parse()
    if err != nil { t.Fatalf("parse failed: %v", err) }
    if len(cs.MCO) == 0 { t.Fatalf("expected MCO present") }
    opt := cs.MCO[0].Options
    found := false
    for i := 0; i < len(opt); {
        tpe := opt[i]; i++
        switch tpe {
        case codestream.MCOOptPrecision:
            if i >= len(opt) { t.Fatalf("precision payload missing") }
            if opt[i]&0x1 == 0 { t.Fatalf("reversible flag not set") }
            found = true
            i++
        case codestream.MCOOptNormScale:
            i += 4
        case codestream.MCOOptRecordOrder:
            if i >= len(opt) { t.Fatalf("order count missing") }
            cnt := int(opt[i]); i++
            i += cnt
        default:
            t.Fatalf("unknown option type %d", tpe)
        }
    }
    if !found { t.Fatalf("precision option not found") }
    dec := NewDecoder()
    if err := dec.Decode(data); err != nil { t.Fatalf("decode failed: %v", err) }
    for c := 0; c < comps; c++ {
        got, _ := dec.GetComponentData(c)
        for i := 0; i < 16 && i < len(got); i++ {
            if got[i] != src[c][i] { t.Fatalf("mismatch comp=%d i=%d got=%d want=%d", c, i, got[i], src[c][i]) }
        }
    }
}
