package main

import (
	"fmt"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	j2k "github.com/cocosip/go-dicom-codec/jpeg2000"
	lossless "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	lossy "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func main() {
    fmt.Println("=== JPEG 2000 Part 2 Multi-component Example ===")

    // Build a binding: components {0,1}, identity matrix, offsets {+5,-5}
    b := j2k.NewMCTBinding().
        Assoc(2). // Matrix then Offset
        Components([]uint16{0, 1}).
        Matrix([][]float64{{1, 0}, {0, 1}}).
        Inverse([][]float64{{1, 0}, {0, 1}}).
        Offsets([]int32{5, -5}).
        ElementType(1). // float32
        MCOPrecision(1). // reversible flag
        Build()

	// Lossy path
	{
		p := lossy.NewLossyParameters().WithQuality(90).WithMCTBindings([]j2k.MCTBindingParams{b})
		enc := lossy.NewCodec(90)

		// Prepare dummy PixelData (RGB 8x8)
		w, h, comps := 8, 8, 3
		n := w * h
		src := make([]byte, n*comps)
		frameInfo := &imagetypes.FrameInfo{
			Width:           uint16(w),
			Height:          uint16(h),
			BitsAllocated:   8,
			BitsStored:      8,
			HighBit:         7,
			SamplesPerPixel: uint16(comps),
		}
		pdIn := codecHelpers.NewTestPixelData(frameInfo)
		pdIn.AddFrame(src)
		pdOut := codecHelpers.NewTestPixelData(frameInfo)
		_ = enc.Encode(pdIn, pdOut, p)
		fmt.Println("Lossy encode with Part 2 bindings completed (markers written)")
	}

	// Lossless path
	{
		p := lossless.NewLosslessParameters().WithNumLevels(0).WithMCTBindings([]j2k.MCTBindingParams{b})
		enc := lossless.NewCodec()

		// Prepare dummy PixelData (2 components 8x8)
		w, h, comps := 8, 8, 2
		n := w * h
		src := make([]byte, n*comps)
		frameInfo := &imagetypes.FrameInfo{
			Width:           uint16(w),
			Height:          uint16(h),
			BitsAllocated:   8,
			BitsStored:      8,
			HighBit:         7,
			SamplesPerPixel: uint16(comps),
		}
		pdIn := codecHelpers.NewTestPixelData(frameInfo)
		pdIn.AddFrame(src)
		pdOut := codecHelpers.NewTestPixelData(frameInfo)
		_ = enc.Encode(pdIn, pdOut, p)
		fmt.Println("Lossless encode with Part 2 bindings completed (markers written)")
	}
}

