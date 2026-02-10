package main

import (
	"fmt"
	"os"
	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	// 读取OpenJPEG编码的J2K文件
	data, err := os.ReadFile("D:/encoded_openjpeg.j2k")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	fmt.Printf("J2K file size: %d bytes\n\n", len(data))

	// 解码
	decoder := jpeg2000.NewDecoder()
	err = decoder.Decode(data)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// 直接获取int32数据，绕过GetPixelData()的转换
	imageData := decoder.GetImageData()

	fmt.Printf("Decoded successfully!\n")
	fmt.Printf("Components: %d\n\n", len(imageData))

	if len(imageData) > 0 && len(imageData[0]) > 0 {
		fmt.Println("First 20 pixels (int32 from decoder.GetImageData()):")
		for i := 0; i < 20 && i < len(imageData[0]); i++ {
			fmt.Printf("  pixel[%2d] = %6d\n", i, imageData[0][i])
		}
		fmt.Println()
	}

	// 读取原始像素值进行对比
	rawData, err := os.ReadFile("D:/pixel_data_raw.bin")
	if err == nil {
		rawPixels := make([]int16, len(rawData)/2)
		for i := 0; i < len(rawPixels); i++ {
			// Little-endian int16
			rawPixels[i] = int16(rawData[i*2]) | int16(rawData[i*2+1])<<8
		}

		fmt.Println("First 20 pixels from original raw data:")
		for i := 0; i < 20 && i < len(rawPixels); i++ {
			fmt.Printf("  pixel[%2d] = %6d\n", i, rawPixels[i])
		}
		fmt.Println()

		// 计算比率和差异
		if len(imageData) > 0 && len(imageData[0]) > 0 {
			fmt.Println("Comparison:")
			for i := 0; i < 10 && i < len(imageData[0]) && i < len(rawPixels); i++ {
				decoded := imageData[0][i]
				original := int32(rawPixels[i])
				diff := original - decoded
				var ratio float64
				if decoded != 0 {
					ratio = float64(original) / float64(decoded)
				}
				fmt.Printf("  pixel[%d]: original=%d, decoded=%d, diff=%d, ratio=%.4f\n",
					i, original, decoded, diff, ratio)
			}
			fmt.Println()

			// 检查是否是简单的位移关系
			if len(imageData[0]) > 0 && imageData[0][0] != 0 {
				original := int32(rawPixels[0])
				decoded := imageData[0][0]
				fmt.Println("Bit shift analysis:")
				for shift := 0; shift <= 8; shift++ {
					shifted := decoded << shift
					fmt.Printf("  decoded << %d = %d (diff from original: %d)\n",
						shift, shifted, original-shifted)
				}
			}
		}
	}
}
