package t2

import (
	"testing"
)

// TestTagTreeDecoderBasic tests basic tag tree decoding
func TestTagTreeDecoderBasic(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		leafX     int
		leafY     int
		threshold int
		bits      []int // Bit sequence to return
		expected  int
	}{
		{
			name:      "2x2 tree, decode value 0",
			width:     2,
			height:    2,
			leafX:     0,
			leafY:     0,
			threshold: 5,
			bits:      []int{1}, // First bit is 1 → value is 0
			expected:  0,
		},
		{
			name:      "2x2 tree, decode value 2",
			width:     2,
			height:    2,
			leafX:     1,
			leafY:     0,
			threshold: 5,
			bits:      []int{0, 0, 1}, // Two 0s then 1 → value is 2
			expected:  2,
		},
		{
			name:      "4x4 tree, decode value 3",
			width:     4,
			height:    4,
			leafX:     2,
			leafY:     1,
			threshold: 10,
			bits:      []int{0, 0, 0, 1}, // Three 0s then 1 → value is 3
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			bitIdx := 0
			bitReader := func() (int, error) {
				if bitIdx >= len(tt.bits) {
					t.Fatalf("Ran out of test bits")
				}
				bit := tt.bits[bitIdx]
				bitIdx++
				return bit, nil
			}

			value, err := decoder.Decode(tt.leafX, tt.leafY, tt.threshold, bitReader)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			if value != tt.expected {
				t.Errorf("Expected value %d, got %d", tt.expected, value)
			}
		})
	}
}

// TestTagTreeDecoderInclusion tests code-block inclusion decoding
func TestTagTreeDecoderInclusion(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		cbX          int
		cbY          int
		currentLayer int
		bits         []int
		wantIncluded bool
		wantLayer    int
	}{
		{
			name:         "Included in layer 0",
			width:        2,
			height:       2,
			cbX:          0,
			cbY:          0,
			currentLayer: 0,
			bits:         []int{1}, // Value 0 ≤ currentLayer 0 → included
			wantIncluded: true,
			wantLayer:    0,
		},
		{
			name:         "Not included yet (value > currentLayer)",
			width:        2,
			height:       2,
			cbX:          1,
			cbY:          1,
			currentLayer: 0,
			bits:         []int{0, 1}, // Value 1 > currentLayer 0 → not included
			wantIncluded: false,
			wantLayer:    -1,
		},
		{
			name:         "Included in layer 2",
			width:        4,
			height:       4,
			cbX:          2,
			cbY:          2,
			currentLayer: 3,
			bits:         []int{0, 0, 1}, // Value 2 ≤ currentLayer 3 → included in layer 2
			wantIncluded: true,
			wantLayer:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			bitIdx := 0
			bitReader := func() (int, error) {
				if bitIdx >= len(tt.bits) {
					t.Fatalf("Ran out of test bits")
				}
				bit := tt.bits[bitIdx]
				bitIdx++
				return bit, nil
			}

			included, layer, err := decoder.DecodeInclusion(tt.cbX, tt.cbY, tt.currentLayer, bitReader)
			if err != nil {
				t.Fatalf("DecodeInclusion failed: %v", err)
			}

			if included != tt.wantIncluded {
				t.Errorf("Expected included=%v, got %v", tt.wantIncluded, included)
			}
			if layer != tt.wantLayer {
				t.Errorf("Expected layer=%d, got %d", tt.wantLayer, layer)
			}
		})
	}
}

// TestTagTreeDecoderZeroBitPlanes tests zero bit-planes decoding
func TestTagTreeDecoderZeroBitPlanes(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		cbX      int
		cbY      int
		bits     []int
		expected int
	}{
		{
			name:     "0 zero bit-planes",
			width:    2,
			height:   2,
			cbX:      0,
			cbY:      0,
			bits:     []int{1}, // First bit is 1 → value is 0
			expected: 0,
		},
		{
			name:     "3 zero bit-planes",
			width:    4,
			height:   4,
			cbX:      1,
			cbY:      2,
			bits:     []int{0, 0, 0, 1}, // Three 0s then 1 → value is 3
			expected: 3,
		},
		{
			name:     "5 zero bit-planes",
			width:    4,
			height:   4,
			cbX:      3,
			cbY:      3,
			bits:     []int{0, 0, 0, 0, 0, 1}, // Five 0s then 1 → value is 5
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTagTree(tt.width, tt.height)
			decoder := NewTagTreeDecoder(tree)

			bitIdx := 0
			bitReader := func() (int, error) {
				if bitIdx >= len(tt.bits) {
					t.Fatalf("Ran out of test bits")
				}
				bit := tt.bits[bitIdx]
				bitIdx++
				return bit, nil
			}

			zbp, err := decoder.DecodeZeroBitPlanes(tt.cbX, tt.cbY, bitReader)
			if err != nil {
				t.Fatalf("DecodeZeroBitPlanes failed: %v", err)
			}

			if zbp != tt.expected {
				t.Errorf("Expected %d zero bit-planes, got %d", tt.expected, zbp)
			}
		})
	}
}

// TestTagTreeDecoderReset tests tag tree state reset
func TestTagTreeDecoderReset(t *testing.T) {
	tree := NewTagTree(2, 2)
	decoder := NewTagTreeDecoder(tree)

	// Decode a value
	bitIdx := 0
	bits := []int{0, 0, 1} // Value 2
	bitReader := func() (int, error) {
		bit := bits[bitIdx]
		bitIdx++
		return bit, nil
	}

	value, err := decoder.Decode(0, 0, 5, bitReader)
	if err != nil || value != 2 {
		t.Fatalf("Initial decode failed: value=%d, err=%v", value, err)
	}

	// Reset
	decoder.Reset()

	// Decode again - should decode from scratch
	bitIdx = 0
	bits = []int{1} // Value 0
	value, err = decoder.Decode(0, 0, 5, bitReader)
	if err != nil || value != 0 {
		t.Errorf("After reset, expected value 0, got %d (err=%v)", value, err)
	}
}

// TestTagTreeDecoderMultipleLeaves tests decoding multiple leaves
func TestTagTreeDecoderMultipleLeaves(t *testing.T) {
	tree := NewTagTree(4, 4)
	decoder := NewTagTreeDecoder(tree)

	// Prepare different bit sequences for different leaves
	leafData := map[int][]int{
		0:  {1},          // Leaf (0,0) → value 0
		1:  {0, 1},       // Leaf (1,0) → value 1
		4:  {0, 0, 1},    // Leaf (0,1) → value 2
		5:  {0, 0, 0, 1}, // Leaf (1,1) → value 3
	}

	for leafIdx, expectedBits := range leafData {
		leafX := leafIdx % 4
		leafY := leafIdx / 4

		bitIdx := 0
		bitReader := func() (int, error) {
			if bitIdx >= len(expectedBits) {
				t.Fatalf("Ran out of bits for leaf %d", leafIdx)
			}
			bit := expectedBits[bitIdx]
			bitIdx++
			return bit, nil
		}

		expectedValue := len(expectedBits) - 1 // Number of 0s before the 1
		value, err := decoder.Decode(leafX, leafY, 10, bitReader)
		if err != nil {
			t.Fatalf("Decode failed for leaf %d: %v", leafIdx, err)
		}

		if value != expectedValue {
			t.Errorf("Leaf %d: expected value %d, got %d", leafIdx, expectedValue, value)
		}
	}
}

// TestTagTreeDecoderBoundaryConditions tests edge cases
func TestTagTreeDecoderBoundaryConditions(t *testing.T) {
	t.Run("Out of bounds coordinates", func(t *testing.T) {
		tree := NewTagTree(2, 2)
		decoder := NewTagTreeDecoder(tree)

		bitReader := func() (int, error) {
			t.Fatal("Should not call bitReader for out of bounds")
			return 0, nil
		}

		// Test out of bounds - should return 0
		value, err := decoder.Decode(-1, 0, 5, bitReader)
		if err != nil || value != 0 {
			t.Errorf("Out of bounds should return 0, got %d (err=%v)", value, err)
		}

		value, err = decoder.Decode(0, -1, 5, bitReader)
		if err != nil || value != 0 {
			t.Errorf("Out of bounds should return 0, got %d (err=%v)", value, err)
		}

		value, err = decoder.Decode(2, 0, 5, bitReader)
		if err != nil || value != 0 {
			t.Errorf("Out of bounds should return 0, got %d (err=%v)", value, err)
		}

		value, err = decoder.Decode(0, 2, 5, bitReader)
		if err != nil || value != 0 {
			t.Errorf("Out of bounds should return 0, got %d (err=%v)", value, err)
		}
	})

	t.Run("Already decoded value below threshold", func(t *testing.T) {
		tree := NewTagTree(2, 2)
		decoder := NewTagTreeDecoder(tree)

		// First decode with value 1
		bitIdx := 0
		bits := []int{0, 1} // Value 1
		bitReader := func() (int, error) {
			bit := bits[bitIdx]
			bitIdx++
			return bit, nil
		}

		value, _ := decoder.Decode(0, 0, 5, bitReader)
		if value != 1 {
			t.Fatalf("Initial decode should give value 1, got %d", value)
		}

		// Second decode with same threshold - should return cached value without reading bits
		callCount := 0
		bitReader2 := func() (int, error) {
			callCount++
			return 0, nil
		}

		value, _ = decoder.Decode(0, 0, 5, bitReader2)
		if value != 1 {
			t.Errorf("Cached decode should give value 1, got %d", value)
		}
		if callCount > 0 {
			t.Errorf("Should not read new bits for cached value, but read %d bits", callCount)
		}
	})
}
