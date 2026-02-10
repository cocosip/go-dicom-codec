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

	// 获取解码后的像素数据
	pixelData := decoder.GetPixelData()

	fmt.Printf("Decoded successfully!\n")
	fmt.Printf("Data length: %d bytes\n\n", len(pixelData))

	// 将字节数据转换为int16
	pixels := make([]int16, len(pixelData)/2)
	for i := 0; i < len(pixels); i++ {
		// Little-endian int16 (matching GetPixelData() output format)
		pixels[i] = int16(pixelData[i*2]) | int16(pixelData[i*2+1])<<8
	}

	fmt.Println("First 20 pixels from OpenJPEG encoded file:")
	for i := 0; i < 20 && i < len(pixels); i++ {
		fmt.Printf("  pixel[%2d] = %6d\n", i, pixels[i])
	}
	fmt.Println()

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

		// 计算比率
		if len(pixels) > 0 && pixels[0] != 0 {
			ratio := float64(rawPixels[0]) / float64(pixels[0])
			fmt.Printf("Ratio analysis:\n")
			fmt.Printf("  rawPixels[0] / pixels[0] = %d / %d = %.4f\n", rawPixels[0], pixels[0], ratio)

			// Check powers of 2
			for i := 0; i <= 10; i++ {
				pow2 := 1 << i
				fmt.Printf("  2^%d = %d\n", i, pow2)
			}
		}
	}
}
