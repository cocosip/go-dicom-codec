package common

// JPEG marker constants
const (
	// Start of Image
	MarkerSOI = 0xFFD8

	// End of Image
	MarkerEOI = 0xFFD9

	// Start of Frame markers
	MarkerSOF0  = 0xFFC0 // Baseline DCT
	MarkerSOF1  = 0xFFC1 // Extended Sequential DCT
	MarkerSOF2  = 0xFFC2 // Progressive DCT
	MarkerSOF3  = 0xFFC3 // Lossless (Sequential)
	MarkerSOF5  = 0xFFC5 // Differential Sequential DCT
	MarkerSOF6  = 0xFFC6 // Differential Progressive DCT
	MarkerSOF7  = 0xFFC7 // Differential Lossless
	MarkerSOF9  = 0xFFC9 // Extended Sequential DCT, Arithmetic coding
	MarkerSOF10 = 0xFFCA // Progressive DCT, Arithmetic coding
	MarkerSOF11 = 0xFFCB // Lossless, Arithmetic coding
	MarkerSOF13 = 0xFFCD // Differential Sequential DCT, Arithmetic coding
	MarkerSOF14 = 0xFFCE // Differential Progressive DCT, Arithmetic coding
	MarkerSOF15 = 0xFFCF // Differential Lossless, Arithmetic coding

	// Define Huffman Table
	MarkerDHT = 0xFFC4

	// Define Quantization Table
	MarkerDQT = 0xFFDB

	// Define Restart Interval
	MarkerDRI = 0xFFDD

	// Start of Scan
	MarkerSOS = 0xFFDA

	// Application segments
	MarkerAPP0  = 0xFFE0
	MarkerAPP1  = 0xFFE1
	MarkerAPP2  = 0xFFE2
	MarkerAPP3  = 0xFFE3
	MarkerAPP4  = 0xFFE4
	MarkerAPP5  = 0xFFE5
	MarkerAPP6  = 0xFFE6
	MarkerAPP7  = 0xFFE7
	MarkerAPP8  = 0xFFE8
	MarkerAPP9  = 0xFFE9
	MarkerAPP10 = 0xFFEA
	MarkerAPP11 = 0xFFEB
	MarkerAPP12 = 0xFFEC
	MarkerAPP13 = 0xFFED
	MarkerAPP14 = 0xFFEE
	MarkerAPP15 = 0xFFEF

	// Comment
	MarkerCOM = 0xFFFE

	// Restart markers
	MarkerRST0 = 0xFFD0
	MarkerRST1 = 0xFFD1
	MarkerRST2 = 0xFFD2
	MarkerRST3 = 0xFFD3
	MarkerRST4 = 0xFFD4
	MarkerRST5 = 0xFFD5
	MarkerRST6 = 0xFFD6
	MarkerRST7 = 0xFFD7
)

// IsSOF returns true if the marker is a Start of Frame marker
func IsSOF(marker uint16) bool {
	return (marker >= MarkerSOF0 && marker <= MarkerSOF3) ||
		(marker >= MarkerSOF5 && marker <= MarkerSOF7) ||
		(marker >= MarkerSOF9 && marker <= MarkerSOF11) ||
		(marker >= MarkerSOF13 && marker <= MarkerSOF15)
}

// IsRST returns true if the marker is a Restart marker
func IsRST(marker uint16) bool {
	return marker >= MarkerRST0 && marker <= MarkerRST7
}

// HasLength returns true if the marker is followed by a length field
func HasLength(marker uint16) bool {
	// Markers without length: SOI, EOI, RSTn, and 0xFF00 (escaped 0xFF)
	if marker == MarkerSOI || marker == MarkerEOI {
		return false
	}
	if IsRST(marker) {
		return false
	}
	return true
}
