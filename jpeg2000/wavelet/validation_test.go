package wavelet

import (
	"math"
	"testing"
)

// TestDWT53Reversibility 验证 5/3 小波变换的完全可逆性
// 这是无损压缩的核心要求：forward -> inverse 必须完全恢复原始数据（误差=0）
// 参考：ISO/IEC 15444-1:2019 Annex C.1
func TestDWT53Reversibility(t *testing.T) {
	tests := []struct {
		name  string
		input []int32
	}{
		{
			name:  "单调递增序列",
			input: []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
		{
			name:  "常数序列",
			input: []int32{100, 100, 100, 100, 100, 100, 100, 100},
		},
		{
			name:  "交替符号",
			input: []int32{10, -10, 10, -10, 10, -10, 10, -10},
		},
		{
			name:  "随机值",
			input: []int32{123, -456, 789, -12, 345, -678, 901, -234},
		},
		{
			name:  "边界值",
			input: []int32{0, 32767, -32768, 1, -1, 1000, -1000, 0},
		},
		{
			name:  "奇数长度",
			input: []int32{1, 2, 3, 4, 5},
		},
		{
			name:  "偶数长度",
			input: []int32{1, 2, 3, 4, 5, 6},
		},
		{
			name:  "最小长度",
			input: []int32{42, 24},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始数据
			original := make([]int32, len(tt.input))
			copy(original, tt.input)

			// 创建工作副本
			data := make([]int32, len(tt.input))
			copy(data, tt.input)

			// 正向变换
			Forward53_1D(data)

			// 逆向变换
			Inverse53_1D(data)

			// 验证完全可逆（误差必须为 0）
			for i := range original {
				if original[i] != data[i] {
					t.Errorf("位置 %d 不可逆: 期望 %d, 得到 %d, 误差 %d",
						i, original[i], data[i], data[i]-original[i])
				}
			}
		})
	}
}

// TestDWT53Reversibility2D 验证 2D 变换的可逆性
func TestDWT53Reversibility2D(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"奇数尺寸5x5", 5, 5},
		{"矩形8x4", 8, 4},
		{"矩形4x8", 4, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height
			original := make([]int32, size)

			// 生成测试数据：渐变模式
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					original[y*tt.width+x] = int32((x + y*tt.width) * 10)
				}
			}

			// 创建工作副本
			data := make([]int32, size)
			copy(data, original)

			// 正向变换
			Forward53_2D(data, tt.width, tt.height)

			// 逆向变换
			Inverse53_2D(data, tt.width, tt.height)

			// 验证完全可逆
			errorCount := 0
			maxError := int32(0)
			for i := range original {
				if original[i] != data[i] {
					errorCount++
					err := abs32(data[i] - original[i])
					if err > maxError {
						maxError = err
					}
				}
			}

			if errorCount > 0 {
				t.Errorf("发现 %d 个误差，最大误差: %d", errorCount, maxError)
			}
		})
	}
}

// TestDWT97Precision 验证 9/7 小波变换的精度
// 9/7 是不可逆变换（有损），但重构误差应该很小
// 参考：ISO/IEC 15444-1:2019 Annex F.3
func TestDWT97Precision(t *testing.T) {
	tests := []struct {
		name           string
		input          []float64
		maxRelativeErr float64
	}{
		{
			name:           "单调递增",
			input:          []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			maxRelativeErr: 1e-10, // 相对误差 < 10^-10
		},
		{
			name:           "常数",
			input:          []float64{100, 100, 100, 100, 100, 100, 100, 100},
			maxRelativeErr: 1e-10,
		},
		{
			name:           "正弦波",
			input:          generateSineWave(16),
			maxRelativeErr: 1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存原始数据
			original := make([]float64, len(tt.input))
			copy(original, tt.input)

			// 创建工作副本
			data := make([]float64, len(tt.input))
			copy(data, tt.input)

			// 正向变换
			Forward97_1D(data)

			// 逆向变换
			Inverse97_1D(data)

			// 验证精度
			for i := range original {
				var relErr float64
				if original[i] != 0 {
					relErr = math.Abs((data[i] - original[i]) / original[i])
				} else {
					relErr = math.Abs(data[i])
				}

				if relErr > tt.maxRelativeErr {
					t.Errorf("位置 %d 精度不足: 期望 %.15f, 得到 %.15f, 相对误差 %e",
						i, original[i], data[i], relErr)
				}
			}
		})
	}
}

// TestDWT97Precision2D 验证 2D 9/7 变换的精度
func TestDWT97Precision2D(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"8x8", 8, 8},
		{"16x16", 16, 16},
		{"32x32", 32, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height
			original := make([]float64, size)

			// 生成测试数据：2D 正弦模式
			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					fx := 2.0 * math.Pi * float64(x) / float64(tt.width)
					fy := 2.0 * math.Pi * float64(y) / float64(tt.height)
					original[y*tt.width+x] = 100 * (math.Sin(fx) + math.Cos(fy))
				}
			}

			// 创建工作副本
			data := make([]float64, size)
			copy(data, original)

			// 正向变换
			Forward97_2D(data, tt.width, tt.height)

			// 逆向变换
			Inverse97_2D(data, tt.width, tt.height)

			// 计算 RMSE（均方根误差）
			sumSqErr := 0.0
			maxAbsErr := 0.0
			for i := range original {
				err := math.Abs(data[i] - original[i])
				sumSqErr += err * err
				if err > maxAbsErr {
					maxAbsErr = err
				}
			}
			rmse := math.Sqrt(sumSqErr / float64(size))

			t.Logf("RMSE: %e, 最大绝对误差: %e", rmse, maxAbsErr)

			// RMSE 应该非常小（< 10^-9）
			if rmse > 1e-9 {
				t.Errorf("RMSE 过大: %e (期望 < 1e-9)", rmse)
			}
		})
	}
}

// TestDWT53MultiLevel 测试多级分解的可逆性
func TestDWT53MultiLevel(t *testing.T) {
	width := 64
	height := 64
	levels := 3

	original := make([]int32, width*height)
	for i := range original {
		original[i] = int32(i % 1000)
	}

	data := make([]int32, width*height)
	copy(data, original)

	// 多级正向变换
	ForwardMultilevel(data, width, height, levels)

	// 多级逆向变换
	InverseMultilevel(data, width, height, levels)

	// 验证可逆性
	errorCount := 0
	for i := range original {
		if original[i] != data[i] {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("多级变换不可逆，发现 %d 个误差", errorCount)
	}
}

// TestDWT97MultiLevel 测试多级 9/7 变换的精度
func TestDWT97MultiLevel(t *testing.T) {
	width := 64
	height := 64
	levels := 3

	original := make([]float64, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			original[y*width+x] = float64((x + y) * 10)
		}
	}

	data := make([]float64, width*height)
	copy(data, original)

	// 多级正向变换
	ForwardMultilevel97(data, width, height, levels)

	// 多级逆向变换
	InverseMultilevel97(data, width, height, levels)

	// 计算 RMSE
	sumSqErr := 0.0
	for i := range original {
		err := data[i] - original[i]
		sumSqErr += err * err
	}
	rmse := math.Sqrt(sumSqErr / float64(len(original)))

	t.Logf("多级 9/7 RMSE: %e", rmse)

	if rmse > 1e-8 {
		t.Errorf("多级 9/7 RMSE 过大: %e", rmse)
	}
}

// TestDWTEdgeCasesValidation 测试边界情况
func TestDWTEdgeCasesValidation(t *testing.T) {
	t.Run("长度为1的信号", func(t *testing.T) {
		data := []int32{42}
		original := []int32{42}

		Forward53_1D(data)
		Inverse53_1D(data)

		if data[0] != original[0] {
			t.Errorf("长度1信号处理失败")
		}
	})

	t.Run("长度为2的信号", func(t *testing.T) {
		data := []int32{10, 20}
		original := []int32{10, 20}

		Forward53_1D(data)
		Inverse53_1D(data)

		if data[0] != original[0] || data[1] != original[1] {
			t.Errorf("长度2信号处理失败")
		}
	})

	t.Run("1x1图像", func(t *testing.T) {
		data := []int32{42}
		original := []int32{42}

		Forward53_2D(data, 1, 1)
		Inverse53_2D(data, 1, 1)

		if data[0] != original[0] {
			t.Errorf("1x1图像处理失败")
		}
	})
}

// 辅助函数

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func generateSineWave(n int) []float64 {
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = 100 * math.Sin(2*math.Pi*float64(i)/float64(n))
	}
	return result
}
