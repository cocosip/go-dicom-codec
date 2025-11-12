package jpeg2000

import (
	"fmt"
	"testing"
)

// TestDebugComparison compares 20x20 (working) vs 24x24 (failing)
func TestDebugComparison(t *testing.T) {
	testCases := []struct {
		size int
		name string
	}{
		{20, "20x20-working"},
		{24, "24x24-failing"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.size

			// Create simple gradient pattern
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, size*size)

			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					idx := y*size + x
					componentData[0][idx] = int32((x + y) % 256)
				}
			}

			// Encode
			params := DefaultEncodeParams(size, size, 1, 8, false)
			params.NumLevels = 0
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			t.Logf("%s: Encoded to %d bytes", tc.name, len(encoded))

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

			// Check first few pixels and last few pixels
			t.Logf("First 10 pixels:")
			for i := 0; i < 10 && i < len(componentData[0]); i++ {
				orig := componentData[0][i]
				decoded := decodedData[i]
				match := ""
				if orig != decoded {
					match = fmt.Sprintf(" ← ERROR (diff=%d)", decoded-orig)
				}
				t.Logf("  [%d] orig=%d, decoded=%d%s", i, orig, decoded, match)
			}

			// Count total errors
			errorCount := 0
			for i := 0; i < len(componentData[0]); i++ {
				if componentData[0][i] != decodedData[i] {
					errorCount++
				}
			}

			errorRate := float64(errorCount) / float64(len(componentData[0])) * 100
			t.Logf("Total errors: %d/%d (%.1f%%)", errorCount, len(componentData[0]), errorRate)

			if errorCount > 0 {
				t.Errorf("Expected perfect reconstruction, got %d errors", errorCount)
			}
		})
	}
}
