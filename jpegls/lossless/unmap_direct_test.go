package lossless

import (
	"fmt"
	"testing"
)

func TestUnmapDirect(t *testing.T) {
	mappedError := 19

	const intBitCount = 32

	// Step by step
	fmt.Printf("mappedError = %d (binary: %032b)\n", mappedError, uint32(mappedError))

	// Extract sign
	temp1 := uint32(mappedError) << (intBitCount - 1)
	fmt.Printf("uint32(mappedError) << 31 = %032b = %d\n", temp1, temp1)

	temp2 := int32(temp1)
	fmt.Printf("int32(temp1) = %d\n", temp2)

	sign := temp2 >> (intBitCount - 1)
	fmt.Printf("sign = temp2 >> 31 = %d (binary: %032b)\n", sign, uint32(sign))

	// Shift mapped error
	shifted := mappedError >> 1
	fmt.Printf("mappedError >> 1 = %d\n", shifted)

	// XOR
	errValue := int(sign) ^ shifted
	fmt.Printf("sign ^ shifted = %d ^ %d = %d\n", sign, shifted, errValue)

	// Expected
	expected := -10
	if errValue != expected {
		t.Errorf("UnmapErrorValue(19) = %d, expected %d", errValue, expected)
	}

	// Also test the actual function
	actual := UnmapErrorValue(19)
	fmt.Printf("\n实际UnmapErrorValue(19) = %d\n", actual)
	if actual != expected {
		t.Errorf("UnmapErrorValue(19) = %d, expected %d", actual, expected)
	}
}
