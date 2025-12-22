package main

import (
	"fmt"
	"os"

	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codec/jpegls/lossless"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/imaging"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: verify <dicom-file> [<reference-file>]")
		fmt.Println("  If reference file is provided, will compare pixel values (for lossless verification)")
		os.Exit(1)
	}

	testFile := os.Args[1]
	var refFile string
	if len(os.Args) >= 3 {
		refFile = os.Args[2]
	}

	fmt.Printf("Verifying DICOM file: %s\n", testFile)

	// Read and decode the test file
	parseResult, err := parser.ParseFile(testFile,
		parser.WithReadOption(parser.ReadAll),
		parser.WithLargeObjectSize(100*1024*1024),
	)
	if err != nil {
		fmt.Printf("✗ Failed to read file: %v\n", err)
		os.Exit(1)
	}

	ds := parseResult.Dataset
	ts := parseResult.TransferSyntax

	fmt.Printf("✓ File read successfully\n")
	fmt.Printf("  Transfer Syntax: %s\n", ts.UID().UID())

	// Get image info
	rows := ds.TryGetUInt16(tag.Rows, 0)
	cols := ds.TryGetUInt16(tag.Columns, 0)
	bits := ds.TryGetUInt16(tag.BitsStored, 0)
	samples := ds.TryGetUInt16(tag.SamplesPerPixel, 0)

	fmt.Printf("  Image size: %dx%d\n", cols, rows)
	fmt.Printf("  Bits stored: %d\n", bits)
	fmt.Printf("  Samples per pixel: %d\n", samples)

	// Decode pixel data
	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		fmt.Printf("✗ Failed to create pixel data: %v\n", err)
		os.Exit(1)
	}

	frameCount := pixelData.FrameCount()
	fmt.Printf("  Frame count: %d\n", frameCount)
	fmt.Printf("  Is encapsulated: %v\n", pixelData.IsEncapsulated())

	// If pixel data is encapsulated (compressed), decode it first
	if pixelData.IsEncapsulated() {
		fmt.Printf("  Decoding compressed pixel data...\n")

		// Get the codec for this transfer syntax
		registry := codec.GetGlobalRegistry()
		c, ok := registry.GetCodec(ts)
		if !ok {
			fmt.Printf("✗ No codec available for transfer syntax %s\n", ts.UID().UID())
			os.Exit(1)
		}

		// Decode pixel data
		decodedPixelData, err := pixelData.Decode(c, nil)
		if err != nil {
			fmt.Printf("✗ Failed to decode pixel data: %v\n", err)
			os.Exit(1)
		}
		pixelData = decodedPixelData
		fmt.Printf("  ✓ Pixel data decoded\n")
	}

	// Get first frame
	frame0, err := pixelData.GetFrame(0)
	if err != nil {
		fmt.Printf("✗ Failed to get frame 0: %v\n", err)
		os.Exit(1)
	}

	expectedSize := int(rows) * int(cols) * int(samples) * ((int(bits) + 7) / 8)
	fmt.Printf("  Decoded frame size: %d bytes (expected: %d bytes)\n", len(frame0), expectedSize)

	if len(frame0) != expectedSize {
		fmt.Printf("✗ Frame size mismatch!\n")
		os.Exit(1)
	}

	fmt.Printf("✓ Decompression successful!\n")

	// If reference file provided, compare pixel values
	if refFile != "" {
		fmt.Printf("\nComparing with reference file: %s\n", refFile)

		refResult, err := parser.ParseFile(refFile,
			parser.WithReadOption(parser.ReadAll),
			parser.WithLargeObjectSize(100*1024*1024),
		)
		if err != nil {
			fmt.Printf("✗ Failed to read reference file: %v\n", err)
			os.Exit(1)
		}

		refPixelData, err := imaging.CreatePixelData(refResult.Dataset)
		if err != nil {
			fmt.Printf("✗ Failed to create reference pixel data: %v\n", err)
			os.Exit(1)
		}

		refFrame0, err := refPixelData.GetFrame(0)
		if err != nil {
			fmt.Printf("✗ Failed to get reference frame: %v\n", err)
			os.Exit(1)
		}

		if len(frame0) != len(refFrame0) {
			fmt.Printf("✗ Frame size mismatch: test=%d, ref=%d\n", len(frame0), len(refFrame0))
			os.Exit(1)
		}

		// Compare pixel by pixel
		errors := 0
		maxDiff := 0
		totalDiff := int64(0)
		firstError := -1

		if bits <= 8 {
			// 8-bit comparison
			for i := 0; i < len(frame0); i++ {
				if frame0[i] != refFrame0[i] {
					if firstError == -1 {
						firstError = i
					}
					errors++
					diff := int(frame0[i]) - int(refFrame0[i])
					if diff < 0 {
						diff = -diff
					}
					if diff > maxDiff {
						maxDiff = diff
					}
					totalDiff += int64(diff)
				}
			}
		} else {
			// 16-bit comparison
			for i := 0; i < len(frame0)/2; i++ {
				testVal := int(frame0[i*2]) | (int(frame0[i*2+1]) << 8)
				refVal := int(refFrame0[i*2]) | (int(refFrame0[i*2+1]) << 8)
				if testVal != refVal {
					if firstError == -1 {
						firstError = i
					}
					errors++
					diff := testVal - refVal
					if diff < 0 {
						diff = -diff
					}
					if diff > maxDiff {
						maxDiff = diff
					}
					totalDiff += int64(diff)
				}
			}
		}

		totalPixels := int(rows) * int(cols) * int(samples)
		if errors > 0 {
			avgDiff := float64(totalDiff) / float64(errors)
			errorPercent := float64(errors) * 100.0 / float64(totalPixels)
			fmt.Printf("✗ Pixel differences found:\n")
			fmt.Printf("  Errors: %d / %d pixels (%.2f%%)\n", errors, totalPixels, errorPercent)
			fmt.Printf("  First error at pixel: %d\n", firstError)
			fmt.Printf("  Max difference: %d\n", maxDiff)
			fmt.Printf("  Average difference: %.2f\n", avgDiff)

			// For lossless codecs, any difference is an error
			if ts.UID().UID() == "1.2.840.10008.1.2.4.70" || // JPEG Lossless SV1
				ts.UID().UID() == "1.2.840.10008.1.2.4.80" || // JPEG-LS Lossless
				ts.UID().UID() == "1.2.840.10008.1.2.4.90" { // JPEG 2000 Lossless
				fmt.Printf("✗ LOSSLESS codec should have 0 errors!\n")
				os.Exit(1)
			}
		} else {
			fmt.Printf("✓ Perfect match! (0 pixel errors)\n")
			fmt.Printf("  %d pixels compared, all identical\n", totalPixels)
		}
	}

	fmt.Println("\n✓ Verification complete!")
}
