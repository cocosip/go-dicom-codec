package main

import (
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: compare_jpegls <file1> <file2>")
		fmt.Println("Compares two JPEG-LS encoded DICOM files")
		return
	}

	file1 := os.Args[1]
	file2 := os.Args[2]

	fmt.Printf("Comparing:\n")
	fmt.Printf("  File 1: %s\n", file1)
	fmt.Printf("  File 2: %s\n", file2)
	fmt.Println()

	data1, err := os.ReadFile(file1)
	if err != nil {
		fmt.Printf("Error reading file 1: %v\n", err)
		return
	}

	data2, err := os.ReadFile(file2)
	if err != nil {
		fmt.Printf("Error reading file 2: %v\n", err)
		return
	}

	fmt.Printf("File sizes:\n")
	fmt.Printf("  File 1: %d bytes\n", len(data1))
	fmt.Printf("  File 2: %d bytes\n", len(data2))
	fmt.Println()

	// Find pixel data tag (7FE0,0010)
	pixelTag := []byte{0xE0, 0x7F, 0x10, 0x00}

	idx1 := findPixelData(data1, pixelTag)
	idx2 := findPixelData(data2, pixelTag)

	if idx1 == -1 || idx2 == -1 {
		fmt.Println("Could not find pixel data in one or both files")
		return
	}

	fmt.Printf("Pixel Data tag found at:\n")
	fmt.Printf("  File 1: offset %d (0x%X)\n", idx1, idx1)
	fmt.Printf("  File 2: offset %d (0x%X)\n", idx2, idx2)
	fmt.Println()

	// Extract encapsulated pixel data
	encap1, err := extractEncapsulatedData(data1[idx1:])
	if err != nil {
		fmt.Printf("Error extracting file 1 data: %v\n", err)
		return
	}

	encap2, err := extractEncapsulatedData(data2[idx2:])
	if err != nil {
		fmt.Printf("Error extracting file 2 data: %v\n", err)
		return
	}

	fmt.Printf("Encapsulated data sizes:\n")
	fmt.Printf("  File 1: %d bytes\n", len(encap1))
	fmt.Printf("  File 2: %d bytes\n", len(encap2))
	fmt.Println()

	// Analyze JPEG-LS markers
	fmt.Println("=== File 1 JPEG-LS Markers ===")
	analyzeJPEGLS(encap1)
	fmt.Println()

	fmt.Println("=== File 2 JPEG-LS Markers ===")
	analyzeJPEGLS(encap2)
	fmt.Println()

	// Compare first 200 bytes
	fmt.Println("=== First 200 bytes comparison ===")
	compareBytes(encap1, encap2, 200)
}

func findPixelData(data []byte, tag []byte) int {
	for i := 0; i < len(data)-len(tag); i++ {
		match := true
		for j := 0; j < len(tag); j++ {
			if data[i+j] != tag[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func extractEncapsulatedData(data []byte) ([]byte, error) {
	// Skip tag (4), VR (2), reserved (2), length (4) = 12 bytes for OB
	if len(data) < 12 {
		return nil, fmt.Errorf("data too short")
	}

	// Check if it's OB VR
	if data[4] == 'O' && data[5] == 'B' {
		// Skip to BOT (Basic Offset Table)
		// BOT starts at offset 12
		// BOT format: (FFFE,E000) + length (4 bytes) + offsets

		// Find first fragment after BOT
		offset := 12
		for offset+8 < len(data) {
			// Check for item tag (FFFE,E000) or (FFFE,E00D)
			if data[offset] == 0xFE && data[offset+1] == 0xFF {
				if data[offset+2] == 0x00 && data[offset+3] == 0xE0 {
					// Item tag found
					fragLen := int(data[offset+4]) | int(data[offset+5])<<8 |
					          int(data[offset+6])<<16 | int(data[offset+7])<<24

					if fragLen > 0 && fragLen < len(data)-offset-8 {
						// Return first fragment (JPEG-LS stream)
						return data[offset+8 : offset+8+fragLen], nil
					}
				} else if data[offset+2] == 0x0D && data[offset+3] == 0xE0 {
					// Sequence delimiter
					break
				}
			}
			offset++
		}
	}

	return nil, fmt.Errorf("could not extract encapsulated data")
}

func analyzeJPEGLS(data []byte) {
	i := 0
	for i < len(data)-1 {
		if data[i] == 0xFF {
			marker := data[i+1]
			markerName := getMarkerName(marker)

			fmt.Printf("  [%04d] 0xFF%02X - %s\n", i, marker, markerName)

			// Get marker length if applicable
			if hasLength(marker) && i+3 < len(data) {
				length := int(data[i+2])<<8 | int(data[i+3])
				fmt.Printf("         Length: %d bytes\n", length)

				// For SOF55, show frame parameters
				if marker == 0xF7 && i+10 < len(data) {
					fmt.Printf("         Precision: %d bits\n", data[i+4])
					height := int(data[i+5])<<8 | int(data[i+6])
					width := int(data[i+7])<<8 | int(data[i+8])
					components := int(data[i+9])
					fmt.Printf("         Dimensions: %dx%d\n", width, height)
					fmt.Printf("         Components: %d\n", components)
				}

				// For LSE, show extension parameters
				if marker == 0xF8 && i+12 < len(data) {
					fmt.Printf("         Type: %d\n", data[i+4])
					maxVal := int(data[i+5])<<8 | int(data[i+6])
					t1 := int(data[i+7])<<8 | int(data[i+8])
					t2 := int(data[i+9])<<8 | int(data[i+10])
					t3 := int(data[i+11])<<8 | int(data[i+12])
					fmt.Printf("         MAXVAL: %d, T1: %d, T2: %d, T3: %d\n", maxVal, t1, t2, t3)
				}

				i += 2 + length
			} else {
				i += 2
			}
		} else {
			// Found scan data
			fmt.Printf("  [%04d] Scan data starts (first 16 bytes):\n", i)
			end := i + 16
			if end > len(data) {
				end = len(data)
			}
			fmt.Printf("         %s\n", hex.EncodeToString(data[i:end]))
			break
		}
	}
}

func getMarkerName(marker byte) string {
	names := map[byte]string{
		0xD8: "SOI (Start of Image)",
		0xF7: "SOF55 (Start of Frame JPEG-LS)",
		0xF8: "LSE (JPEG-LS Extension)",
		0xDA: "SOS (Start of Scan)",
		0xD9: "EOI (End of Image)",
		0xC0: "SOF0 (Baseline DCT)",
		0xC3: "SOF3 (Lossless)",
	}
	if name, ok := names[marker]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (0x%02X)", marker)
}

func hasLength(marker byte) bool {
	// Markers without length field
	noLength := map[byte]bool{
		0xD8: true, // SOI
		0xD9: true, // EOI
		0x01: true, // TEM
	}
	// RST markers (0xD0-0xD7)
	if marker >= 0xD0 && marker <= 0xD7 {
		return false
	}
	return !noLength[marker]
}

func compareBytes(data1, data2 []byte, maxBytes int) {
	minLen := len(data1)
	if len(data2) < minLen {
		minLen = len(data2)
	}
	if maxBytes < minLen {
		minLen = maxBytes
	}

	for i := 0; i < minLen; i += 16 {
		end := i + 16
		if end > minLen {
			end = minLen
		}

		hex1 := hex.EncodeToString(data1[i:end])
		hex2 := hex.EncodeToString(data2[i:end])

		if hex1 != hex2 {
			fmt.Printf("  [%04d] File1: %s\n", i, hex1)
			fmt.Printf("  [%04d] File2: %s <-- DIFFERENT\n", i, hex2)
		} else {
			fmt.Printf("  [%04d] Both:  %s\n", i, hex1)
		}
	}
}
