package codec

import (
	"github.com/cocosip/go-dicom/pkg/imaging/types"
)

// TestPixelData is a simple implementation of types.PixelData for testing
type TestPixelData struct {
	frames    [][]byte
	frameInfo *types.FrameInfo
}

// NewTestPixelData creates a new TestPixelData with the given frame info
func NewTestPixelData(frameInfo *types.FrameInfo) *TestPixelData {
	return &TestPixelData{
		frames:    make([][]byte, 0),
		frameInfo: frameInfo,
	}
}

// GetFrame returns the pixel data for the specified frame (0-indexed)
func (p *TestPixelData) GetFrame(frameIndex int) ([]byte, error) {
	if frameIndex < 0 || frameIndex >= len(p.frames) {
		return nil, nil
	}
	return p.frames[frameIndex], nil
}

// AddFrame appends a new frame to the pixel data
func (p *TestPixelData) AddFrame(frameData []byte) error {
	p.frames = append(p.frames, frameData)
	return nil
}

// FrameCount returns the number of frames in the pixel data
func (p *TestPixelData) FrameCount() int {
	return len(p.frames)
}

// GetFrameInfo returns frame metadata for codec operations
func (p *TestPixelData) GetFrameInfo() *types.FrameInfo {
	return p.frameInfo
}

// IsEncapsulated returns true if pixel data is encapsulated (compressed)
func (p *TestPixelData) IsEncapsulated() bool {
	return false
}
