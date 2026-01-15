// Debug round-trip encoding/decoding
// Usage: go run debug_roundtrip.go <input.bin> <output.j2k> <width> <height> <components> <bitdepth>

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	if len(os.Args) < 7 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.bin> <output.j2k> <width> <height> <components> <bitdepth>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]
	width, _ := strconv.Atoi(os.Args[3])
	height, _ := strconv.Atoi(os.Args[4])
	numComponents, _ := strconv.Atoi(os.Args[5])
	bitDepth, _ := strconv.Atoi(os.Args[6])

	fmt.Println("Round-Trip Debug Tool")
	fmt.Println("=" + repeatString("=", 70))

	// Read original
	originalData, _ := os.ReadFile(inputFile)

	// Read encoded
	encodedData, _ := os.ReadFile(outputFile)

	// Decode
	decoder := jpeg2000.NewDecoder()
	err := decoder.Decode(encodedData)
	if err != nil {
		fmt.Printf("Decode failed: %v\n", err)
		os.Exit(1)
	}

	componentData := decoder.GetImageData()

	fmt.Printf("\nOriginal file analysis:\n")
	fmt.Printf("  Size: %d bytes\n", len(originalData))
	fmt.Printf("  First 32 bytes (hex): ")
	for i := 0; i < 32 && i < len(originalData); i++ {
		fmt.Printf("%02X ", originalData[i])
	}
	fmt.Println()

	if bitDepth > 8 {
		fmt.Printf("\n  As 16-bit little-endian values:\n")
		fmt.Printf("    First 10 values: ")
		for i := 0; i < 20 && i+1 < len(originalData); i += 2 {
			val := binary.LittleEndian.Uint16(originalData[i:])
			fmt.Printf("%d ", val)
		}
		fmt.Println()
	} else {
		fmt.Printf("\n  As 8-bit values:\n")
		fmt.Printf("    First 20 values: ")
		for i := 0; i < 20 && i < len(originalData); i++ {
			fmt.Printf("%d ", originalData[i])
		}
		fmt.Println()
	}

	fmt.Printf("\nDecoded component data:\n")
	fmt.Printf("  Components: %d\n", len(componentData))
	fmt.Printf("  Component 0 length: %d\n", len(componentData[0]))
	fmt.Printf("  First 10 values: ")
	for i := 0; i < 10 && i < len(componentData[0]); i++ {
		fmt.Printf("%d ", componentData[0][i])
	}
	fmt.Println()

	// Try different conversion methods
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("Testing different byte conversion methods:")
	fmt.Println(repeatString("=", 70))

	// Method 1: Interleaved
	fmt.Println("\nMethod 1: Interleaved byte-by-byte")
	testMethod1(originalData, componentData, width, height, numComponents, bitDepth)

	// Method 2: Planar
	fmt.Println("\nMethod 2: Planar (component-by-component)")
	testMethod2(originalData, componentData, width, height, numComponents, bitDepth)

	// Method 3: Try big-endian for 16-bit
	if bitDepth > 8 {
		fmt.Println("\nMethod 3: Big-endian 16-bit interleaved")
		testMethod3(originalData, componentData, width, height, numComponents)
	}
}

func testMethod1(original []byte, components [][]int32, width, height, numComponents, bitDepth int) {
	totalPixels := width * height
	var reconstructed []byte

	if bitDepth <= 8 {
		reconstructed = make([]byte, totalPixels*numComponents)
		idx := 0
		for i := 0; i < totalPixels; i++ {
			for c := 0; c < numComponents; c++ {
				reconstructed[idx] = byte(components[c][i])
				idx++
			}
		}
	} else {
		reconstructed = make([]byte, totalPixels*numComponents*2)
		idx := 0
		for i := 0; i < totalPixels; i++ {
			for c := 0; c < numComponents; c++ {
				val := uint16(components[c][i])
				reconstructed[idx] = byte(val & 0xFF)
				reconstructed[idx+1] = byte((val >> 8) & 0xFF)
				idx += 2
			}
		}
	}

	compareBytes(original, reconstructed, "Interleaved")
}

func testMethod2(original []byte, components [][]int32, width, height, numComponents, bitDepth int) {
	totalPixels := width * height
	var reconstructed []byte

	if bitDepth <= 8 {
		reconstructed = make([]byte, totalPixels*numComponents)
		idx := 0
		for c := 0; c < numComponents; c++ {
			for i := 0; i < totalPixels; i++ {
				reconstructed[idx] = byte(components[c][i])
				idx++
			}
		}
	} else {
		reconstructed = make([]byte, totalPixels*numComponents*2)
		idx := 0
		for c := 0; c < numComponents; c++ {
			for i := 0; i < totalPixels; i++ {
				val := uint16(components[c][i])
				reconstructed[idx] = byte(val & 0xFF)
				reconstructed[idx+1] = byte((val >> 8) & 0xFF)
				idx += 2
			}
		}
	}

	compareBytes(original, reconstructed, "Planar")
}

func testMethod3(original []byte, components [][]int32, width, height, numComponents int) {
	totalPixels := width * height
	reconstructed := make([]byte, totalPixels*numComponents*2)
	idx := 0

	for i := 0; i < totalPixels; i++ {
		for c := 0; c < numComponents; c++ {
			val := uint16(components[c][i])
			reconstructed[idx] = byte((val >> 8) & 0xFF)   // High byte first
			reconstructed[idx+1] = byte(val & 0xFF)        // Low byte second
			idx += 2
		}
	}

	compareBytes(original, reconstructed, "Big-endian")
}

func compareBytes(original, reconstructed []byte, method string) {
	minLen := len(original)
	if len(reconstructed) < minLen {
		minLen = len(reconstructed)
	}

	matches := 0
	for i := 0; i < minLen; i++ {
		if original[i] == reconstructed[i] {
			matches++
		}
	}

	matchPct := float64(matches) / float64(minLen) * 100

	fmt.Printf("  %s: %d/%d bytes match (%.2f%%)\n", method, matches, minLen, matchPct)

	if matches == minLen {
		fmt.Printf("  ✓✓✓ PERFECT MATCH! This is the correct format!\n")
	} else if matchPct > 95 {
		fmt.Printf("  ✓ Very close! (>95%%)\n")
	}

	// Show first few mismatches
	if matches < minLen {
		fmt.Printf("  First 5 differences:\n")
		shown := 0
		for i := 0; i < minLen && shown < 5; i++ {
			if original[i] != reconstructed[i] {
				fmt.Printf("    Byte %d: original=0x%02X, reconstructed=0x%02X\n",
					i, original[i], reconstructed[i])
				shown++
			}
		}
	}
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
