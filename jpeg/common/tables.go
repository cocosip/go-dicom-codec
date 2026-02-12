package common

// Standard JPEG quantization tables

// DefaultLuminanceQuantTable is the standard luminance quantization table
var DefaultLuminanceQuantTable = [64]int32{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

// DefaultChrominanceQuantTable is the standard chrominance quantization table
var DefaultChrominanceQuantTable = [64]int32{
	17, 18, 24, 47, 99, 99, 99, 99,
	18, 21, 26, 66, 99, 99, 99, 99,
	24, 26, 56, 99, 99, 99, 99, 99,
	47, 66, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
}

// ScaleQuantTable scales a quantization table by quality factor (1-100)
func ScaleQuantTable(baseTable [64]int32, quality int) [64]int32 {
	var result [64]int32

	// Convert quality to scale factor
	// Quality 50 = no scaling
	// Quality < 50: increase quantization (lower quality, higher compression)
	// Quality > 50: decrease quantization (higher quality, lower compression)
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}

	for i := 0; i < 64; i++ {
		val := (baseTable[i]*int32(scale) + 50) / 100
		if val < 1 {
			val = 1
		}
		if val > 255 {
			val = 255
		}
		result[i] = val
	}

	return result
}

// Standard Huffman tables for baseline JPEG

// StandardDCLuminanceBits contains the number of codes for each length (DC luminance)
// This is the standard table used by libjpeg for baseline JPEG
var StandardDCLuminanceBits = [16]int{
	0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0,
}

// StandardDCLuminanceValues contains the Huffman values (DC luminance)
var StandardDCLuminanceValues = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15,
}

// StandardDCChrominanceBits contains the number of codes for each length (DC chrominance)
var StandardDCChrominanceBits = [16]int{
	0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0,
}

// StandardDCChrominanceValues contains the Huffman values (DC chrominance)
var StandardDCChrominanceValues = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
}

// StandardACLuminanceBits contains the number of codes for each length (AC luminance)
var StandardACLuminanceBits = [16]int{
	0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125,
}

// StandardACLuminanceValues contains the Huffman values (AC luminance)
var StandardACLuminanceValues = []byte{
	0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
	0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
	0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
	0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
	0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
	0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
	0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
	0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
	0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
	0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
	0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
	0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
	0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
	0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
	0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
	0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
	0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
	0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
	0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
	0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
	0xf9, 0xfa,
}

// StandardACChrominanceBits contains the number of codes for each length (AC chrominance)
var StandardACChrominanceBits = [16]int{
	0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119,
}

// StandardACChrominanceValues contains the Huffman values (AC chrominance)
var StandardACChrominanceValues = []byte{
	0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
	0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
	0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
	0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
	0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
	0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
	0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
	0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
	0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
	0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
	0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
	0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
	0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
	0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
	0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
	0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
	0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
	0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
	0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
	0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
	0xf9, 0xfa,
}

// Extended Huffman tables for high bit-depth (>12 bits) lossless JPEG
// These tables support category 0-16 for 16-bit precision

// ExtendedDCLuminanceBits contains the number of codes for each length (DC luminance, extended)
// Supports category 0-16 for bit depths up to 16
var ExtendedDCLuminanceBits = [16]int{
	0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 1, 1, 2, // Extended for cat 12-16 (adds one code of length 16)
}

// ExtendedDCLuminanceValues contains the Huffman values (DC luminance, extended)
var ExtendedDCLuminanceValues = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
}

// ExtendedDCChrominanceBits contains the number of codes for each length (DC chrominance, extended)
// Supports category 0-16 for bit depths up to 16
var ExtendedDCChrominanceBits = [16]int{
	0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 3, // Extended for cat 12-16 (adds category 16)
}

// ExtendedDCChrominanceValues contains the Huffman values (DC chrominance, extended)
var ExtendedDCChrominanceValues = []byte{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
}

// LosslessDCLuminanceBits defines the number of codes for each bit length for the luminance DC table (categories 0-15).
// Matches the lossless DC Huffman table used by libjpeg/fo-dicom; bits array specifies counts for lengths 1-16.
var LosslessDCLuminanceBits = [16]int{
	0, 2, 3, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0,
}

// LosslessDCLuminanceValues lists the luminance DC Huffman values in tree order for the lossless table (12 values total).
var LosslessDCLuminanceValues = []byte{
	0x00, 0x04, 0x02, 0x03, 0x05, 0x01, 0x06, 0x07, 0x0C, 0x0B, 0x08, 0x0F,
}

// LosslessDCChrominanceBits defines the number of codes for each bit length for the chrominance DC lossless table (categories 0-15).
var LosslessDCChrominanceBits = [16]int{
	0, 2, 3, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0,
}

// LosslessDCChrominanceValues lists the chrominance DC Huffman values in tree order for the lossless table.
var LosslessDCChrominanceValues = []byte{
	0x00, 0x04, 0x02, 0x03, 0x05, 0x01, 0x06, 0x07, 0x0C, 0x0B, 0x08, 0x0F,
}

// BuildStandardHuffmanTable builds a standard Huffman table
func BuildStandardHuffmanTable(bits [16]int, values []byte) *HuffmanTable {
	table := &HuffmanTable{
		Bits:   bits,
		Values: values,
	}
	_ = table.Build() // Build() always succeeds for standard tables
	return table
}
