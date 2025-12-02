package lossy

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/uid"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements the JPEG 2000 Lossy codec
// Transfer Syntax UID: 1.2.840.10008.1.2.4.91
type Codec struct {
	transferSyntax *transfer.Syntax
	quality        int // Default quality (1-100)
}

// NewCodec creates a new JPEG 2000 Lossy codec
// quality: 1-100, where 100 is near-lossless (default: 80)
func NewCodec(quality int) *Codec {
	return NewCodecWithTransferSyntax(transfer.JPEG2000, quality)
}

// NewCodecWithTransferSyntax allows constructing the codec for alternate JPEG 2000 transfer syntaxes.
func NewCodecWithTransferSyntax(ts *transfer.Syntax, quality int) *Codec {
	if quality < 1 || quality > 100 {
		quality = 80 // default
	}
	return &Codec{
		transferSyntax: ts, // Lossy or Lossless
		quality:        quality,
	}
}

// Name returns the codec name
func (c *Codec) Name() string {
	return fmt.Sprintf("JPEG 2000 Lossy (Quality %d)", c.quality)
}

// TransferSyntax returns the transfer syntax this codec handles
func (c *Codec) TransferSyntax() *transfer.Syntax {
	return c.transferSyntax
}

// Encode encodes pixel data to JPEG 2000 Lossy format
func (c *Codec) Encode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	if isHTJ2K(c.transferSyntax) {
		return fmt.Errorf("HTJ2K lossy encode not implemented for transfer syntax %s", c.transferSyntax.UID().UID())
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Get encoding parameters
	var lossyParams *JPEG2000LossyParameters
	if params != nil {
		// Try to use typed parameters if provided
		if jp, ok := params.(*JPEG2000LossyParameters); ok {
			lossyParams = jp
		} else {
			// Fallback: create from generic parameters
			lossyParams = NewLossyParameters()
			if q := params.GetParameter("quality"); q != nil {
				if qInt, ok := q.(int); ok && qInt >= 1 && qInt <= 100 {
					lossyParams.Quality = qInt
				}
			}
			if nl := params.GetParameter("numLevels"); nl != nil {
				if nlInt, ok := nl.(int); ok && nlInt >= 0 && nlInt <= 6 {
					lossyParams.NumLevels = nlInt
				}
			}
		}
	} else {
		// Use codec defaults
		lossyParams = NewLossyParameters()
		lossyParams.Quality = c.quality
	}

	// Validate parameters
	lossyParams.Validate()

	// Create encoding parameters for lossy compression
	encParams := jpeg2000.DefaultEncodeParams(
		int(src.Width),
		int(src.Height),
		int(src.SamplesPerPixel),
		int(src.BitsStored),
		src.PixelRepresentation != 0,
	)

	// Configure for lossy compression (9/7 wavelet)
	encParams.Lossless = false
	encParams.NumLevels = lossyParams.NumLevels
	encParams.Quality = lossyParams.Quality

	// Create encoder
	encoder := jpeg2000.NewEncoder(encParams)

	// Encode
	encoded, err := encoder.Encode(src.Data)
	if err != nil {
		return fmt.Errorf("JPEG 2000 encode failed: %w", err)
	}

	// Set destination data
	dst.Data = encoded
	dst.Width = src.Width
	dst.Height = src.Height
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = src.BitsAllocated
	dst.BitsStored = src.BitsStored
	dst.HighBit = src.HighBit
	dst.SamplesPerPixel = src.SamplesPerPixel
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = c.transferSyntax.UID().UID()

	return nil
}

// Decode decodes JPEG 2000 Lossy data to uncompressed pixel data
func (c *Codec) Decode(src *codec.PixelData, dst *codec.PixelData, params codec.Parameters) error {
	if src == nil || dst == nil {
		return fmt.Errorf("source and destination PixelData cannot be nil")
	}

	if isHTJ2K(c.transferSyntax) {
		return fmt.Errorf("HTJ2K lossy decode not implemented for transfer syntax %s", c.transferSyntax.UID().UID())
	}

	// Validate input data
	if len(src.Data) == 0 {
		return fmt.Errorf("source pixel data is empty")
	}

	// Create decoder
	decoder := jpeg2000.NewDecoder()

	// Decode (decoder automatically detects lossy vs lossless from codestream)
	if err := decoder.Decode(src.Data); err != nil {
		return fmt.Errorf("JPEG 2000 decode failed: %w", err)
	}

	// Set destination data
	dst.Data = decoder.GetPixelData()
	dst.Width = uint16(decoder.Width())
	dst.Height = uint16(decoder.Height())
	dst.NumberOfFrames = src.NumberOfFrames
	dst.BitsAllocated = src.BitsAllocated
	dst.BitsStored = uint16(decoder.BitDepth())
	dst.HighBit = dst.BitsStored - 1
	dst.SamplesPerPixel = uint16(decoder.Components())
	dst.PixelRepresentation = src.PixelRepresentation
	dst.PlanarConfiguration = src.PlanarConfiguration
	dst.PhotometricInterpretation = src.PhotometricInterpretation
	dst.TransferSyntaxUID = transfer.ExplicitVRLittleEndian.UID().UID() // Decoded to uncompressed

	return nil
}

// RegisterJPEG2000LossyCodec registers the JPEG 2000 Lossy codec with the global registry
func RegisterJPEG2000LossyCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodec(80) // Default quality: 80
	registry.RegisterCodec(transfer.JPEG2000, j2kCodec)
}

// RegisterJPEG2000MultiComponentCodec registers JPEG 2000 Part 2 multi-component codec.
func RegisterJPEG2000MultiComponentCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodecWithTransferSyntax(transfer.JPEG2000Part2MultiComponent, 80)
	registry.RegisterCodec(transfer.JPEG2000Part2MultiComponent, j2kCodec)
}

// RegisterHTJ2KCodec registers HTJ2K codec placeholder.
func RegisterHTJ2KCodec() {
	registry := codec.GetGlobalRegistry()
	j2kCodec := NewCodecWithTransferSyntax(transfer.HTJ2K, 80)
	registry.RegisterCodec(transfer.HTJ2K, j2kCodec)
}

func init() {
	RegisterJPEG2000LossyCodec()
	RegisterJPEG2000MultiComponentCodec()
	RegisterHTJ2KCodec()
}

func isHTJ2K(ts *transfer.Syntax) bool {
	if ts == nil {
		return false
	}
	u := ts.UID().UID()
	return u == uid.HTJ2KLossless.UID() ||
		u == uid.HTJ2KLosslessRPCL.UID() ||
		u == uid.HTJ2K.UID()
}
