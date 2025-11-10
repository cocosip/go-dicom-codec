package extended

// Decode decodes JPEG Extended (SOF1) data
func Decode(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	// Use simplified implementation that leverages Go's standard JPEG decoder
	return DecodeSimple(jpegData)
}
