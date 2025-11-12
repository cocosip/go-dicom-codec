package jpeg2000

import (
	"fmt"
	"testing"
)

// TestDebugUniform tests uniform data (all pixels same value)
func TestDebugUniform(t *testing.T) {
	sizes := []int{16, 20, 24, 28, 32}
	value := int32(50) // Uniform value

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			// Create uniform data
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, size*size)

			for i := 0; i < size*size; i++ {
				componentData[0][i] = value
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
			for i := 0; i < len(componentData[0]); i++ {
				if componentData[0][i] != decodedData[i] {
					errorCount++
					if errorCount <= 5 {
						t.Logf("Error at pixel %d: orig=%d, decoded=%d", i, componentData[0][i], decodedData[i])
					}
				}
			}

			if errorCount == 0 {
				t.Logf("✓ %dx%d uniform: Perfect (%d bytes)", size, size, len(encoded))
			} else {
				t.Errorf("✗ %dx%d uniform: %d errors", size, size, errorCount)
			}
		})
	}
}
