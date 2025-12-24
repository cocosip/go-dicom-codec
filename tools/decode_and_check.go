package main

import (
	"fmt"
	"os"

	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"

	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: decode_and_check <compressed.dcm>")
		return
	}

	filePath := os.Args[1]
	fmt.Printf("Decoding and checking: %s\n\n", filePath)

	// Parse DICOM file
	result, err := parser.ParseFile(filePath,
		parser.WithReadOption(parser.ReadAll),
		parser.WithLargeObjectSize(100*1024*1024),
	)
	if err != nil {
		fmt.Printf("ERROR: Failed to parse: %v\n", err)
		return
	}

	ds := result.Dataset
	ts := result.TransferSyntax

	// Get image metadata
	rows := ds.TryGetUInt16(tag.Rows, 0)
	cols := ds.TryGetUInt16(tag.Columns, 0)
	bitsStored := ds.TryGetUInt16(tag.BitsStored, 0)
	pixelRep := ds.TryGetUInt16(tag.PixelRepresentation, 0)

	fmt.Printf("Transfer Syntax: %s\n", ts.UID().UID())
	fmt.Printf("Dimensions: %d x %d\n", cols, rows)
	fmt.Printf("Bits Stored: %d\n", bitsStored)
	fmt.Printf("Pixel Representation: %d (%s)\n\n", pixelRep, func() string {
		if pixelRep == 0 {
			return "unsigned"
		}
		return "signed"
	}())

	if !ts.IsEncapsulated() {
		fmt.Println("File is not compressed, nothing to decode")
		return
	}

	// Get pixel data
	pd, ok := ds.Get(tag.PixelData)
	if !ok {
		fmt.Println("ERROR: No PixelData element found")
		return
	}

	// Extract compressed data
	var compressedData []byte
	switch v := pd.(type) {
	case *element.OtherByteFragment:
		frags := v.Fragments()
		for _, frag := range frags {
			compressedData = append(compressedData, frag.Data()...)
		}
	case *element.OtherWordFragment:
		frags := v.Fragments()
		for _, frag := range frags {
			compressedData = append(compressedData, frag.Data()...)
		}
	default:
		fmt.Printf("ERROR: Unexpected pixel data type: %T\n", pd)
		return
	}

	fmt.Printf("Compressed size: %d bytes\n", len(compressedData))

	// Find codec
	registry := codec.GetGlobalRegistry()
	codecs := registry.GetCodecs()

	var foundCodec codec.Codec
	for _, c := range codecs {
		if c.UID() == ts.UID().UID() {
			foundCodec = c
			break
		}
	}

	if foundCodec == nil {
		fmt.Printf("ERROR: No codec found for UID: %s\n", ts.UID().UID())
		return
	}

	fmt.Printf("Using codec: %s\n\n", foundCodec.Name())

	// Decode (using transcoder which handles the decode)
	transcoder := codec.NewTranscoder(ts, nil, codec.WithCodecRegistry(registry))
	newDS, err := transcoder.TranscodeToNative(ds)
	if err != nil {
		fmt.Printf("ERROR: Decode failed: %v\n", err)
		return
	}

	// Get decoded pixel data
	decodedPD, ok := newDS.Get(tag.PixelData)
	if !ok {
		fmt.Println("ERROR: No decoded PixelData")
		return
	}

	var pixelData []byte
	switch v := decodedPD.(type) {
	case *element.OtherByte:
		pixelData = v.GetData()
	case *element.OtherWord:
		pixelData = v.GetData()
	default:
		fmt.Printf("ERROR: Unexpected decoded pixel data type: %T\n", decodedPD)
		return
	}

	fmt.Printf("✓ Decode successful!\n")
	fmt.Printf("  Decoded size: %d bytes\n\n", len(pixelData))

	// Analyze pixel values
	analyzePixels16Bit(pixelData, int(bitsStored), pixelRep)
}

func analyzePixels16Bit(data []byte, bitsStored int, pixelRep uint16) {
	if len(data) < 2 {
		fmt.Println("No pixel data")
		return
	}

	var minVal, maxVal int32 = 65535, -32768
	var minRaw, maxRaw uint16 = 65535, 0

	signBit := uint16(1) << (bitsStored - 1)

	for i := 0; i < len(data)/2; i++ {
		// Read as little-endian uint16
		raw := uint16(data[i*2]) | uint16(data[i*2+1])<<8

		if raw < minRaw {
			minRaw = raw
		}
		if raw > maxRaw {
			maxRaw = raw
		}

		// Interpret based on PixelRepresentation
		val := int32(raw)
		if pixelRep == 1 && val >= int32(signBit) {
			val -= (1 << bitsStored)
		}

		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	fmt.Printf("=== DECODED PIXEL VALUE ANALYSIS ===\n")
	fmt.Printf("Raw value range: [%d, %d] (as unsigned)\n", minRaw, maxRaw)
	fmt.Printf("Interpreted range: [%d, %d] (PR=%d)\n\n", minVal, maxVal, pixelRep)

	// Check if representation is correct
	if minRaw >= signBit {
		fmt.Printf("✓ All raw values >= %d (sign bit threshold)\n", signBit)
		fmt.Println("✓ Data uses full range, representation appears correct")
	} else if maxRaw < signBit {
		fmt.Printf("✓ All raw values < %d (sign bit threshold)\n", signBit)
		if pixelRep == 1 {
			fmt.Println("⚠️  WARNING: PixelRepresentation=1 (signed) but values fit in unsigned range")
			fmt.Println("   This may cause display issues in some viewers")
		} else {
			fmt.Println("✓ PixelRepresentation=0 (unsigned) matches data")
		}
	} else {
		fmt.Printf("✓ Values span sign bit threshold (some >= %d)\n", signBit)
		fmt.Println("✓ Representation appears correct for full range")
	}

	// Sample values
	fmt.Println("\n=== SAMPLE VALUES (first 10 pixels) ===")
	for i := 0; i < 10 && i*2 < len(data); i++ {
		raw := uint16(data[i*2]) | uint16(data[i*2+1])<<8
		val := int32(raw)
		if pixelRep == 1 && val >= int32(signBit) {
			val -= (1 << bitsStored)
		}
		fmt.Printf("  Pixel %d: raw=%d, interpreted=%d\n", i, raw, val)
	}
}
