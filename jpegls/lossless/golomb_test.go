package lossless

import (
	"bytes"
	"testing"
)

func TestEncodedDecodeValueSymmetry(t *testing.T) {
	// Test EncodeMappedValue <-> DecodeValue symmetry
	testCases := []struct {
		k                     int
		mappedError           int
		limit                 int
		quantizedBitsPerPixel int
	}{
		{0, 0, 64, 16},
		{0, 1, 64, 16},
		{0, 5, 64, 16},
		{1, 0, 64, 16},
		{1, 10, 64, 16},
		{5, 100, 64, 16},
		{10, 500, 64, 16},
		{10, 1024, 64, 16},
		// Test limit boundary
		{10, 10000, 64, 16},
	}

	for _, tc := range testCases {
		var buf bytes.Buffer
		writer := NewGolombWriter(&buf)

		// Encode
		err := writer.EncodeMappedValue(tc.k, tc.mappedError, tc.limit, tc.quantizedBitsPerPixel)
		if err != nil {
			t.Fatalf("EncodeMappedValue failed for k=%d, mappedError=%d: %v", tc.k, tc.mappedError, err)
		}

		if err := writer.Flush(); err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		// Decode
		reader := NewGolombReader(&buf)
		decoded, err := reader.DecodeValue(tc.k, tc.limit, tc.quantizedBitsPerPixel)
		if err != nil {
			t.Fatalf("DecodeValue failed for k=%d, mappedError=%d: %v", tc.k, tc.mappedError, err)
		}

		if decoded != tc.mappedError {
			t.Errorf("Mismatch: k=%d, original=%d, decoded=%d", tc.k, tc.mappedError, decoded)
		}
	}

	t.Log("✓ EncodeMappedValue/DecodeValue symmetry test passed")
}

func TestGolombRoundTrip(t *testing.T) {
	// Test values
	testValues := []int{0, 1, 2, 3, 5, 10, 100, 255, 1000, 65535}

	for k := 0; k <= 8; k++ {
		for _, val := range testValues {
			var buf bytes.Buffer
			writer := NewGolombWriter(&buf)

			// Write
			err := writer.WriteGolomb(val, k)
			if err != nil {
				t.Fatalf("WriteGolomb(%d, k=%d) failed: %v", val, k, err)
			}

			err = writer.Flush()
			if err != nil {
				t.Fatalf("Flush failed: %v", err)
			}

			// Read back directly from stuffed data (CharLS way)
			// The new GolombReader handles byte stuffing and markers internally
			stuffedData := buf.Bytes()

			// Read back
			reader := NewGolombReader(bytes.NewReader(stuffedData))
			decoded, err := reader.ReadGolomb(k)
			if err != nil {
				t.Fatalf("ReadGolomb(k=%d) failed: %v", k, err)
			}

			if decoded != val {
				t.Errorf("Round-trip failed for value=%d, k=%d: got %d", val, k, decoded)
			}
		}
	}

	t.Log("✓ Golomb round-trip test passed")
}

func TestSimpleEncode(t *testing.T) {
	// Create simple 4x4 image with gradient
	width, height := 4, 4
	bitDepth := 8
	numPixels := width * height

	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i * 10) // 0, 10, 20, 30, ...
	}

	t.Logf("Input pixels (first 16): %v", pixelData)

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Check that encoded data is not all zeros
	allZero := true
	for _, b := range encoded {
		if b != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		t.Error("❌ Encoded data is all zeros!")
	} else {
		t.Log("✓ Encoded data contains non-zero bytes")
	}

	// Find scan data
	scanStart := -1
	for i := 0; i < len(encoded)-1; i++ {
		if encoded[i] == 0xFF && encoded[i+1] == 0xDA {
			// Skip SOS header
			length := int(encoded[i+2])<<8 | int(encoded[i+3])
			scanStart = i + 2 + length
			break
		}
	}

	if scanStart > 0 {
		scanEnd := len(encoded) - 2 // Before EOI
		scanData := encoded[scanStart:scanEnd]
		t.Logf("Scan data size: %d bytes", len(scanData))
		t.Logf("First 20 bytes of scan data: %X", scanData[:min(20, len(scanData))])

		allZeroScan := true
		for _, b := range scanData {
			if b != 0 {
				allZeroScan = false
				break
			}
		}

		if allZeroScan {
			t.Error("❌ Scan data is all zeros!")
		} else {
			t.Log("✓ Scan data contains non-zero bytes")
		}
	}

	// Decode
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %dx%d, %d components, %d bits", w, h, c, b)
	t.Logf("Decoded pixels (first 16): %v", decoded[:min(16, len(decoded))])

	// Compare
	if len(pixelData) != len(decoded) {
		t.Fatalf("Size mismatch: %d vs %d", len(pixelData), len(decoded))
	}

	diffs := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decoded[i] {
			diffs++
		}
	}

	if diffs > 0 {
		t.Errorf("Found %d differences", diffs)
	} else {
		t.Log("✓ Perfect reconstruction")
	}
}

func Test16BitEncode(t *testing.T) {
	// Test 16-bit encoding like the real DICOM image
	width, height := 4, 4
	bitDepth := 16
	numPixels := width * height

	// Create 16-bit pixel data (little-endian)
	pixelData := make([]byte, numPixels*2)
	for i := 0; i < numPixels; i++ {
		// Signed 16-bit values: -1000, -900, -800, etc.
		val := int16(-1000 + i*100)
		pixelData[i*2] = byte(val & 0xFF)
		pixelData[i*2+1] = byte((val >> 8) & 0xFF)
	}

	t.Logf("Input 16-bit pixels (first 8 values as int16): ")
	for i := 0; i < 8; i++ {
		val := int16(pixelData[i*2]) | (int16(pixelData[i*2+1]) << 8)
		t.Logf("  [%d] = %d", i, val)
	}

	// Encode
	encoded, err := Encode(pixelData, width, height, 1, bitDepth)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Find scan data
	scanStart := -1
	for i := 0; i < len(encoded)-1; i++ {
		if encoded[i] == 0xFF && encoded[i+1] == 0xDA {
			// Skip SOS header
			length := int(encoded[i+2])<<8 | int(encoded[i+3])
			scanStart = i + 2 + length
			break
		}
	}

	if scanStart > 0 {
		scanEnd := len(encoded) - 2 // Before EOI
		scanData := encoded[scanStart:scanEnd]
		t.Logf("Scan data size: %d bytes", len(scanData))
		t.Logf("First 40 bytes of scan data: %X", scanData[:min(40, len(scanData))])

		// Check if scan data is all zeros
		allZero := true
		for _, b := range scanData {
			if b != 0 {
				allZero = false
				break
			}
		}

		if allZero {
			t.Error("❌ CRITICAL: Scan data is all zeros for 16-bit image!")
			t.Error("This is the same issue we see with D:\\1.dcm")
		} else {
			t.Log("✓ Scan data contains non-zero bytes")
		}
	}

	// Try to decode
	decoded, w, h, c, b, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	t.Logf("Decoded: %dx%d, %d components, %d bits", w, h, c, b)

	// Compare
	if len(pixelData) != len(decoded) {
		t.Fatalf("Size mismatch: %d vs %d", len(pixelData), len(decoded))
	}

	diffs := 0
	for i := 0; i < len(pixelData); i++ {
		if pixelData[i] != decoded[i] {
			diffs++
		}
	}

	if diffs > 0 {
		t.Errorf("Found %d byte differences", diffs)
	} else {
		t.Log("✓ Perfect reconstruction")
	}
}
