package wavelet

import (
	"math"
	"testing"
)

// TestForwardInverse97_1D tests 1D forward and inverse 9/7 transform
func TestForwardInverse97_1D(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"Size 4", 4},
		{"Size 8", 8},
		{"Size 16", 16},
		{"Size 32", 32},
		{"Size 64", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test signal
			original := make([]float64, tt.size)
			for i := range original {
				original[i] = float64(i%10) + 0.5
			}

			// Make a copy
			data := make([]float64, tt.size)
			copy(data, original)

			// Forward transform
			Forward97_1D(data)

			// Inverse transform
			Inverse97_1D(data)

			// Check reconstruction (with tolerance for floating point)
			maxError := 0.0
			for i := range data {
				err := math.Abs(data[i] - original[i])
				if err > maxError {
					maxError = err
				}
			}

			// 9/7 is irreversible, but error should be very small
			if maxError > 1e-10 {
				t.Errorf("Reconstruction error too large: %e", maxError)
			}
		})
	}
}

// TestForwardInverse97_2D tests 2D forward and inverse 9/7 transform
func TestForwardInverse97_2D(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"Non-square 8x16", 8, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height

			// Create test image
			original := make([]float64, size)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					original[y*tt.width+x] = float64((x+y)%10) + 0.5
				}
			}

			// Make a copy
			data := make([]float64, size)
			copy(data, original)

			// Forward transform
			Forward97_2D(data, tt.width, tt.height)

			// Inverse transform
			Inverse97_2D(data, tt.width, tt.height)

			// Check reconstruction
			maxError := 0.0
			for i := range data {
				err := math.Abs(data[i] - original[i])
				if err > maxError {
					maxError = err
				}
			}

			if maxError > 1e-10 {
				t.Errorf("Reconstruction error too large: %e", maxError)
			}
		})
	}
}

// TestForwardInverseMultilevel97 tests multilevel 9/7 transform
func TestForwardInverseMultilevel97(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		levels int
	}{
		{"16x16 1-level", 16, 16, 1},
		{"32x32 2-level", 32, 32, 2},
		{"64x64 3-level", 64, 64, 3},
		{"128x128 4-level", 128, 128, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height

			// Create test image with gradient pattern
			original := make([]float64, size)
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					original[y*tt.width+x] = float64(x+y) / float64(tt.width+tt.height)
				}
			}

			// Make a copy
			data := make([]float64, size)
			copy(data, original)

			// Forward multilevel transform
			ForwardMultilevel97(data, tt.width, tt.height, tt.levels)

			// Inverse multilevel transform
			InverseMultilevel97(data, tt.width, tt.height, tt.levels)

			// Check reconstruction
			maxError := 0.0
			avgError := 0.0
			for i := range data {
				err := math.Abs(data[i] - original[i])
				if err > maxError {
					maxError = err
				}
				avgError += err
			}
			avgError /= float64(len(data))

			if maxError > 1e-9 {
				t.Errorf("Max reconstruction error too large: %e", maxError)
			}
			if avgError > 1e-10 {
				t.Errorf("Avg reconstruction error too large: %e", avgError)
			}
		})
	}
}

// TestSubbandEnergy97 tests that energy is concentrated in LL subband
func TestSubbandEnergy97(t *testing.T) {
	width, height := 32, 32
	size := width * height

	// Create smooth gradient image
	data := make([]float64, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			data[y*width+x] = float64(x+y) / 2.0
		}
	}

	// Forward transform
	Forward97_2D(data, width, height)

	// Calculate energy in each subband
	nL := (width + 1) / 2
	mL := (height + 1) / 2

	energyLL := 0.0
	energyHL := 0.0
	energyLH := 0.0
	energyHH := 0.0

	// LL subband (top-left)
	for y := 0; y < mL; y++ {
		for x := 0; x < nL; x++ {
			val := data[y*width+x]
			energyLL += val * val
		}
	}

	// HL subband (top-right)
	for y := 0; y < mL; y++ {
		for x := nL; x < width; x++ {
			val := data[y*width+x]
			energyHL += val * val
		}
	}

	// LH subband (bottom-left)
	for y := mL; y < height; y++ {
		for x := 0; x < nL; x++ {
			val := data[y*width+x]
			energyLH += val * val
		}
	}

	// HH subband (bottom-right)
	for y := mL; y < height; y++ {
		for x := nL; x < width; x++ {
			val := data[y*width+x]
			energyHH += val * val
		}
	}

	totalEnergy := energyLL + energyHL + energyLH + energyHH

	// For smooth images, most energy should be in LL
	llRatio := energyLL / totalEnergy

	if llRatio < 0.95 {
		t.Logf("LL energy ratio: %.4f", llRatio)
		t.Logf("Energies - LL: %.2f, HL: %.2f, LH: %.2f, HH: %.2f",
			energyLL, energyHL, energyLH, energyHH)
	}

	// At least some energy concentration expected
	if llRatio < 0.5 {
		t.Errorf("LL energy ratio too low: %.4f (expected > 0.5)", llRatio)
	}
}

// TestEdgeCases97 tests edge cases for 9/7 transform
func TestEdgeCases97(t *testing.T) {
	t.Run("Size 1", func(t *testing.T) {
		data := []float64{42.5}
		Forward97_1D(data)
		// Size 1 should remain unchanged
		if data[0] != 42.5 {
			t.Errorf("Size 1 changed: got %f, want 42.5", data[0])
		}
	})

	t.Run("Size 2", func(t *testing.T) {
		original := []float64{10.5, 20.5}
		data := make([]float64, 2)
		copy(data, original)

		Forward97_1D(data)
		Inverse97_1D(data)

		for i := range data {
			if math.Abs(data[i]-original[i]) > 1e-10 {
				t.Errorf("Reconstruction failed at %d: got %f, want %f",
					i, data[i], original[i])
			}
		}
	})

	t.Run("All zeros", func(t *testing.T) {
		data := make([]float64, 16)
		Forward97_1D(data)

		// All zeros should remain zeros
		for i, v := range data {
			if math.Abs(v) > 1e-10 {
				t.Errorf("Zero preservation failed at %d: got %f", i, v)
			}
		}
	})

	t.Run("Constant signal", func(t *testing.T) {
		data := make([]float64, 16)
		for i := range data {
			data[i] = 100.5
		}

		Forward97_1D(data)

		// High-pass coefficients should be near zero
		nL := (len(data) + 1) / 2
		for i := nL; i < len(data); i++ {
			if math.Abs(data[i]) > 1e-6 {
				t.Errorf("High-pass coefficient should be near zero: got %f", data[i])
			}
		}
	})
}

// TestConversionFunctions tests int32 <-> float64 conversion
func TestConversionFunctions(t *testing.T) {
	t.Run("Int32 to Float64", func(t *testing.T) {
		input := []int32{-100, -1, 0, 1, 100, 1000}
		output := ConvertInt32ToFloat64(input)

		if len(output) != len(input) {
			t.Fatalf("Length mismatch: got %d, want %d", len(output), len(input))
		}

		for i := range input {
			expected := float64(input[i])
			if output[i] != expected {
				t.Errorf("Conversion failed at %d: got %f, want %f",
					i, output[i], expected)
			}
		}
	})

	t.Run("Float64 to Int32", func(t *testing.T) {
		input := []float64{-100.7, -1.4, 0.0, 1.5, 100.3, 1000.8}
		expected := []int32{-101, -1, 0, 2, 100, 1001}

		output := ConvertFloat64ToInt32(input)

		if len(output) != len(input) {
			t.Fatalf("Length mismatch: got %d, want %d", len(output), len(input))
		}

		for i := range input {
			if output[i] != expected[i] {
				t.Errorf("Conversion failed at %d: got %d, want %d",
					i, output[i], expected[i])
			}
		}
	})

	t.Run("Round trip", func(t *testing.T) {
		original := []int32{-50, -10, 0, 10, 50, 100}
		float := ConvertInt32ToFloat64(original)
		result := ConvertFloat64ToInt32(float)

		for i := range original {
			if result[i] != original[i] {
				t.Errorf("Round trip failed at %d: got %d, want %d",
					i, result[i], original[i])
			}
		}
	})
}

// TestLossyNature97 tests that 9/7 transform is lossy when using int32 conversion
func TestLossyNature97(t *testing.T) {
	width, height := 32, 32
	size := width * height

	// Create int32 test data
	original := make([]int32, size)
	for i := range original {
		original[i] = int32(i % 256)
	}

	// Convert to float64
	data := ConvertInt32ToFloat64(original)

	// Apply transform
	ForwardMultilevel97(data, width, height, 2)
	InverseMultilevel97(data, width, height, 2)

	// Convert back to int32
	result := ConvertFloat64ToInt32(data)

	// Calculate error
	differences := 0
	maxError := int32(0)
	for i := range original {
		diff := result[i] - original[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			differences++
		}
		if diff > maxError {
			maxError = diff
		}
	}

	// Some differences expected due to rounding, but should be small
	t.Logf("Pixels with differences: %d / %d", differences, size)
	t.Logf("Max error: %d", maxError)

	// Error should be very small (typically 0-1 due to rounding)
	if maxError > 2 {
		t.Errorf("Max error too large: %d (expected <= 2)", maxError)
	}
}

// Benchmark97_1D benchmarks 1D forward transform
func Benchmark97_1D(b *testing.B) {
	sizes := []int{64, 256, 1024}

	for _, size := range sizes {
		b.Run("", func(b *testing.B) {
			data := make([]float64, size)
			for i := range data {
				data[i] = float64(i % 100)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward97_1D(data)
			}
		})
	}
}

// Benchmark97_2D benchmarks 2D forward transform
func Benchmark97_2D(b *testing.B) {
	tests := []struct {
		width  int
		height int
	}{
		{64, 64},
		{256, 256},
		{512, 512},
	}

	for _, tt := range tests {
		b.Run("", func(b *testing.B) {
			size := tt.width * tt.height
			data := make([]float64, size)
			for i := range data {
				data[i] = float64(i % 100)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Forward97_2D(data, tt.width, tt.height)
			}
		})
	}
}
