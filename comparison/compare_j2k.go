// JPEG2000 comparison tool for analyzing differences between OpenJPEG and Go encoders
// Usage: go run compare_j2k.go <openjpeg_output.j2k> <go_output.j2k> [detailed]

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <openjpeg_output.j2k> <go_output.j2k> [detailed]\n", os.Args[0])
		os.Exit(1)
	}

	openjpegFile := os.Args[1]
	goFile := os.Args[2]
	detailed := len(os.Args) > 3 && os.Args[3] == "detailed"

	fmt.Println("JPEG2000 Comparison Tool")
	fmt.Println("=" + string(bytes.Repeat([]byte("="), 60)))

	// Read files
	openjpegData, err := os.ReadFile(openjpegFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read OpenJPEG file: %v\n", err)
		os.Exit(1)
	}

	goData, err := os.ReadFile(goFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read Go file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFile Sizes:\n")
	fmt.Printf("  OpenJPEG: %d bytes\n", len(openjpegData))
	fmt.Printf("  Go:       %d bytes\n", len(goData))
	fmt.Printf("  Difference: %d bytes (%.2f%%)\n",
		len(goData)-len(openjpegData),
		float64(len(goData)-len(openjpegData))/float64(len(openjpegData))*100)

	// Parse codestreams
	fmt.Println("\n" + string(bytes.Repeat([]byte("-"), 60)))
	fmt.Println("Parsing OpenJPEG codestream...")
	openjpegMarkers := parseCodestream(openjpegData, "OpenJPEG")

	fmt.Println("\n" + string(bytes.Repeat([]byte("-"), 60)))
	fmt.Println("Parsing Go codestream...")
	goMarkers := parseCodestream(goData, "Go")

	// Compare marker sequences
	fmt.Println("\n" + string(bytes.Repeat([]byte("="), 60)))
	fmt.Println("MARKER SEQUENCE COMPARISON")
	fmt.Println(string(bytes.Repeat([]byte("="), 60)))
	compareMarkerSequence(openjpegMarkers, goMarkers)

	// Detailed comparison if requested
	if detailed {
		fmt.Println("\n" + string(bytes.Repeat([]byte("="), 60)))
		fmt.Println("DETAILED MARKER COMPARISON")
		fmt.Println(string(bytes.Repeat([]byte("="), 60)))
		detailedMarkerComparison(openjpegMarkers, goMarkers)
	}

	// Try to decode both and compare
	fmt.Println("\n" + string(bytes.Repeat([]byte("="), 60)))
	fmt.Println("DECODING VERIFICATION")
	fmt.Println(string(bytes.Repeat([]byte("="), 60)))
	verifyDecoding(openjpegData, goData)
}

type MarkerInfo struct {
	Type   uint16
	Name   string
	Offset int
	Length int
	Data   []byte
}

func parseCodestream(data []byte, source string) []MarkerInfo {
	markers := []MarkerInfo{}
	r := bytes.NewReader(data)
	offset := 0

	for {
		// Read marker
		var marker uint16
		err := binary.Read(r, binary.BigEndian, &marker)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Error reading marker at offset %d: %v\n", offset, err)
			break
		}

		markerName := getMarkerName(marker)
		markerOffset := offset
		offset += 2

		var markerData []byte
		var length int

		// Some markers don't have length fields
		if marker == 0xFF4F || marker == 0xFF93 || marker == 0xFFD9 { // SOC, SOD, EOC
			length = 0
		} else {
			// Read length
			var segLength uint16
			err = binary.Read(r, binary.BigEndian, &segLength)
			if err != nil {
				fmt.Printf("Error reading marker length at offset %d: %v\n", offset, err)
				break
			}
			offset += 2
			length = int(segLength) - 2

			// Read marker data
			markerData = make([]byte, length)
			n, err := r.Read(markerData)
			if err != nil && err != io.EOF {
				fmt.Printf("Error reading marker data at offset %d: %v\n", offset, err)
				break
			}
			offset += n
		}

		markers = append(markers, MarkerInfo{
			Type:   marker,
			Name:   markerName,
			Offset: markerOffset,
			Length: length,
			Data:   markerData,
		})

		// Stop at SOD (Start of Data) - compressed data follows
		if marker == 0xFF93 {
			// Read remaining compressed data until EOC
			remaining := data[offset:]
			eocPos := bytes.Index(remaining, []byte{0xFF, 0xD9})
			if eocPos != -1 {
				compressedSize := eocPos
				fmt.Printf("  Compressed data size: %d bytes\n", compressedSize)
				offset += compressedSize
			}
		}
	}

	fmt.Printf("  Total markers found: %d\n", len(markers))
	return markers
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

func compareMarkerSequence(openjpeg, goMarkers []MarkerInfo) {
	maxLen := len(openjpeg)
	if len(goMarkers) > maxLen {
		maxLen = len(goMarkers)
	}

	fmt.Printf("\n%-5s %-40s %-40s %-10s\n", "Idx", "OpenJPEG", "Go", "Match")
	fmt.Println(string(bytes.Repeat([]byte("-"), 100)))

	matchCount := 0
	for i := 0; i < maxLen; i++ {
		var opjName, goName string
		var opjLen, goLen int
		match := "✗"

		if i < len(openjpeg) {
			opjName = fmt.Sprintf("%s (%d bytes)", openjpeg[i].Name, openjpeg[i].Length)
			opjLen = openjpeg[i].Length
		} else {
			opjName = "-"
		}

		if i < len(goMarkers) {
			goName = fmt.Sprintf("%s (%d bytes)", goMarkers[i].Name, goMarkers[i].Length)
			goLen = goMarkers[i].Length
		} else {
			goName = "-"
		}

		if i < len(openjpeg) && i < len(goMarkers) {
			if openjpeg[i].Type == goMarkers[i].Type && opjLen == goLen {
				match = "✓"
				matchCount++
			} else if openjpeg[i].Type == goMarkers[i].Type {
				match = "~ (size diff)"
			}
		}

		fmt.Printf("%-5d %-40s %-40s %-10s\n", i, opjName, goName, match)
	}

	fmt.Println(string(bytes.Repeat([]byte("-"), 100)))
	fmt.Printf("Match rate: %d/%d (%.1f%%)\n", matchCount, maxLen, float64(matchCount)/float64(maxLen)*100)
}

func detailedMarkerComparison(openjpeg, goMarkers []MarkerInfo) {
	for i := 0; i < len(openjpeg) && i < len(goMarkers); i++ {
		if openjpeg[i].Type != goMarkers[i].Type {
			continue
		}

		fmt.Printf("\n%s\n", openjpeg[i].Name)
		fmt.Println(string(bytes.Repeat([]byte("-"), 60)))

		if len(openjpeg[i].Data) == 0 && len(goMarkers[i].Data) == 0 {
			fmt.Println("  (No payload)")
			continue
		}

		// Compare bytes
		minLen := len(openjpeg[i].Data)
		if len(goMarkers[i].Data) < minLen {
			minLen = len(goMarkers[i].Data)
		}

		differences := 0
		for j := 0; j < minLen; j++ {
			if openjpeg[i].Data[j] != goMarkers[i].Data[j] {
				if differences < 10 { // Show first 10 differences
					fmt.Printf("  Byte %d: OpenJPEG=0x%02X, Go=0x%02X\n",
						j, openjpeg[i].Data[j], goMarkers[i].Data[j])
				}
				differences++
			}
		}

		if differences > 0 {
			fmt.Printf("  Total byte differences: %d\n", differences)
		} else {
			fmt.Println("  ✓ Payload identical")
		}

		if len(openjpeg[i].Data) != len(goMarkers[i].Data) {
			fmt.Printf("  Length difference: OpenJPEG=%d, Go=%d\n",
				len(openjpeg[i].Data), len(goMarkers[i].Data))
		}
	}
}

func verifyDecoding(openjpegData, goData []byte) {
	// Decode OpenJPEG output
	fmt.Println("\nDecoding OpenJPEG output...")
	opjDecoder := jpeg2000.NewDecoder()
	opjResult, err := opjDecoder.Decode(openjpegData)
	var opjPixels []byte
	var opjWidth, opjHeight, opjComponents, opjBitDepth int

	if err != nil {
		fmt.Printf("  ✗ Failed to decode: %v\n", err)
	} else {
		fmt.Printf("  ✓ Decoded successfully\n")
		opjWidth = opjResult.Width
		opjHeight = opjResult.Height
		opjComponents = opjResult.Components
		opjBitDepth = opjResult.BitDepth
		opjPixels = convertComponentsToRaw(opjResult.ComponentData, opjWidth, opjHeight, opjComponents, opjBitDepth)
		fmt.Printf("    Dimensions: %dx%d\n", opjWidth, opjHeight)
		fmt.Printf("    Components: %d\n", opjComponents)
		fmt.Printf("    Bit depth: %d\n", opjBitDepth)
		fmt.Printf("    Pixel data size: %d bytes\n", len(opjPixels))
	}

	// Decode Go output
	fmt.Println("\nDecoding Go output...")
	goDecoder := jpeg2000.NewDecoder()
	goResult, err := goDecoder.Decode(goData)
	var goPixels []byte
	var goWidth, goHeight, goComponents, goBitDepth int

	if err != nil {
		fmt.Printf("  ✗ Failed to decode: %v\n", err)
	} else {
		fmt.Printf("  ✓ Decoded successfully\n")
		goWidth = goResult.Width
		goHeight = goResult.Height
		goComponents = goResult.Components
		goBitDepth = goResult.BitDepth
		goPixels = convertComponentsToRaw(goResult.ComponentData, goWidth, goHeight, goComponents, goBitDepth)
		fmt.Printf("    Dimensions: %dx%d\n", goWidth, goHeight)
		fmt.Printf("    Components: %d\n", goComponents)
		fmt.Printf("    Bit depth: %d\n", goBitDepth)
		fmt.Printf("    Pixel data size: %d bytes\n", len(goPixels))
	}

	// Compare decoded pixels
	if opjPixels != nil && goPixels != nil {
		fmt.Println("\nComparing decoded pixels...")
		if bytes.Equal(opjPixels, goPixels) {
			fmt.Println("  ✓ Decoded pixels are identical!")
		} else {
			differences := 0
			maxDiff := 0
			minLen := len(opjPixels)
			if len(goPixels) < minLen {
				minLen = len(goPixels)
			}

			for i := 0; i < minLen; i++ {
				if opjPixels[i] != goPixels[i] {
					diff := int(opjPixels[i]) - int(goPixels[i])
					if diff < 0 {
						diff = -diff
					}
					if diff > maxDiff {
						maxDiff = diff
					}
					differences++
				}
			}

			fmt.Printf("  ✗ Pixel differences found: %d/%d (%.2f%%)\n",
				differences, minLen, float64(differences)/float64(minLen)*100)
			fmt.Printf("  Maximum difference: %d\n", maxDiff)
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
