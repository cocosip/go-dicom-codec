package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Register all codecs by importing them
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codec/jpegls/lossless"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/io/buffer"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║         DICOM Transfer Syntax Transcoder                       ║")
	fmt.Println("║         Converts DICOM files between compression formats       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Get input file path
	inputPath := getInputFilePath()
	if inputPath == "" {
		fmt.Println("\n✗ No input file specified. Exiting...")
		waitForExit()
		return
	}

	// Check if file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Printf("\n✗ File not found: %s\n", inputPath)
		waitForExit()
		return
	}

	fmt.Printf("\n📂 Input file: %s\n", inputPath)

	// Read DICOM file
	fmt.Println("\n⏳ Reading DICOM file...")
	parseResult, err := parser.ParseFile(inputPath,
		parser.WithReadOption(parser.ReadAll),     // Read all elements including pixel data
		parser.WithLargeObjectSize(100*1024*1024), // Allow large objects up to 100MB
	)
	if err != nil {
		fmt.Printf("✗ Failed to read DICOM file: %v\n", err)
		waitForExit()
		return
	}

	ds := parseResult.Dataset
	sourceTS := parseResult.TransferSyntax

	fmt.Printf("✓ Successfully read DICOM file\n")
	fmt.Printf("  Source Transfer Syntax: %s\n", sourceTS.UID().UID())

	// Debug: Check pixel data element type
	if pixelDataElem, ok := ds.Get(tag.PixelData); ok {
		fmt.Printf("  Pixel Data Element Type: %T\n", pixelDataElem)
	} else {
		fmt.Printf("  ⚠ Warning: Pixel Data element not found\n")
	}

	// Display image information
	displayImageInfo(ds)

	// Get output directory
	outputDir := getOutputDirectory(inputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("\n✗ Failed to create output directory: %v\n", err)
		waitForExit()
		return
	}

	fmt.Printf("\n📁 Output directory: %s\n", outputDir)

	// Define target transfer syntaxes
	targetFormats := []struct {
		name       string
		ts         *transfer.Syntax
		suffix     string
		isLossless bool
	}{
		{
			name:       "JPEG Baseline (Lossy 8-bit)",
			ts:         transfer.JPEGBaseline8Bit,
			suffix:     "jpeg_baseline",
			isLossless: false,
		},
		{
			name:       "JPEG Lossless SV1",
			ts:         transfer.JPEGLosslessSV1,
			suffix:     "jpeg_lossless_sv1",
			isLossless: true,
		},
		{
			name:       "JPEG-LS Lossless",
			ts:         transfer.JPEGLSLossless,
			suffix:     "jpegls_lossless",
			isLossless: true,
		},
		{
			name:       "JPEG 2000 Lossless",
			ts:         transfer.JPEG2000Lossless,
			suffix:     "j2k_lossless",
			isLossless: true,
		},
		{
			name:       "JPEG 2000 Lossy",
			ts:         transfer.JPEG2000,
			suffix:     "j2k_lossy",
			isLossless: false,
		},
	}

	// Get codec registry
	registry := codec.GetGlobalRegistry()

	// Transcode to each format
	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("Starting transcoding process...")
	fmt.Println(strings.Repeat("═", 70))

	successCount := 0
	failCount := 0

	for i, format := range targetFormats {
		fmt.Printf("\n[%d/%d] Transcoding to %s\n", i+1, len(targetFormats), format.name)
		fmt.Printf("      Transfer Syntax: %s\n", format.ts.UID().UID())

		// Pre-check for JPEG Baseline 8-bit limitation
		bitsStored := ds.TryGetUInt16(tag.BitsStored, 0)
		if format.ts == transfer.JPEGBaseline8Bit && bitsStored > 8 {
			fmt.Printf("      ⊘ Skipped: JPEG Baseline only supports 8-bit images (your image is %d-bit)\n", bitsStored)
			fmt.Printf("      💡 Tip: Use JPEG 2000 Lossy for high-quality lossy compression of 16-bit images\n")
			failCount++
			continue
		}

		// Generate output filename
		baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.dcm", baseName, format.suffix))

		// Perform transcoding
		if err := transcodeDICOMFile(ds, outputPath, sourceTS, format.ts, registry); err != nil {
			fmt.Printf("      ✗ Failed: %v\n", err)
			failCount++
			continue
		}

		// Get file sizes
		inputSize, _ := getFileSize(inputPath)
		outputSize, _ := getFileSize(outputPath)
		ratio := float64(inputSize) / float64(outputSize)

		fmt.Printf("      ✓ Success!\n")
		fmt.Printf("      📊 Size: %s → %s (%.2fx compression)\n",
			formatBytes(inputSize), formatBytes(outputSize), ratio)
		fmt.Printf("      💾 Output: %s\n", filepath.Base(outputPath))

		successCount++
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("Transcoding Summary")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("✓ Successful: %d\n", successCount)
	if failCount > 0 {
		fmt.Printf("✗ Failed:     %d\n", failCount)
	}
	fmt.Printf("📁 Output directory: %s\n", outputDir)
	fmt.Println(strings.Repeat("═", 70))

	// Wait for user input before exit
	waitForExit()
}

// getInputFilePath gets the input DICOM file path from command line or user input
func getInputFilePath() string {
	// Check command line arguments
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// Interactive input
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter DICOM file path (or drag and drop file here): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	// Remove quotes if present (from drag and drop)
	input = strings.Trim(input, "\"'")

	return input
}

// getOutputDirectory determines the output directory based on input file
func getOutputDirectory(inputPath string) string {
	// Get the directory of the input file
	inputDir := filepath.Dir(inputPath)

	// Create output directory name
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Join(inputDir, baseName+"_transcoded")

	return outputDir
}

// displayImageInfo shows information about the DICOM image
func displayImageInfo(ds *dataset.Dataset) {
	fmt.Println("\n📋 Image Information:")

	// Get image dimensions
	rows := ds.TryGetUInt16(tag.Rows, 0)
	if rows > 0 {
		fmt.Printf("  Rows: %d\n", rows)
	}

	cols := ds.TryGetUInt16(tag.Columns, 0)
	if cols > 0 {
		fmt.Printf("  Columns: %d\n", cols)
	}

	// Get bit depth
	bits := ds.TryGetUInt16(tag.BitsStored, 0)
	if bits > 0 {
		fmt.Printf("  Bits Stored: %d\n", bits)
	}

	// Get samples per pixel
	samples := ds.TryGetUInt16(tag.SamplesPerPixel, 0)
	if samples > 0 {
		fmt.Printf("  Samples Per Pixel: %d\n", samples)
	}

	// Get photometric interpretation
	if pi, ok := ds.GetString(tag.PhotometricInterpretation); ok {
		fmt.Printf("  Photometric Interpretation: %s\n", pi)
	}

	// Get modality
	if modality, ok := ds.GetString(tag.Modality); ok {
		fmt.Printf("  Modality: %s\n", modality)
	}
}

// transcodeDICOMFile converts a DICOM dataset from one transfer syntax to another
func transcodeDICOMFile(ds *dataset.Dataset, outputPath string, sourceTS, targetTS *transfer.Syntax, registry *codec.Registry) error {
	// Skip if already in target format
	if sourceTS.UID().UID() == targetTS.UID().UID() {
		// Just copy the dataset with correct transfer syntax
		if err := writer.WriteFile(outputPath, ds, writer.WithTransferSyntax(sourceTS)); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	}

	// Create source pixel data from DICOM dataset
	srcPixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return fmt.Errorf("failed to create source pixel data: %w", err)
	}

	// Get target codec
	targetCodec, ok := registry.GetCodec(targetTS)
	if !ok {
		return fmt.Errorf("no codec available for target transfer syntax %s", targetTS.UID().UID())
	}

	// Create source wrapper using TestPixelData
	// This allows us to extract frames from the DicomPixelData
	srcFrameInfo := srcPixelData.GetFrameInfo()
	srcWrapper := codecHelpers.NewTestPixelData(srcFrameInfo)

	// Add all frames from source
	for i := 0; i < srcPixelData.FrameCount(); i++ {
		frameData, err := srcPixelData.GetFrame(i)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", i, err)
		}
		if err := srcWrapper.AddFrame(frameData); err != nil {
			return fmt.Errorf("failed to add source frame %d: %w", i, err)
		}
	}

	// Create destination wrapper for encoded data
	dstWrapper := codecHelpers.NewTestPixelData(srcFrameInfo)

	// Encode using the codec
	// The codec will compress each frame and add to dstWrapper
	if err := targetCodec.Encode(srcWrapper, dstWrapper, nil); err != nil {
		return fmt.Errorf("encoding to %s failed: %w", targetTS.UID().UID(), err)
	}

	// Clone the dataset and remove old pixel data
	newDS := ds.Clone()
	newDS.Remove(tag.PixelData)

	// Create encapsulated pixel data element (OtherByteFragment for compressed data)
	obf := element.NewOtherByteFragment(tag.PixelData)

	// Add each encoded frame as a fragment
	for i := 0; i < dstWrapper.FrameCount(); i++ {
		frameData, err := dstWrapper.GetFrame(i)
		if err != nil {
			return fmt.Errorf("failed to get encoded frame %d: %w", i, err)
		}
		obf.AddFragment(buffer.NewMemory(frameData))
	}

	// Add pixel data element to dataset
	if err := newDS.Add(obf); err != nil {
		return fmt.Errorf("failed to add pixel data: %w", err)
	}

	// Write with correct transfer syntax
	if err := writer.WriteFile(outputPath, newDS, writer.WithTransferSyntax(targetTS)); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// getFileSize returns the size of a file in bytes
func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// waitForExit waits for user input before exiting
func waitForExit() {
	fmt.Println("\n" + strings.Repeat("─", 70))
	fmt.Print("Press Enter to exit...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
