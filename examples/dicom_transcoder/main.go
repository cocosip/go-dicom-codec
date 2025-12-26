package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Register all codecs by importing them
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codec/jpegls/lossless"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	fmt.Println("DICOM Transfer Syntax Transcoder")
	fmt.Println("Converts DICOM files between compression formats")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Println()

	// Compatibility: allow forcing unsigned output for viewers that ignore PR=1 shift
	forceUnsigned := os.Getenv("DICOM_FORCE_UNSIGNED") == "1"

	// Get input file path
	inputPath := getInputFilePath()
	if inputPath == "" {
		fmt.Println("\nNo input file specified. Exiting...")
		waitForExit()
		return
	}

	// Check if file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Printf("\nFile not found: %s\n", inputPath)
		waitForExit()
		return
	}

	fmt.Printf("\nInput file: %s\n", inputPath)

	// Read DICOM file
	fmt.Println("\nReading DICOM file...")
	parseResult, err := parser.ParseFile(inputPath,
		parser.WithReadOption(parser.ReadAll),
		parser.WithLargeObjectSize(100*1024*1024),
	)
	if err != nil {
		fmt.Printf("Failed to read DICOM file: %v\n", err)
		waitForExit()
		return
	}

	ds := parseResult.Dataset
	sourceTS := parseResult.TransferSyntax

	fmt.Printf("Successfully read DICOM file\n")
	fmt.Printf("  Source Transfer Syntax: %s\n", sourceTS.UID().UID())

	// Display image information
	displayImageInfo(ds)

	// Get output directory
	outputDir := getOutputDirectory(inputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("\nFailed to create output directory: %v\n", err)
		waitForExit()
		return
	}

	fmt.Printf("\nOutput directory: %s\n", outputDir)

	// Define target transfer syntaxes
	targetFormats := []struct {
		name       string
		ts         *transfer.Syntax
		suffix     string
		isLossless bool
	}{
		{"JPEG Baseline (Lossy 8-bit)", transfer.JPEGBaseline8Bit, "jpeg_baseline", false},
		{"JPEG JPEGProcess2_4", transfer.JPEGProcess2_4, "jpeg_process2_4", true},
		{"JPEG Lossless", transfer.JPEGLossless, "jpeg_lossless", true},
		{"JPEG Lossless SV1", transfer.JPEGLosslessSV1, "jpeg_lossless_sv1", true},
		{"JPEG-LS Lossless", transfer.JPEGLSLossless, "jpegls_lossless", true},
		{"RLE", transfer.RLELossless, "rle", false},
		{"JPEG 2000 Lossless", transfer.JPEG2000Lossless, "j2k_lossless", true},
		{"JPEG 2000 Lossy", transfer.JPEG2000, "j2k_lossy", false},
	}

	// Get codec registry
	registry := codec.GetGlobalRegistry()

	// Transcode to each format
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("Starting transcoding process...")
	fmt.Println(strings.Repeat("-", 70))

	successCount := 0
	failCount := 0

	for i, format := range targetFormats {
		fmt.Printf("\n[%d/%d] Transcoding to %s\n", i+1, len(targetFormats), format.name)
		fmt.Printf("      Transfer Syntax: %s\n", format.ts.UID().UID())

		// Pre-check for JPEG Baseline 8-bit limitation
		bitsStored := ds.TryGetUInt16(tag.BitsStored, 0)
		if format.ts == transfer.JPEGBaseline8Bit && bitsStored > 8 {
			fmt.Printf("      Skipped: JPEG Baseline only supports 8-bit images (your image is %d-bit)\n", bitsStored)
			failCount++
			continue
		}

		// Generate output filename
		baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.dcm", baseName, format.suffix))

		// Perform transcoding
		if err := transcodeDICOMFile(ds, outputPath, sourceTS, format.ts, registry, forceUnsigned); err != nil {
			fmt.Printf("      Failed: %v\n", err)
			failCount++
			continue
		}

		// Get file sizes
		inputSize, _ := getFileSize(inputPath)
		outputSize, _ := getFileSize(outputPath)
		ratio := float64(inputSize) / float64(outputSize)

		fmt.Printf("      Success!\n")
		fmt.Printf("      Size: %s -> %s (%.2fx compression)\n",
			formatBytes(inputSize), formatBytes(outputSize), ratio)
		fmt.Printf("      Output: %s\n", filepath.Base(outputPath))

		successCount++
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("Transcoding Summary")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Success: %d\n", successCount)
	if failCount > 0 {
		fmt.Printf("Failed:  %d\n", failCount)
	}
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Println(strings.Repeat("-", 70))

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
	fmt.Println("\nImage Information:")

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
func transcodeDICOMFile(ds *dataset.Dataset, outputPath string, sourceTS, targetTS *transfer.Syntax, registry *codec.Registry, forceUnsigned bool) error {
	// Skip if already in target format
	if sourceTS.UID().UID() == targetTS.UID().UID() {
		clone := ds.Clone()
		if err := writer.WriteFile(outputPath, clone, writer.WithTransferSyntax(sourceTS)); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	}

	// Use go-dicom transcoder which handles encapsulated data, BOT/padding, etc.
	transcoder := codec.NewTranscoder(sourceTS, targetTS, codec.WithCodecRegistry(registry))
	newDS, err := transcoder.Transcode(ds)
	if err != nil {
		return fmt.Errorf("transcode failed: %w", err)
	}

	// For lossless JPEG variants with PR=1, update metadata to unsigned with intercept shift
	if isLosslessJPEG(targetTS) && newDS.TryGetUInt16(tag.PixelRepresentation, 0) == 1 {
		if err := forceUnsignedPixelData(newDS); err != nil {
			return fmt.Errorf("force unsigned (lossless auto): %w", err)
		}
	}

	// Compatibility: remove preset window width/center so viewers auto-window on new pixel range
	_ = newDS.Remove(tag.WindowCenter)
	_ = newDS.Remove(tag.WindowWidth)

	// Write with correct transfer syntax (also ensures File Meta includes TSUID)
	if err := writer.WriteFile(outputPath, newDS, writer.WithTransferSyntax(targetTS)); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// forceUnsignedPixelData rewrites PixelRepresentation to unsigned and adjusts RescaleIntercept
// so that modality values (HU) remain consistent. This is a compatibility path for viewers
// that fail to apply the signed offset defined by JPEG Lossless for PR=1.
func forceUnsignedPixelData(ds *dataset.Dataset) error {
	pr := ds.TryGetUInt16(tag.PixelRepresentation, 0)
	if pr == 0 {
		return nil
	}

	// Update PixelRepresentation to unsigned.
	_ = ds.Remove(tag.PixelRepresentation)
	_ = ds.Add(element.NewUnsignedShort(tag.PixelRepresentation, []uint16{0}))

	// Adjust RescaleIntercept if present: I' = I - 2^(BitsStored-1) * Slope
	bitsStored := ds.TryGetUInt16(tag.BitsStored, 0)
	if bitsStored == 0 {
		bitsStored = ds.TryGetUInt16(tag.BitsAllocated, 0)
	}
	offset := float64(uint32(1) << (bitsStored - 1))

	slope := 1.0
	if v, err := ds.GetFloat64(tag.RescaleSlope, 0); err == nil {
		slope = v
	}
	if intercept, err := ds.GetFloat64(tag.RescaleIntercept, 0); err == nil {
		newIntercept := intercept - offset*slope
		_ = ds.Remove(tag.RescaleIntercept)
		_ = ds.Add(element.NewDouble(tag.RescaleIntercept, []float64{newIntercept}))
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
	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Print("Press Enter to exit...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// isLosslessJPEG reports whether the target TS is one of the lossless JPEG/L JPEG-LS variants
func isLosslessJPEG(ts *transfer.Syntax) bool {
	return ts == transfer.JPEGLosslessSV1 ||
		ts == transfer.JPEGLossless ||
		ts == transfer.JPEGLSLossless
}
