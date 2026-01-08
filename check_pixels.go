package main

import (
	"fmt"
	"hash/crc32"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"

	// Register codecs
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codec/jpegls/lossless"
)

func stats(path string) {
	res, err := parser.ParseFile(path, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		fmt.Printf("%s: parse error: %v\n", path, err)
		return
	}
	ds := res.Dataset
	if res.TransferSyntax != nil && res.TransferSyntax.IsEncapsulated() {
		tr := codec.NewTranscoder(res.TransferSyntax, transfer.ExplicitVRLittleEndian)
		newDS, err := tr.Transcode(ds)
		if err != nil {
			fmt.Printf("%s: transcode error: %v\n", path, err)
			return
		}
		ds = newDS
	}

	pd, err := imaging.CreatePixelData(ds)
	if err != nil {
		fmt.Printf("%s: pixel error: %v\n", path, err)
		return
	}
	min1, max1, err := pd.MinMax(true)
	if err != nil {
		fmt.Printf("%s: minmax error: %v\n", path, err)
		return
	}
	data, _ := pd.GetFrame(0)
	fmt.Printf("%s: frames=%d bitsStored=%d PR=%d min=%g max=%g crc32= %v \n",
		path, pd.FrameCount(), pd.Info.BitsStored, pd.Info.PixelRepresentation, min1, max1, crc32.ChecksumIEEE(data))
}

func main() {
	fmt.Println("Testing original file:")
	stats(`D:\1.dcm`)

	fmt.Println("\nTesting our JPEG-LS encoding:")
	stats(`D:\1_transcoded\1_jpegls_lossless.dcm`)

	fmt.Println("\nTesting fo-dicom JPEG-LS encoding:")
	stats(`D:\11.dcm`)
}
