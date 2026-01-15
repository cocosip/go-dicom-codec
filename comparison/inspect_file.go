// Inspect raw file to detect format
// Usage: go run inspect_file.go <file>

package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", filename)
	fmt.Printf("Size: %d bytes (%.2f KB)\n\n", len(data), float64(len(data))/1024)

	// Show first 256 bytes in hex
	fmt.Println("First 256 bytes (hex):")
	fmt.Println(repeatString("=", 70))
	for i := 0; i < 256 && i < len(data); i += 16 {
		fmt.Printf("%08X: ", i)
		for j := 0; j < 16 && i+j < len(data); j++ {
			fmt.Printf("%02X ", data[i+j])
		}
		fmt.Print(" | ")
		for j := 0; j < 16 && i+j < len(data); j++ {
			c := data[i+j]
			if c >= 32 && c <= 126 {
				fmt.Printf("%c", c)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}

	// Check for common file signatures
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("File Format Detection:")
	fmt.Println(repeatString("=", 70))

	if len(data) >= 2 {
		// JPEG 2000
		if data[0] == 0xFF && data[1] == 0x4F {
			fmt.Println("✓ Detected: JPEG 2000 codestream (0xFF4F = SOC marker)")
		}
		// JPEG
		if data[0] == 0xFF && data[1] == 0xD8 {
			fmt.Println("✓ Detected: JPEG image (0xFFD8)")
		}
	}

	if len(data) >= 4 {
		// PNG
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			fmt.Println("✓ Detected: PNG image")
		}
		// TIFF (Intel byte order)
		if data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00 {
			fmt.Println("✓ Detected: TIFF image (little-endian)")
		}
		// TIFF (Motorola byte order)
		if data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A {
			fmt.Println("✓ Detected: TIFF image (big-endian)")
		}
	}

	if len(data) >= 8 {
		// Check for DICOM
		if string(data[128:132]) == "DICM" {
			fmt.Println("✓ Detected: DICOM file")
		}
	}

	if len(data) >= 12 {
		// JPEG 2000 JP2
		if data[4] == 0x6A && data[5] == 0x50 && data[6] == 0x20 && data[7] == 0x20 {
			fmt.Println("✓ Detected: JPEG 2000 JP2 format")
		}
	}

	// Check if it looks like raw pixel data
	fmt.Println("\nRaw Pixel Data Analysis:")
	fmt.Println(repeatString("-", 70))

	// Calculate basic statistics
	var sum uint64
	var min, max byte = 255, 0
	histogram := make([]int, 256)

	for _, b := range data[:min256(len(data), 10000)] { // Sample first 10000 bytes
		sum += uint64(b)
		if b < min {
			min = b
		}
		if b > max {
			max = b
		}
		histogram[b]++
	}

	sampleSize := min256(len(data), 10000)
	avg := float64(sum) / float64(sampleSize)

	fmt.Printf("Sample size: %d bytes\n", sampleSize)
	fmt.Printf("Min value: %d (0x%02X)\n", min, min)
	fmt.Printf("Max value: %d (0x%02X)\n", max, max)
	fmt.Printf("Average: %.2f\n", avg)

	// Count zeros and 0xFF
	zeros := histogram[0]
	ones := histogram[255]
	fmt.Printf("Zero bytes: %d (%.2f%%)\n", zeros, float64(zeros)/float64(sampleSize)*100)
	fmt.Printf("0xFF bytes: %d (%.2f%%)\n", ones, float64(ones)/float64(sampleSize)*100)

	// Check for 16-bit patterns
	if len(data) >= 4 {
		fmt.Println("\n16-bit Analysis (first 4 values):")
		// Little-endian
		v0_le := binary.LittleEndian.Uint16(data[0:2])
		v1_le := binary.LittleEndian.Uint16(data[2:4])
		fmt.Printf("  Little-endian: %d, %d\n", v0_le, v1_le)

		// Big-endian
		v0_be := binary.BigEndian.Uint16(data[0:2])
		v1_be := binary.BigEndian.Uint16(data[2:4])
		fmt.Printf("  Big-endian: %d, %d\n", v0_be, v1_be)
	}

	// Check for repeating patterns
	fmt.Println("\nPattern Analysis:")
	if hasRepeatingPattern(data[:min256(len(data), 1000)], 2) {
		fmt.Println("  ✓ Detected repeating 2-byte pattern (possibly interleaved)")
	}
	if hasRepeatingPattern(data[:min256(len(data), 1000)], 3) {
		fmt.Println("  ✓ Detected repeating 3-byte pattern (possibly RGB)")
	}
	if hasRepeatingPattern(data[:min256(len(data), 1000)], 4) {
		fmt.Println("  ✓ Detected repeating 4-byte pattern (possibly RGBA)")
	}
}

func min256(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hasRepeatingPattern(data []byte, period int) bool {
	if len(data) < period*3 {
		return false
	}

	matches := 0
	for i := 0; i < len(data)-period; i++ {
		if data[i] == data[i+period] {
			matches++
		}
	}

	// If more than 60% match, consider it a pattern
	return float64(matches)/float64(len(data)-period) > 0.6
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
