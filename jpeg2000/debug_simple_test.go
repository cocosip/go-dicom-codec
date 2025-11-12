package jpeg2000

import (
	"fmt"
	"testing"
)

// TestDebugSimplePatterns tests very simple patterns to isolate the issue
func TestDebugSimplePatterns(t *testing.T) {
	testCases := []struct {
		size  int
		value int32
		name  string
	}{
		{20, 0, "20x20-zeros"},
		{24, 0, "24x24-zeros"},
		{28, 0, "28x28-zeros"},
		{32, 0, "32x32-zeros"},
		{20, 1, "20x20-ones"},
		{24, 1, "24x24-ones"},
		{28, 1, "28x28-ones"},
		{32, 1, "32x32-ones"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size
			numPixels := size * size

			// Create uniform data with single value
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)
			for i := 0; i < numPixels; i++ {
				componentData[0][i] = tc.value
			}

			// Encode
			params := DefaultEncodeParams(size, size, 1, 8, false)
			params.NumLevels = 0
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decodedData, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("GetComponentData failed: %v", err)
			}

			// Verify
			errorCount := 0
			for i := 0; i < numPixels; i++ {
				if componentData[0][i] != decodedData[i] {
					errorCount++
					if errorCount <= 5 {
						t.Logf("  Error at pixel %d: orig=%d, decoded=%d", i, componentData[0][i], decodedData[i])
					}
				}
			}

			if errorCount == 0 {
				t.Logf("✓ %s: Perfect (%d bytes)", tc.name, len(encoded))
			} else {
				errorRate := float64(errorCount) / float64(numPixels) * 100
				t.Errorf("✗ %s: %d errors (%.1f%%)", tc.name, errorCount, errorRate)
			}
		})
	}
}

// TestDebugFirstColumn tests encoding/decoding of just the first column
func TestDebugFirstColumn(t *testing.T) {
	testSizes := []int{20, 24, 28, 32}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			numPixels := size * size
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			// Set only first column to non-zero values
			for y := 0; y < size; y++ {
				componentData[0][y*size] = int32(y)
			}

			// Encode
			params := DefaultEncodeParams(size, size, 1, 8, false)
			params.NumLevels = 0
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding failed: %v", err)
			}

			decodedData, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("GetComponentData failed: %v", err)
			}

			// Check first column
			t.Logf("First column (x=0):")
			errorCount := 0
			for y := 0; y < size && y < 10; y++ {
				idx := y * size
				orig := componentData[0][idx]
				decoded := decodedData[idx]
				match := ""
				if orig != decoded {
					match = fmt.Sprintf(" ← ERROR (diff=%d)", decoded-orig)
					errorCount++
				}
				t.Logf("  y=%d: orig=%d, decoded=%d%s", y, orig, decoded, match)
			}

			// Count total errors
			totalErrors := 0
			for i := 0; i < numPixels; i++ {
				if componentData[0][i] != decodedData[i] {
					totalErrors++
				}
			}

			if totalErrors > 0 {
				t.Errorf("Total errors: %d/%d", totalErrors, numPixels)
			} else {
				t.Logf("✓ Perfect reconstruction")
			}
		})
	}
}
