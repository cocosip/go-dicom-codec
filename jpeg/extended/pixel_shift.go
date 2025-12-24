package extended

import (
	"encoding/binary"
	"fmt"
)

// shiftSignedToUnsigned converts signed samples into unsigned domain based on BitsStored.
func shiftSignedToUnsigned(data []byte, bitsStored uint16) ([]byte, error) {
	if bitsStored == 0 || bitsStored > 16 {
		return nil, fmt.Errorf("unsupported BitsStored=%d", bitsStored)
	}
	bytesPerSample := 1
	if bitsStored > 8 {
		bytesPerSample = 2
	}
	if len(data)%bytesPerSample != 0 {
		return nil, fmt.Errorf("frame length %d not aligned to %d bytes/sample", len(data), bytesPerSample)
	}
	offset := int32(1) << (bitsStored - 1)
	out := make([]byte, len(data))
	if bytesPerSample == 1 {
		for i := 0; i < len(data); i++ {
			signedVal := int32(int8(data[i]))
			unsignedVal := signedVal + offset
			out[i] = byte(unsignedVal & 0xFF)
		}
	} else {
		for i := 0; i < len(data); i += 2 {
			signedVal := int32(int16(binary.LittleEndian.Uint16(data[i:])))
			unsignedVal := signedVal + offset
			binary.LittleEndian.PutUint16(out[i:], uint16(unsignedVal))
		}
	}
	return out, nil
}

// shiftUnsignedToSigned converts unsigned samples back to signed domain based on BitsStored.
func shiftUnsignedToSigned(data []byte, bitsStored uint16) ([]byte, error) {
	if bitsStored == 0 || bitsStored > 16 {
		return nil, fmt.Errorf("unsupported BitsStored=%d", bitsStored)
	}
	bytesPerSample := 1
	if bitsStored > 8 {
		bytesPerSample = 2
	}
	if len(data)%bytesPerSample != 0 {
		return nil, fmt.Errorf("frame length %d not aligned to %d bytes/sample", len(data), bytesPerSample)
	}
	offset := int32(1) << (bitsStored - 1)
	out := make([]byte, len(data))
	if bytesPerSample == 1 {
		for i := 0; i < len(data); i++ {
			unsignedVal := int32(data[i])
			signedVal := unsignedVal - offset
			out[i] = byte(int8(signedVal))
		}
	} else {
		for i := 0; i < len(data); i += 2 {
			unsignedVal := int32(binary.LittleEndian.Uint16(data[i:]))
			signedVal := unsignedVal - offset
			binary.LittleEndian.PutUint16(out[i:], uint16(int16(signedVal)))
		}
	}
	return out, nil
}
