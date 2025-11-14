package t1

import (
	"testing"
)

// TestDetailedTwoValues tests [1, 0, 0, 2] with detailed tracking
func TestDetailedTwoValues(t *testing.T) {
	width, height := 2, 2
	data := []int32{1, 0, 0, 2}

	maxBitplane := 1
	numPasses := (maxBitplane + 1) * 3

	t.Logf("============================================================")
	t.Logf("Testing pattern: [1, 0, 0, 2]")
	t.Logf("Position 0: value=1 (binary: 01)")
	t.Logf("Position 1: value=0 (binary: 00)")
	t.Logf("Position 2: value=0 (binary: 00)")
	t.Logf("Position 3: value=2 (binary: 10)")
	t.Logf("MaxBitplane: %d, NumPasses: %d", maxBitplane, numPasses)
	t.Logf("============================================================")

	// Encode
	enc := NewT1Encoder(width, height, 0)
	encoded, err := enc.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	t.Logf("Encoded %d bytes", len(encoded))

	// Decode
	dec := NewT1Decoder(width, height, 0)
	err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	decoded := dec.GetData()

	t.Logf("\nResults:")
	for i := range data {
		status := "✓"
		if decoded[i] != data[i] {
			status = "✗"
		}
		t.Logf("  Position %d: expected=%d, got=%d %s", i, data[i], decoded[i], status)
	}

	// Count errors
	errors := 0
	for i := range data {
		if decoded[i] != data[i] {
			errors++
		}
	}

	if errors > 0 {
		t.Errorf("Failed: %d/%d values incorrect", errors, len(data))
	}
}

// TestEachPositionSeparately tests each position individually
func TestEachPositionSeparately(t *testing.T) {
	tests := []struct {
		name string
		data []int32
	}{
		{"Position 0 only (1)", []int32{1, 0, 0, 0}},
		{"Position 1 only (1)", []int32{0, 1, 0, 0}},
		{"Position 2 only (1)", []int32{0, 0, 1, 0}},
		{"Position 3 only (1)", []int32{0, 0, 0, 1}},
		{"Position 3 only (2)", []int32{0, 0, 0, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find max value to determine maxBitplane
			maxVal := int32(0)
			for _, v := range tt.data {
				if v < 0 {
					v = -v
				}
				if v > maxVal {
					maxVal = v
				}
			}

			maxBitplane := 0
			if maxVal > 0 {
				for maxVal > 0 {
					maxVal >>= 1
					maxBitplane++
				}
				maxBitplane--
			}

			numPasses := (maxBitplane + 1) * 3

			enc := NewT1Encoder(2, 2, 0)
			encoded, err := enc.Encode(tt.data, numPasses, 0)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			dec := NewT1Decoder(2, 2, 0)
			err = dec.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			decoded := dec.GetData()

			for i := range tt.data {
				if decoded[i] != tt.data[i] {
					t.Errorf("Position %d: expected=%d, got=%d", i, tt.data[i], decoded[i])
				}
			}
		})
	}
}
