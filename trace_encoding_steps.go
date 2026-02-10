package main

import (
	"fmt"
	"os"
	"github.com/cocosip/go-dicom-codec/jpeg2000/wavelet"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("我们的编码器 - 详细编码步骤追踪")
	fmt.Println("========================================\n")

	// 读取原始像素数据
	rawData, err := os.ReadFile("D:/pixel_data_raw.bin")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// 解析为int16
	pixels := make([]int16, len(rawData)/2)
	for i := 0; i < len(pixels); i++ {
		pixels[i] = int16(rawData[i*2]) | int16(rawData[i*2+1])<<8
	}

	fmt.Println("【步骤1：原始像素数据】")
	fmt.Println("前20个像素值:")
	for i := 0; i < 20; i++ {
		fmt.Printf("  pixel[%2d] = %6d\n", i, pixels[i])
	}
	fmt.Println()

	// 参数
	width, height := 888, 459
	numLevels := 5
	isSigned := true

	fmt.Println("【步骤2：DC Level Shift】")
	// 转换为int32并应用DC shift（如果需要）
	data := make([]int32, len(pixels))
	for i := range pixels {
		data[i] = int32(pixels[i])
	}

	if isSigned {
		fmt.Println("IsSigned=true, 不应用DC level shift")
		fmt.Println("数据保持不变")
	} else {
		shift := int32(1 << 15) // 2^(16-1)
		fmt.Printf("IsSigned=false, DC shift = %d\n", shift)
		for i := range data {
			data[i] -= shift
		}
	}
	fmt.Println("前20个值（DC shift后）:")
	for i := 0; i < 20; i++ {
		fmt.Printf("  data[%2d] = %6d\n", i, data[i])
	}
	fmt.Println()

	// 复制一份用于DWT
	dwtData := make([]int32, len(data))
	copy(dwtData, data)

	fmt.Println("【步骤3：DWT (5/3可逆小波变换)】")
	fmt.Printf("图像尺寸: %dx%d\n", width, height)
	fmt.Printf("分解层数: %d\n", numLevels)

	// 应用多层DWT
	wavelet.ForwardMultilevelWithParity(dwtData, width, height, numLevels, 0, 0)

	fmt.Println("\nDWT变换后的系数分布:")
	fmt.Println("（注：系数按子带组织: LL在左上，HL/LH/HH分布在其他位置）")

	// 打印LL子带（左上角）的前20个系数
	llWidth := width >> numLevels
	llHeight := height >> numLevels
	fmt.Printf("\nLL子带 (最低频，左上角 %dx%d):\n", llWidth, llHeight)
	fmt.Println("前20个系数:")
	for i := 0; i < 20 && i < llWidth*llHeight; i++ {
		row := i / llWidth
		col := i % llWidth
		idx := row*width + col
		fmt.Printf("  LL[%2d] (row=%d,col=%d) = %6d\n", i, row, col, dwtData[idx])
	}

	// 打印HL1子带（第一层的高-低频）的前几个系数
	hl1Width := width >> (numLevels - 1)
	hl1Height := height >> (numLevels - 1)
	hl1X0 := hl1Width / 2
	fmt.Printf("\nHL1子带 (第1层高-低频，位置起始x=%d, 大小约%dx%d):\n", hl1X0, hl1Width/2, hl1Height)
	fmt.Println("前10个系数:")
	for i := 0; i < 10; i++ {
		row := i / (hl1Width / 2)
		col := i % (hl1Width / 2)
		idx := row*width + (hl1X0 + col)
		if idx < len(dwtData) {
			fmt.Printf("  HL1[%2d] = %6d\n", i, dwtData[idx])
		}
	}

	fmt.Println("\n【步骤4：量化】")
	fmt.Println("对于无损编码（可逆5/3小波）:")
	fmt.Println("  - 不应用浮点量化")
	fmt.Println("  - 系数保持整数形式")
	fmt.Println("  - QCD中的exponent值用于T1编码的范围计算")

	fmt.Println("\n【步骤5：T1编码前的准备】")
	fmt.Println("T1编码（EBCOT）将DWT系数划分为code-blocks (64x64)")
	fmt.Println("每个code-block独立进行位平面编码")

	// 计算T1_NMSEDEC_FRACBITS缩放
	const T1_NMSEDEC_FRACBITS = 6
	fmt.Printf("\nT1_NMSEDEC_FRACBITS缩放: << %d\n", T1_NMSEDEC_FRACBITS)
	fmt.Println("前10个LL系数（左移6位后）:")
	for i := 0; i < 10; i++ {
		row := i / llWidth
		col := i % llWidth
		idx := row*width + col
		scaled := dwtData[idx] << T1_NMSEDEC_FRACBITS
		fmt.Printf("  LL[%2d] = %6d -> %6d (scaled)\n", i, dwtData[idx], scaled)
	}

	fmt.Println("\n========================================")
	fmt.Println("编码步骤追踪完成")
	fmt.Println("========================================")
	fmt.Println("\n提示：现在运行 encode_openjpeg.exe 查看OpenJPEG的")
	fmt.Println("      编码过程输出，对比每个步骤的数值")
}
