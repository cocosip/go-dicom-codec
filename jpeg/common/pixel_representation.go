package common

// DetectActualPixelRepresentation analyzes actual pixel values to determine if signed representation is needed.
// This is more reliable than trusting the DICOM PixelRepresentation tag, which may be incorrect.
//
// Logic:
//   - If all raw values are < 2^(bitsStored-1), data fits in unsigned range [0, 2^(n-1)-1], no sign bit needed
//   - If any raw values >= 2^(bitsStored-1), data needs the full range (either unsigned [0, 2^n-1] or signed with negatives)
//     For JPEG encoding, we treat this as needing full unsigned range
//
// Parameters:
//   - pixelData: raw pixel bytes (little-endian for 16-bit)
//   - bitsStored: number of significant bits (8, 12, 16, etc.)
//
// Returns:
//   - needsFullRange: true if data uses values >= 2^(bitsStored-1), requiring full range encoding
//   - minVal, maxVal: value range when interpreted as raw unsigned values
func DetectActualPixelRepresentation(pixelData []byte, bitsStored int) (needsFullRange bool, minVal, maxVal uint32) {
	if bitsStored <= 0 || bitsStored > 16 || len(pixelData) == 0 {
		return false, 0, 0
	}

	signBitThreshold := uint32(1) << (bitsStored - 1)

	// For 8-bit data
	if bitsStored <= 8 {
		var min, max uint8 = 255, 0

		for _, b := range pixelData {
			if b < min {
				min = b
			}
			if b > max {
				max = b
			}
		}

		minVal, maxVal = uint32(min), uint32(max)

		// If max value >= sign bit threshold, needs full range
		return maxVal >= signBitThreshold, minVal, maxVal
	}

	// For 9-16 bit data (stored in 2 bytes)
	if len(pixelData) < 2 {
		return false, 0, 0
	}

	var min, max uint16 = 65535, 0

	for i := 0; i < len(pixelData)/2; i++ {
		val := uint16(pixelData[i*2]) | uint16(pixelData[i*2+1])<<8
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	minVal, maxVal = uint32(min), uint32(max)

	// If max value >= sign bit threshold, needs full range
	return maxVal >= signBitThreshold, minVal, maxVal
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
