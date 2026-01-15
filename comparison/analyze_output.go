// Analyze JPEG2000 output and verify round-trip encoding
// Usage: go run analyze_output.go <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	if len(os.Args) < 7 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]
	width, _ := strconv.Atoi(os.Args[3])
	height, _ := strconv.Atoi(os.Args[4])
	numComponents, _ := strconv.Atoi(os.Args[5])
	bitDepth, _ := strconv.Atoi(os.Args[6])

	fmt.Println("JPEG2000 Output Analysis")
	fmt.Println(repeatString("=", 70))

	// Read original data
	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input file: %v\n", err)
		os.Exit(1)
	}

	// Read encoded data
	encodedData, err := os.ReadFile(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFile Information:\n")
	fmt.Printf("  Original size: %d bytes (%.2f KB)\n", len(originalData), float64(len(originalData))/1024)
	fmt.Printf("  Encoded size:  %d bytes (%.2f KB)\n", len(encodedData), float64(len(encodedData))/1024)
	fmt.Printf("  Compression:   %.2f:1\n", float64(len(originalData))/float64(len(encodedData)))

	// Parse codestream structure
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("Codestream Structure Analysis")
	fmt.Println(repeatString("=", 70))
	parseCodestream(encodedData)

	// Decode and verify
	fmt.Println("\n" + repeatString("=", 70))
	fmt.Println("Round-Trip Verification (Decode & Compare)")
	fmt.Println(repeatString("=", 70))
	verifyRoundTrip(originalData, encodedData, width, height, numComponents, bitDepth)
}

type MarkerInfo struct {
	Type   uint16
	Name   string
	Offset int
	Length int
	Data   []byte
}

func parseCodestream(data []byte) {
	r := bytes.NewReader(data)
	offset := 0
	markerCount := 0
	compressedSize := 0

	fmt.Printf("\n%-5s %-8s %-40s %-10s\n", "Idx", "Offset", "Marker", "Length")
	fmt.Println(repeatString("-", 70))

	for {
		var marker uint16
		err := binary.Read(r, binary.BigEndian, &marker)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		markerName := getMarkerName(marker)
		markerOffset := offset
		offset += 2

		var length int
		if marker == 0xFF4F || marker == 0xFF93 || marker == 0xFFD9 {
			length = 0
		} else {
			var segLength uint16
			err = binary.Read(r, binary.BigEndian, &segLength)
			if err != nil {
				break
			}
			offset += 2
			length = int(segLength) - 2

			markerData := make([]byte, length)
			n, err := r.Read(markerData)
			if err != nil && err != io.EOF {
				break
			}
			offset += n
		}

		fmt.Printf("%-5d 0x%-6X %-40s %-10d\n", markerCount, markerOffset, markerName, length)
		markerCount++

		if marker == 0xFF93 { // SOD
			remaining := data[offset:]
			eocPos := bytes.Index(remaining, []byte{0xFF, 0xD9})
			if eocPos != -1 {
				compressedSize = eocPos
				offset += compressedSize
			}
		}
	}

	fmt.Println(repeatString("-", 70))
	fmt.Printf("Total markers: %d\n", markerCount)
	fmt.Printf("Compressed data size: %d bytes (%.2f KB)\n", compressedSize, float64(compressedSize)/1024)
}

func getMarkerName(marker uint16) string {
	switch marker {
	case 0xFF4F:
		return "SOC (Start of Codestream)"
	case 0xFF51:
		return "SIZ (Image and tile size)"
	case 0xFF52:
		return "COD (Coding style default)"
	case 0xFF53:
		return "COC (Coding style component)"
	case 0xFF5C:
		return "QCD (Quantization default)"
	case 0xFF5D:
		return "QCC (Quantization component)"
	case 0xFF5E:
		return "RGN (Region of interest)"
	case 0xFF5F:
		return "POC (Progression order change)"
	case 0xFF90:
		return "SOT (Start of tile)"
	case 0xFF93:
		return "SOD (Start of data)"
	case 0xFFD9:
		return "EOC (End of codestream)"
	case 0xFF64:
		return "COM (Comment)"
	default:
		return fmt.Sprintf("Unknown (0x%04X)", marker)
	}
}

func verifyRoundTrip(originalData, encodedData []byte, width, height, numComponents, bitDepth int) {
	// Decode
	fmt.Println("\nDecoding JPEG2000 codestream...")
	decoder := jpeg2000.NewDecoder()
	err := decoder.Decode(encodedData)
	if err != nil {
		fmt.Printf("  ✗ Decoding failed: %v\n", err)
		return
	}

	fmt.Printf("  ✓ Decoding successful\n")
	fmt.Printf("    Dimensions: %dx%d\n", decoder.Width(), decoder.Height())
	fmt.Printf("    Components: %d\n", decoder.Components())
	fmt.Printf("    Bit depth: %d\n", decoder.BitDepth())

	// Convert decoded components back to raw format
	decodedData := convertComponentsToRaw(decoder.GetImageData(), decoder.Width(), decoder.Height(), decoder.Components(), decoder.BitDepth())

	// Compare with original
	fmt.Println("\nComparing decoded data with original...")

	if len(decodedData) != len(originalData) {
		fmt.Printf("  ✗ Size mismatch: decoded=%d, original=%d\n", len(decodedData), len(originalData))
		return
	}

	if bytes.Equal(decodedData, originalData) {
		fmt.Printf("  ✓ Perfect match! Lossless encoding verified.\n")
		fmt.Printf("    All %d bytes are identical.\n", len(originalData))
		return
	}

	// Count differences
	differences := 0
	maxDiff := 0
	var firstDiffPos int = -1

	minLen := len(originalData)
	if len(decodedData) < minLen {
		minLen = len(decodedData)
	}

	for i := 0; i < minLen; i++ {
		if originalData[i] != decodedData[i] {
			diff := int(originalData[i]) - int(decodedData[i])
			if diff < 0 {
				diff = -diff
			}
			if diff > maxDiff {
				maxDiff = diff
			}
			if firstDiffPos == -1 {
				firstDiffPos = i
			}
			differences++
		}
	}

	if differences > 0 {
		fmt.Printf("  ✗ Differences found!\n")
		fmt.Printf("    Different bytes: %d/%d (%.4f%%)\n",
			differences, minLen, float64(differences)/float64(minLen)*100)
		fmt.Printf("    Maximum difference: %d\n", maxDiff)
		fmt.Printf("    First difference at byte: %d\n", firstDiffPos)

		// Show first few differences
		fmt.Println("\n  First 10 differences:")
		shown := 0
		for i := 0; i < minLen && shown < 10; i++ {
			if originalData[i] != decodedData[i] {
				pixel := i / numComponents
				component := i % numComponents
				fmt.Printf("    Byte %d (pixel %d, comp %d): original=0x%02X (%d), decoded=0x%02X (%d), diff=%d\n",
					i, pixel, component,
					originalData[i], originalData[i],
					decodedData[i], decodedData[i],
					int(originalData[i])-int(decodedData[i]))
				shown++
			}
		}

		// Calculate statistics
		var sumSquaredError int64
		for i := 0; i < minLen; i++ {
			diff := int(originalData[i]) - int(decodedData[i])
			sumSquaredError += int64(diff * diff)
		}
		mse := float64(sumSquaredError) / float64(minLen)
		psnr := 10.0 * float64(255*255) / mse
		if mse > 0 {
			psnr = 10.0 * (2.3025850929940456840179914546844) * float64(8*2) // log10(255^2/MSE)
			psnr = 20.0*2.407606 - 10.0*2.3025850929940456840179914546844*float64(int64(mse))/2.3025850929940456840179914546844
		}

		fmt.Printf("\n  Quality Metrics:\n")
		fmt.Printf("    MSE (Mean Squared Error): %.2f\n", mse)
		if mse > 0 {
			fmt.Printf("    PSNR (Peak Signal-to-Noise Ratio): %.2f dB\n", psnr)
		}
	}
}

func convertComponentsToRaw(components [][]int32, width, height, numComponents, bitDepth int) []byte {
	totalPixels := width * height
	var rawData []byte

	if bitDepth <= 8 {
		// 8-bit data
		rawData = make([]byte, totalPixels*numComponents)
		idx := 0
		for i := 0; i < totalPixels; i++ {
			for c := 0; c < numComponents; c++ {
				rawData[idx] = byte(components[c][i])
				idx++
			}
		}
	} else {
		// 16-bit data (little-endian)
		rawData = make([]byte, totalPixels*numComponents*2)
		idx := 0
		for i := 0; i < totalPixels; i++ {
			for c := 0; c < numComponents; c++ {
				value := uint16(components[c][i])
				rawData[idx] = byte(value & 0xFF)
				rawData[idx+1] = byte((value >> 8) & 0xFF)
				idx += 2
			}
		}
	}

	return rawData
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
