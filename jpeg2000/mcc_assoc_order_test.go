package jpeg2000

import "testing"

// Verify AssocType overrides record application order
func TestMCCAssocOrderOverride(t *testing.T) {
    w, h, comps := 8, 8, 3
    n := w * h
    src := make([][]int32, comps)
    for c := 0; c < comps; c++ {
        src[c] = make([]int32, n)
        for i := 0; i < n; i++ { src[c][i] = int32(i % 100) }
    }
    params := DefaultEncodeParams(w, h, comps, 8, true)
    I := make([][]float64, comps)
    for i := 0; i < comps; i++ { I[i] = make([]float64, comps); for j := 0; j < comps; j++ { if i == j { I[i][j] = 1 } } }
    params.MCTMatrix = I
    params.InverseMCTMatrix = I
    params.MCTReversible = true
    // Provide offsets
    params.MCTOffsets = []int32{5, 0, -3}
    // Force indices reversed but assoc type matrix then offset
    params.MCORecordOrder = []uint8{1, 0}
    params.MCTAssocType = 2 // MatrixThenOffset
    params.NumLevels = 0
    enc := NewEncoder(params)
    data, err := enc.EncodeComponents(src)
    if err != nil { t.Fatalf("encode failed: %v", err) }
    dec := NewDecoder()
    if err := dec.Decode(data); err != nil { t.Fatalf("decode failed: %v", err) }
    for c := 0; c < comps; c++ {
        got, _ := dec.GetComponentData(c)
        // Identity matrix first, then offsets -> result equals src + offsets
        for i := 0; i < 16 && i < len(got); i++ {
            want := src[c][i] + params.MCTOffsets[c]
            if got[i] != want { t.Fatalf("mismatch comp=%d i=%d got=%d want=%d", c, i, got[i], want) }
        }
    }
}
