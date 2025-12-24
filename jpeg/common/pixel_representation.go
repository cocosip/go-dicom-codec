package common

// DetectActualPixelRepresentation analyzes pixel values considering the current PixelRepresentation tag.
// Returns whether the data actually contains negative values when interpreted as signed.
//
// This is critical for JPEG encoding:
//   - If data is tagged PR=1 but all values are positive when interpreted, NO shift needed
//   - If data is tagged PR=1 and has negative values when interpreted, shift IS needed
//   - If data is tagged PR=0, never shift
//
// Parameters:
//   - pixelData: raw pixel bytes (little-endian for 16-bit)
//   - bitsStored: number of significant bits (8, 12, 16, etc.)
//   - currentPR: current PixelRepresentation tag value (0=unsigned, 1=signed)
//
// Returns:
//   - hasNegatives: true if data contains negative values when interpreted per currentPR
//   - minVal, maxVal: value range when interpreted as signed (if PR=1) or unsigned (if PR=0)
func DetectActualPixelRepresentation(pixelData []byte, bitsStored int, currentPR int) (hasNegatives bool, minVal, maxVal int32) {
	if bitsStored <= 0 || bitsStored > 16 || len(pixelData) == 0 {
		return false, 0, 0
	}

	signBit := int32(1) << (bitsStored - 1)

	// For 8-bit data
	if bitsStored <= 8 {
		minVal, maxVal = 255, -128

		for _, b := range pixelData {
			val := int32(b)

			// Interpret according to currentPR
			if currentPR == 1 && val >= signBit {
				val -= (1 << bitsStored) // Convert to signed
			}

			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}

		return minVal < 0, minVal, maxVal
	}

	// For 9-16 bit data (stored in 2 bytes)
	if len(pixelData) < 2 {
		return false, 0, 0
	}

	minVal, maxVal = 65535, -32768

	for i := 0; i < len(pixelData)/2; i++ {
		raw := uint16(pixelData[i*2]) | uint16(pixelData[i*2+1])<<8
		val := int32(raw)

		// Interpret according to currentPR
		if currentPR == 1 && val >= signBit {
			val -= (1 << bitsStored) // Convert to signed
		}

		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	// Has negatives if interpreted value goes below 0
	return minVal < 0, minVal, maxVal
}

// ConvertSignedToUnsigned converts pixel data from signed to unsigned representation.
// For signed data in range [-2^(n-1), 2^(n-1)-1], converts to unsigned [0, 2^n-1].
//
// Parameters:
//   - pixelData: raw pixel bytes (little-endian for 16-bit), will be modified in-place
//   - bitsStored: number of significant bits
func ConvertSignedToUnsigned(pixelData []byte, bitsStored int) {
	if bitsStored <= 0 || bitsStored > 16 || len(pixelData) < 2 {
		return
	}

	offset := int32(1) << (bitsStored - 1)

	if bitsStored <= 8 {
		for i := 0; i < len(pixelData); i++ {
			val := int32(pixelData[i])
			// If interpreted as signed and negative, add offset
			if val >= offset {
				val -= (1 << bitsStored)
			}
			// Convert to unsigned range
			val += offset
			pixelData[i] = byte(val)
		}
	} else {
		for i := 0; i < len(pixelData)/2; i++ {
			raw := uint16(pixelData[i*2]) | uint16(pixelData[i*2+1])<<8
			val := int32(raw)

			// If interpreted as signed and negative, convert
			if val >= offset {
				val -= (1 << bitsStored)
			}
			// Convert to unsigned range
			val += offset

			// Write back as little-endian
			pixelData[i*2] = byte(val & 0xFF)
			pixelData[i*2+1] = byte((val >> 8) & 0xFF)
		}
	}
}

// ConvertUnsignedToSigned converts pixel data from unsigned to signed representation.
// For unsigned data in range [0, 2^n-1], converts to signed [-2^(n-1), 2^(n-1)-1].
//
// Parameters:
//   - pixelData: raw pixel bytes (little-endian for 16-bit), will be modified in-place
//   - bitsStored: number of significant bits
func ConvertUnsignedToSigned(pixelData []byte, bitsStored int) {
	if bitsStored <= 0 || bitsStored > 16 || len(pixelData) < 2 {
		return
	}

	offset := int32(1) << (bitsStored - 1)

	if bitsStored <= 8 {
		for i := 0; i < len(pixelData); i++ {
			val := int32(pixelData[i])
			// Convert from unsigned to signed range
			val -= offset
			// Wrap to unsigned byte range for storage
			if val < 0 {
				val += (1 << bitsStored)
			}
			pixelData[i] = byte(val)
		}
	} else {
		for i := 0; i < len(pixelData)/2; i++ {
			raw := uint16(pixelData[i*2]) | uint16(pixelData[i*2+1])<<8
			val := int32(raw)

			// Convert from unsigned to signed range
			val -= offset
			// Wrap to unsigned range for storage
			if val < 0 {
				val += (1 << bitsStored)
			}

			// Write back as little-endian
			pixelData[i*2] = byte(val & 0xFF)
			pixelData[i*2+1] = byte((val >> 8) & 0xFF)
		}
	}
}
