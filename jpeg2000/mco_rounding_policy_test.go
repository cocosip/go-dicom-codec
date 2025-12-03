package jpeg2000

import "testing"

func TestMCORoundingPolicyFloor(t *testing.T) {
	w, h, comps := 8, 8, 2
	n := w * h
	src := make([][]int32, comps)
	for c := 0; c < comps; c++ {
		src[c] = make([]int32, n)
		for i := 0; i < n; i++ {
			src[c][i] = int32(i%7 + 1)
		}
	}
	p := DefaultEncodeParams(w, h, comps, 8, true)
	inv := [][]float64{{0.5, 0}, {0, 0.5}}
	b := MCTBindingParams{AssocType: 1, ComponentIDs: []uint16{0, 1}, Matrix: [][]float64{{1, 0}, {0, 1}}, Inverse: inv, ElementType: 1, MCOPrecision: 0x4}
	p.MCTBindings = []MCTBindingParams{b}
	p.NumLevels = 0
	data, err := NewEncoder(p).EncodeComponents(src)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	dec := NewDecoder()
	if err := dec.Decode(data); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		want0 := int32(int(src[0][i]) / 2)
		want1 := int32(int(src[1][i]) / 2)
		if got := dec.data[0][i]; got != want0 {
			t.Fatalf("comp0 mismatch i=%d got=%d want=%d", i, got, want0)
		}
		if got := dec.data[1][i]; got != want1 {
			t.Fatalf("comp1 mismatch i=%d got=%d want=%d", i, got, want1)
		}
	}
}
