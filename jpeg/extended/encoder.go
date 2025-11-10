package extended

// Encode encodes pixel data to JPEG Extended format (SOF1)
// components: 1 for grayscale, 3 for RGB
// bitDepth: 8 or 12 bits per sample
// quality: 1-100, where 100 is best quality
func Encode(pixelData []byte, width, height, components, bitDepth, quality int) ([]byte, error) {
	// Use simplified implementation that leverages Go's standard JPEG encoder
	return EncodeSimple(pixelData, width, height, components, bitDepth, quality)
}
