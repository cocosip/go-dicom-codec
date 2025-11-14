package t1

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// TestGradientMQTrace traces a minimal gradient case to find where MQ desync happens
func TestGradientMQTrace(t *testing.T) {
	// Enable MQ debug logging
	mqc.EnableEncoderDebug()
	mqc.EnableDecoderDebug()

	// Use a minimal 3x3 gradient that passes
	t.Run("3x3_gradient_working", func(t *testing.T) {
		width, height := 3, 3
		data := make([]int32, width*height)
		for i := range data {
			data[i] = -int32(i+1) * 10 // -10, -20, -30, ...
		}

		maxBitplane := 6 // log2(128) = 7, but max value is 90
		numPasses := (maxBitplane + 1) * 3

		t.Logf("============ 3x3 GRADIENT (WORKING) ============")
		t.Logf("Data: %v", data)

		// Encode
		t.Logf("\n=== ENCODING ===")
		enc := NewT1Encoder(width, height, 0)
		encoded, err := enc.Encode(data, numPasses, 0)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		t.Logf("Encoded %d bytes", len(encoded))

		// Decode
		t.Logf("\n=== DECODING ===")
		dec := NewT1Decoder(width, height, 0)
		err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decoded := dec.GetData()

		// Check results
		t.Logf("\n=== RESULTS ===")
		errorCount := 0
		for i := range data {
			if decoded[i] != data[i] {
				t.Errorf("Position %d: expected=%d, got=%d", i, data[i], decoded[i])
				errorCount++
			}
		}
		if errorCount == 0 {
			t.Logf("3x3 gradient: PASS (all values correct)")
		} else {
			t.Logf("3x3 gradient: FAIL (%d errors)", errorCount)
		}
	})

	// Use 5x5 gradient that fails
	t.Run("5x5_gradient_failing", func(t *testing.T) {
		width, height := 5, 5
		data := make([]int32, width*height)
		for i := range data {
			data[i] = -int32(i+1) * 5 // -5, -10, -15, ...
		}

		maxBitplane := 6 // log2(128) = 7, but max value is 125
		numPasses := (maxBitplane + 1) * 3

		t.Logf("============ 5x5 GRADIENT (FAILING) ============")
		t.Logf("Data: %v", data)

		// Encode
		t.Logf("\n=== ENCODING ===")
		enc := NewT1Encoder(width, height, 0)
		encoded, err := enc.Encode(data, numPasses, 0)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		t.Logf("Encoded %d bytes", len(encoded))

		// Decode
		t.Logf("\n=== DECODING ===")
		dec := NewT1Decoder(width, height, 0)
		err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		decoded := dec.GetData()

		// Check results
		t.Logf("\n=== RESULTS ===")
		errorCount := 0
		maxError := int32(0)
		for i := range data {
			if decoded[i] != data[i] {
				err := decoded[i] - data[i]
				if err < 0 {
					err = -err
				}
				if err > maxError {
					maxError = err
				}
				t.Errorf("Position %d: expected=%d, got=%d (error=%d)", i, data[i], decoded[i], decoded[i]-data[i])
				errorCount++
			}
		}
		if errorCount == 0 {
			t.Logf("5x5 gradient: PASS (all values correct)")
		} else {
			t.Logf("5x5 gradient: FAIL (%d errors, max error=%d)", errorCount, maxError)
		}
	})
}
