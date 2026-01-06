package lossless

import (
	"bytes"
	"testing"

	jplscommon "github.com/cocosip/go-dicom-codec/jpegls/common"
)

func TestRunInterruptionSymmetry(t *testing.T) {
	// Test that encodeRunInterruptionError and decodeRunInterruptionError are inverses

	testCases := []struct {
		errorValue  int
		contextType int // 0 or 1
		rangeVal    int
	}{
		{10, 1, 256}, // Pixel value 10, ra=0, rb=0
		{5, 1, 256},
		{-5, 1, 256},
		{20, 0, 256},
		{-10, 0, 256},
	}

	for _, tc := range testCases {
		var buf bytes.Buffer
		gw := NewGolombWriter(&buf)

		// Create context
		encCtx := NewRunModeContext(tc.contextType, tc.rangeVal)
		t.Logf("\nTest: errorValue=%d, contextType=%d", tc.errorValue, tc.contextType)
		t.Logf("Initial context: A=%d, N=%d, NN=%d", encCtx.A, encCtx.N, encCtx.NN)

		// Get k
		k := encCtx.GetGolombCode()
		t.Logf("k=%d", k)

		// Compute map
		mapBit := encCtx.ComputeMap(tc.errorValue, k)
		t.Logf("mapBit=%v", mapBit)

		// Compute eMappedErrorValue
		eMappedErrorValue := 2*jplscommon.Abs(tc.errorValue) - encCtx.runInterruptionType
		if mapBit {
			eMappedErrorValue--
		}
		t.Logf("eMappedErrorValue=%d", eMappedErrorValue)

		// Encode
		limit := 32 - J[0] - 1 // Assuming runIndex=0
		qbpp := 8
		err := gw.EncodeMappedValue(k, eMappedErrorValue, limit, qbpp)
		if err != nil {
			t.Fatalf("EncodeMappedValue failed: %v", err)
		}

		// Update encoder context
		encCtx.UpdateVariables(tc.errorValue, eMappedErrorValue, 64)

		err = gw.Flush()
		if err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		encoded := buf.Bytes()
		t.Logf("Encoded bytes: %X", encoded)

		// Decode
		decCtx := NewRunModeContext(tc.contextType, tc.rangeVal)
		gr := NewGolombReader(&buf)

		k_dec := decCtx.GetGolombCode()
		if k_dec != k {
			t.Errorf("k mismatch: encoded with %d, decoded with %d", k, k_dec)
		}

		decoded_eMapped, err := gr.DecodeValue(k_dec, limit, qbpp)
		if err != nil {
			t.Fatalf("DecodeValue failed: %v", err)
		}

		t.Logf("Decoded eMappedErrorValue=%d", decoded_eMapped)

		if decoded_eMapped != eMappedErrorValue {
			t.Errorf("eMapped mismatch: expected %d, got %d", eMappedErrorValue, decoded_eMapped)
		}

		// Reconstruct error value
		errorValue_dec := decCtx.ComputeErrorValue(decoded_eMapped+decCtx.runInterruptionType, k_dec)
		t.Logf("Decoded errorValue=%d", errorValue_dec)

		if errorValue_dec != tc.errorValue {
			t.Errorf("errorValue mismatch: expected %d, got %d", tc.errorValue, errorValue_dec)
		}
	}
}
