package t1

import (
	"testing"
	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// TestMQTraceWorking traces a working case [1,3,0,0]
func TestMQTraceWorking(t *testing.T) {
	// Enable MQ debug logging
	mqc.EnableEncoderDebug()
	mqc.EnableDecoderDebug()

	width, height := 2, 2
	data := []int32{1, 3, 0, 0}

	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3

	t.Logf("============ WORKING CASE: [1,3,0,0] ============")

	// Encode
	t.Logf("\n=== ENCODING ===")
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes: %v", len(encoded), encoded)

	// Decode
	t.Logf("\n=== DECODING ===")
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	t.Logf("\n=== RESULTS ===")
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Position %d: expected=%d, got=%d", i, data[i], decoded[i])
		} else {
			t.Logf("Position %d: %d ✓", i, decoded[i])
		}
	}
}

// TestMQTraceFailing traces a failing case [1,2,0,0]
func TestMQTraceFailing(t *testing.T) {
	// Enable MQ debug logging
	mqc.EnableEncoderDebug()
	mqc.EnableDecoderDebug()

	width, height := 2, 2
	data := []int32{1, 2, 0, 0}

	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3

	t.Logf("============ FAILING CASE: [1,2,0,0] ============")

	// Encode
	t.Logf("\n=== ENCODING ===")
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes: %v", len(encoded), encoded)

	// Decode
	t.Logf("\n=== DECODING ===")
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	t.Logf("\n=== RESULTS ===")
	for i := range data {
		if decoded[i] != data[i] {
			t.Errorf("Position %d: expected=%d, got=%d", i, data[i], decoded[i])
		} else {
			t.Logf("Position %d: %d ✓", i, decoded[i])
		}
	}
}
