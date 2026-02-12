// Package jpeg2000 provides tests and benchmarks for the JPEG2000 implementation.
package jpeg2000

import (
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/testdata"
)

// BenchmarkDecoderSmallImage benchmarks decoding a small 8x8 image
func BenchmarkDecoderSmallImage(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(8, 8, 8)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

// BenchmarkDecoderMediumImage benchmarks decoding a 64x64 image
func BenchmarkDecoderMediumImage(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(64, 64, 8)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

// BenchmarkDecoderLargeImage benchmarks decoding a 256x256 image
func BenchmarkDecoderLargeImage(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(256, 256, 8)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

// BenchmarkDecoder512Image benchmarks decoding a 512x512 image
func BenchmarkDecoder512Image(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(512, 512, 12)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

// BenchmarkDecoderDifferentBitDepths benchmarks different bit depths
func BenchmarkDecoderBitDepth8(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 8)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

func BenchmarkDecoderBitDepth12(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 12)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

func BenchmarkDecoderBitDepth16(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 16)
	decoder := NewDecoder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.Decode(data)
	}
}

// BenchmarkDecoderGetPixelData benchmarks pixel data extraction
func BenchmarkDecoderGetPixelData8bit(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 8)
	decoder := NewDecoder()
	_ = decoder.Decode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.GetPixelData()
	}
}

func BenchmarkDecoderGetPixelData16bit(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 16)
	decoder := NewDecoder()
	_ = decoder.Decode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decoder.GetPixelData()
	}
}

// BenchmarkDecoderFullPipeline benchmarks the complete decode + get pixel data pipeline
func BenchmarkDecoderFullPipeline(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(256, 256, 12)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := NewDecoder()
		_ = decoder.Decode(data)
		_ = decoder.GetPixelData()
	}
}

// BenchmarkCodestreamParsing benchmarks just the codestream parsing
func BenchmarkCodestreamParsing(b *testing.B) {
	data := testdata.GenerateSimpleJ2K(128, 128, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := NewDecoder()
		_ = decoder.Decode(data)
	}
}
