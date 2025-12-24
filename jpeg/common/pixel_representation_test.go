package common

import (
	"encoding/binary"
	"testing"
)

func TestDetectActualPixelRepresentation(t *testing.T) {
	tests := []struct {
		name          string
		pixelData     []byte
		bitsStored    int
		currentPR     int
		wantNegatives bool
		wantMin       int32
		wantMax       int32
	}{
		{
			name:          "16-bit PR=1 with positive values only [286, 1781]",
			pixelData:     make16BitData([]uint16{286, 590, 1781, 500, 1000}),
			bitsStored:    16,
			currentPR:     1,
			wantNegatives: false, // all values < 32768, interpreted as positive
			wantMin:       286,
			wantMax:       1781,
		},
		{
			name:          "16-bit PR=1 with actual negatives [-2048, 255]",
			pixelData:     make16BitData([]uint16{63488, 0, 100, 255}), // 63488 = -2048 in signed
			bitsStored:    16,
			currentPR:     1,
			wantNegatives: true, // 63488 >= 32768, interpreted as -2048
			wantMin:       -2048,
			wantMax:       255,
		},
		{
			name:          "16-bit PR=0 unsigned [0, 65535]",
			pixelData:     make16BitData([]uint16{0, 32768, 40000, 65535}),
			bitsStored:    16,
			currentPR:     0,
			wantNegatives: false, // all unsigned, no negatives
			wantMin:       0,
			wantMax:       65535,
		},
		{
			name:          "12-bit PR=1 with positives [0, 2047]",
			pixelData:     make16BitData([]uint16{0, 500, 1000, 2047}),
			bitsStored:    12,
			currentPR:     1,
			wantNegatives: false, // all < 2048
			wantMin:       0,
			wantMax:       2047,
		},
		{
			name:          "12-bit PR=1 with negatives [-2048, 2047]",
			pixelData:     make16BitData([]uint16{2048, 2100, 4000, 100}), // 2048+ becomes negative
			bitsStored:    12,
			currentPR:     1,
			wantNegatives: true,
			wantMin:       -2048,
			wantMax:       100,
		},
		{
			name:          "8-bit PR=0 unsigned [0, 255]",
			pixelData:     []byte{0, 50, 100, 200, 255},
			bitsStored:    8,
			currentPR:     0,
			wantNegatives: false,
			wantMin:       0,
			wantMax:       255,
		},
		{
			name:          "8-bit PR=1 with negatives [-128, 127]",
			pixelData:     []byte{128, 200, 255, 0, 50, 127}, // 128+ becomes negative
			bitsStored:    8,
			currentPR:     1,
			wantNegatives: true,
			wantMin:       -128,
			wantMax:       127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNegatives, gotMin, gotMax := DetectActualPixelRepresentation(tt.pixelData, tt.bitsStored, tt.currentPR)

			if gotNegatives != tt.wantNegatives {
				t.Errorf("DetectActualPixelRepresentation() gotNegatives = %v, want %v", gotNegatives, tt.wantNegatives)
			}
			if gotMin != tt.wantMin {
				t.Errorf("DetectActualPixelRepresentation() gotMin = %v, want %v", gotMin, tt.wantMin)
			}
			if gotMax != tt.wantMax {
				t.Errorf("DetectActualPixelRepresentation() gotMax = %v, want %v", gotMax, tt.wantMax)
			}
		})
	}
}

func TestConvertSignedToUnsigned(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		bitsStored int
		expected   []byte
	}{
		{
			name:       "16-bit: -100 to unsigned",
			input:      make16BitSignedData([]int16{-100}),
			bitsStored: 16,
			expected:   make16BitData([]uint16{32768 - 100}),
		},
		{
			name:       "16-bit: 0 to unsigned",
			input:      make16BitSignedData([]int16{0}),
			bitsStored: 16,
			expected:   make16BitData([]uint16{32768}),
		},
		{
			name:       "16-bit: 100 to unsigned",
			input:      make16BitSignedData([]int16{100}),
			bitsStored: 16,
			expected:   make16BitData([]uint16{32768 + 100}),
		},
		{
			name:       "12-bit: -2048 to unsigned",
			input:      make16BitData([]uint16{4096 - 2048}), // -2048 in 12-bit signed
			bitsStored: 12,
			expected:   make16BitData([]uint16{0}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			ConvertSignedToUnsigned(data, tt.bitsStored)

			if len(data) != len(tt.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(data), len(tt.expected))
			}

			for i := range data {
				if data[i] != tt.expected[i] {
					t.Errorf("Byte %d: got 0x%02x, want 0x%02x", i, data[i], tt.expected[i])
				}
			}
		})
	}
}

func TestConvertUnsignedToSigned(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		bitsStored int
		expected   []byte
	}{
		{
			name:       "16-bit: 32668 to -100",
			input:      make16BitData([]uint16{32768 - 100}),
			bitsStored: 16,
			expected:   make16BitSignedData([]int16{-100}),
		},
		{
			name:       "16-bit: 32768 to 0",
			input:      make16BitData([]uint16{32768}),
			bitsStored: 16,
			expected:   make16BitSignedData([]int16{0}),
		},
		{
			name:       "16-bit: 32868 to 100",
			input:      make16BitData([]uint16{32768 + 100}),
			bitsStored: 16,
			expected:   make16BitSignedData([]int16{100}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			ConvertUnsignedToSigned(data, tt.bitsStored)

			if len(data) != len(tt.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(data), len(tt.expected))
			}

			for i := range data {
				if data[i] != tt.expected[i] {
					t.Errorf("Byte %d: got 0x%02x, want 0x%02x", i, data[i], tt.expected[i])
				}
			}
		})
	}
}

func TestRoundTrip_SignedUnsignedConversion(t *testing.T) {
	// Test that converting signed->unsigned->signed gives back original
	original := make16BitSignedData([]int16{-1000, -500, 0, 500, 1000})
	data := make([]byte, len(original))
	copy(data, original)

	ConvertSignedToUnsigned(data, 16)
	ConvertUnsignedToSigned(data, 16)

	for i := range data {
		if data[i] != original[i] {
			t.Errorf("Round-trip failed at byte %d: got 0x%02x, want 0x%02x", i, data[i], original[i])
		}
	}
}

// Helper: make16BitData creates a byte array from uint16 values in little-endian format
func make16BitData(values []uint16) []byte {
	data := make([]byte, len(values)*2)
	for i, val := range values {
		binary.LittleEndian.PutUint16(data[i*2:], val)
	}
	return data
}

// Helper: make16BitSignedData creates a byte array from int16 values in little-endian format
func make16BitSignedData(values []int16) []byte {
	data := make([]byte, len(values)*2)
	for i, val := range values {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(val))
	}
	return data
}
