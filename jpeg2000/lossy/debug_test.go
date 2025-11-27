package lossy

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

// TestDirectEncodeDecode tests direct encoder/decoder usage to debug DC shift issue
func TestDirectEncodeDecode(t *testing.T) {
	// Simple 4x4 image with known values
	width := 4
	height := 4

	// Create component data (grayscale)
	componentData := [][]int32{make([]int32, width*height)}
	for i := 0; i < width*height; i++ {
		componentData[0][i] = int32(i * 16) // 0, 16, 32, ..., 240
	}

	// Print original data
	t.Logf("Original data (as int32):")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t.Logf("  [%d,%d] = %d", y, x, componentData[0][y*width+x])
		}
	}

	// Create encoding parameters (lossy with 9/7 wavelet)
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 1    // Use only 1 level for debugging
	params.Lossless = false // Use 9/7 wavelet (lossy)

	// Encode
	encoder := jpeg2000.NewEncoder(params)
	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Decode
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded data as int32
	imageData := decoder.GetImageData()

	// Print decoded data as int32
	t.Logf("Decoded data (as int32):")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t.Logf("  [%d,%d] = %d", y, x, imageData[0][y*width+x])
		}
	}

	// Get decoded data as bytes
	pixelData := decoder.GetPixelData()
	t.Logf("Decoded data (as bytes):")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			t.Logf("  [%d,%d] = %d", y, x, pixelData[y*width+x])
		}
	}

	// Calculate errors for int32
	t.Logf("Errors (int32):")
	maxError := int32(0)
	for i := 0; i < width*height; i++ {
		diff := imageData[0][i] - componentData[0][i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		if diff != 0 {
			t.Logf("  Pixel %d: original=%d, decoded=%d, error=%d",
				i, componentData[0][i], imageData[0][i], imageData[0][i]-componentData[0][i])
		}
	}

	t.Logf("Max error (int32): %d", maxError)

	// Calculate errors for bytes
	t.Logf("Errors (bytes):")
	maxErrorByte := 0
	for i := 0; i < width*height; i++ {
		diff := int(pixelData[i]) - int(componentData[0][i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxErrorByte {
			maxErrorByte = diff
		}
		if diff != 0 {
			t.Logf("  Pixel %d: original=%d, decoded=%d, error=%d",
				i, componentData[0][i], pixelData[i], int(pixelData[i])-int(componentData[0][i]))
		}
	}

	t.Logf("Max error (bytes): %d", maxErrorByte)

	// For debugging, allow larger error temporarily
	if maxError > 5 {
		t.Errorf("Max error (int32) too large: %d", maxError)
	}
	if maxErrorByte > 5 {
		t.Errorf("Max error (bytes) too large: %d", maxErrorByte)
	}
}

// TestByteEncodeDecode tests encoding from byte array
func TestByteEncodeDecode(t *testing.T) {
	// Simple 4x4 image with known values
	width := 4
	height := 4
	numPixels := width * height

	// Create byte pixel data
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i * 16) // 0, 16, 32, ..., 240
	}

	// Print original data
	t.Logf("Original pixel data (bytes):")
	for i := 0; i < numPixels; i++ {
		t.Logf("  Pixel %d = %d", i, pixelData[i])
	}

	// Create encoding parameters (lossy with 9/7 wavelet)
	params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLevels = 1    // Use only 1 level for debugging
	params.Lossless = false // Use 9/7 wavelet (lossy)

	// Encode using byte array
	encoder := jpeg2000.NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// Decode
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Get decoded data as bytes
	decodedBytes := decoder.GetPixelData()

	// Print decoded data
	t.Logf("Decoded pixel data (bytes):")
	for i := 0; i < numPixels; i++ {
		t.Logf("  Pixel %d = %d", i, decodedBytes[i])
	}

	// Calculate errors
	t.Logf("Errors:")
	maxError := 0
	for i := 0; i < numPixels; i++ {
		diff := int(decodedBytes[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		if diff != 0 {
			t.Logf("  Pixel %d: original=%d, decoded=%d, error=%d",
				i, pixelData[i], decodedBytes[i], int(decodedBytes[i])-int(pixelData[i]))
		}
	}

	t.Logf("Max error: %d", maxError)

	if maxError > 5 {
		t.Errorf("Max error too large: %d", maxError)
	}
}
