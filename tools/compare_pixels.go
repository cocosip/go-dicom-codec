package main

import (
	"fmt"
	"os"

	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"

	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: compare_pixels <original.dcm> <transcoded.dcm>")
		return
	}

	originalPath := os.Args[1]
	transcodedPath := os.Args[2]

	fmt.Printf("Comparing pixels:\n")
	fmt.Printf("  Original: %s\n", originalPath)
	fmt.Printf("  Transcoded: %s\n\n", transcodedPath)

	// Read original
	origPixels, origPR, err := getPixels(originalPath)
	if err != nil {
		fmt.Printf("ERROR reading original: %v\n", err)
		return
	}

	// Read transcoded
	transPixels, transPR, err := getPixels(transcodedPath)
	if err != nil {
		fmt.Printf("ERROR reading transcoded: %v\n", err)
		return
	}

	// Compare
	fmt.Printf("Original PixelRepresentation: %d\n", origPR)
	fmt.Printf("Transcoded PixelRepresentation: %d\n\n", transPR)

	if len(origPixels) != len(transPixels) {
		fmt.Printf("ERROR: Size mismatch: %d vs %d bytes\n", len(origPixels), len(transPixels))
		return
	}

	// Sample comparison
	fmt.Println("=== SAMPLE PIXEL COMPARISON (first 10) ===")
	matches := 0
	mismatches := 0
	for i := 0; i < 10 && i*2 < len(origPixels); i++ {
		origVal := uint16(origPixels[i*2]) | uint16(origPixels[i*2+1])<<8
		transVal := uint16(transPixels[i*2]) | uint16(transPixels[i*2+1])<<8

		match := ""
		if origVal == transVal {
			match = "✓"
			matches++
		} else {
			match = "✗"
			mismatches++
		}
		fmt.Printf("  Pixel %d: orig=%d, trans=%d %s\n", i, origVal, transVal, match)
	}

	// Full comparison
	fmt.Println("\n=== FULL COMPARISON ===")
	totalMismatches := 0
	for i := 0; i < len(origPixels); i++ {
		if origPixels[i] != transPixels[i] {
			totalMismatches++
		}
	}

	if totalMismatches == 0 {
		fmt.Println("✓ PERFECT MATCH: All pixels identical (lossless)")
	} else {
		fmt.Printf("✗ MISMATCH: %d bytes differ (%.2f%%)\n",
			totalMismatches, float64(totalMismatches)/float64(len(origPixels))*100)
	}
}

func getPixels(filePath string) ([]byte, uint16, error) {
	result, err := parser.ParseFile(filePath,
		parser.WithReadOption(parser.ReadAll),
		parser.WithLargeObjectSize(100*1024*1024),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("parse failed: %w", err)
	}

	ds := result.Dataset
	ts := result.TransferSyntax
	pr := ds.TryGetUInt16(tag.PixelRepresentation, 0)

	// If compressed, transcode to native
	if ts.IsEncapsulated() {
		registry := codec.GetGlobalRegistry()
		transcoder := codec.NewTranscoder(ts, transfer.ExplicitVRLittleEndian, codec.WithCodecRegistry(registry))
		newDS, err := transcoder.Transcode(ds)
		if err != nil {
			return nil, 0, fmt.Errorf("transcode failed: %w", err)
		}
		ds = newDS
	}

	// Get pixel data
	pd, ok := ds.Get(tag.PixelData)
	if !ok {
		return nil, 0, fmt.Errorf("no pixel data")
	}

	var pixelData []byte
	switch v := pd.(type) {
	case *element.OtherByte:
		pixelData = v.GetData()
	case *element.OtherWord:
		pixelData = v.GetData()
	default:
		return nil, 0, fmt.Errorf("unexpected pixel data type: %T", pd)
	}

	return pixelData, pr, nil
}
