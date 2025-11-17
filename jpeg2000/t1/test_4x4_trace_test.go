package t1

import (
    "testing"
)

func Test4x4Trace(t *testing.T) {
    width, height := 4, 4
    values := []int32{
        -128, -127, -126, -125,
        -124, -123, -122, -121,
        -120, -119, -118, -117,
        -116, -115, -114, -113,
    }

    maxBitplane := 7
    numPasses := (maxBitplane + 1) * 3

    enc := NewT1Encoder(width, height, 0)
    encoded, err := enc.Encode(values, numPasses, 0)
    if err != nil {
        t.Fatalf("encode failed: %v", err)
    }

    dec := NewT1Decoder(width, height, 0)
    if err := dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0); err != nil {
        t.Fatalf("decode failed: %v", err)
    }

    out := dec.GetData()
    errors := 0
    for i := range values {
        if out[i] != values[i] {
            t.Logf("[%d] expected=%d got=%d", i, values[i], out[i])
            errors++
        }
    }
    if errors > 0 {
        t.Fatalf("mismatch: %d/%d", errors, len(values))
    }
}