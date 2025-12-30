package lossless

import (
	"fmt"
	"testing"
)

func TestMapUnmapSymmetry(t *testing.T) {
	testCases := []int{-10, -5, -1, 0, 1, 5, 10, 20, -20, 100, -100}

	for _, original := range testCases {
		mapped := MapErrorValue(original)
		unmapped := UnmapErrorValue(mapped)

		fmt.Printf("原始=%d, 映射=%d, 反映射=%d\n", original, mapped, unmapped)

		if unmapped != original {
			t.Errorf("不对称! 原始=%d, 映射=%d, 反映射=%d", original, mapped, unmapped)
		}
	}
}
