package jpeg2000

import (
	"fmt"

	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
)

// TileLayout represents the tile grid layout
type TileLayout struct {
	// Image dimensions
	imageWidth  int
	imageHeight int

	// Tile dimensions
	tileWidth  int
	tileHeight int

	// Tile grid
	numTilesX int
	numTilesY int

	// Tile offsets
	tileOffsetX int
	tileOffsetY int
}

// NewTileLayout creates a tile layout from SIZ segment
func NewTileLayout(siz *codestream.SIZSegment) *TileLayout {
	layout := &TileLayout{
		imageWidth:  int(siz.Xsiz - siz.XOsiz),
		imageHeight: int(siz.Ysiz - siz.YOsiz),
		tileWidth:   int(siz.XTsiz),
		tileHeight:  int(siz.YTsiz),
		tileOffsetX: int(siz.XTOsiz),
		tileOffsetY: int(siz.YTOsiz),
	}

	// Calculate number of tiles
	layout.numTilesX = (layout.imageWidth + layout.tileWidth - 1) / layout.tileWidth
	layout.numTilesY = (layout.imageHeight + layout.tileHeight - 1) / layout.tileHeight

	return layout
}

// GetTileCount returns the total number of tiles
func (tl *TileLayout) GetTileCount() int {
	return tl.numTilesX * tl.numTilesY
}

// GetTileBounds returns the bounds of a specific tile in image coordinates
// Returns (x0, y0, x1, y1) where (x0,y0) is top-left and (x1,y1) is bottom-right (exclusive)
func (tl *TileLayout) GetTileBounds(tileIdx int) (x0, y0, x1, y1 int) {
	if tileIdx < 0 || tileIdx >= tl.GetTileCount() {
		return 0, 0, 0, 0
	}

	// Calculate tile grid position
	tileX := tileIdx % tl.numTilesX
	tileY := tileIdx / tl.numTilesX

	// Calculate tile bounds
	x0 = tileX*tl.tileWidth + tl.tileOffsetX
	y0 = tileY*tl.tileHeight + tl.tileOffsetY
	x1 = x0 + tl.tileWidth
	y1 = y0 + tl.tileHeight

	// Clip to image bounds
	if x1 > tl.imageWidth {
		x1 = tl.imageWidth
	}
	if y1 > tl.imageHeight {
		y1 = tl.imageHeight
	}

	return
}

// GetTileSize returns the actual size of a tile (may be smaller at edges)
func (tl *TileLayout) GetTileSize(tileIdx int) (width, height int) {
	x0, y0, x1, y1 := tl.GetTileBounds(tileIdx)
	return x1 - x0, y1 - y0
}

// TileAssembler assembles multiple tiles into a complete image
type TileAssembler struct {
	layout     *TileLayout
	components int
	imageData  [][]int32 // [component][pixel]
}

// NewTileAssembler creates a new tile assembler
func NewTileAssembler(siz *codestream.SIZSegment) *TileAssembler {
	layout := NewTileLayout(siz)

	ta := &TileAssembler{
		layout:     layout,
		components: int(siz.Csiz),
	}

	// Initialize image data arrays
	numPixels := layout.imageWidth * layout.imageHeight
	ta.imageData = make([][]int32, ta.components)
	for i := range ta.imageData {
		ta.imageData[i] = make([]int32, numPixels)
	}

	return ta
}

// AssembleTile copies tile data into the image at the correct position
// tileIdx: index of the tile
// tileData: decoded tile data [component][pixel]
func (ta *TileAssembler) AssembleTile(tileIdx int, tileData [][]int32) error {
	if tileIdx < 0 || tileIdx >= ta.layout.GetTileCount() {
		return fmt.Errorf("invalid tile index: %d", tileIdx)
	}

	if len(tileData) != ta.components {
		return fmt.Errorf("tile has %d components, expected %d", len(tileData), ta.components)
	}

	// Get tile bounds
	x0, y0, x1, y1 := ta.layout.GetTileBounds(tileIdx)
	tileWidth := x1 - x0
	tileHeight := y1 - y0

	// Verify tile data size
	expectedSize := tileWidth * tileHeight
	for c := 0; c < ta.components; c++ {
		if len(tileData[c]) != expectedSize {
			return fmt.Errorf("component %d: tile data size %d, expected %d",
				c, len(tileData[c]), expectedSize)
		}
	}

	// Copy tile data to image
	for c := 0; c < ta.components; c++ {
		for ty := 0; ty < tileHeight; ty++ {
			// Source: tile row
			tileSrcIdx := ty * tileWidth

			// Destination: image row
			imgY := y0 + ty
			imgDstIdx := imgY*ta.layout.imageWidth + x0

			// Copy row
			copy(ta.imageData[c][imgDstIdx:imgDstIdx+tileWidth],
				tileData[c][tileSrcIdx:tileSrcIdx+tileWidth])
		}
	}

	return nil
}

// GetImageData returns the assembled image data
func (ta *TileAssembler) GetImageData() [][]int32 {
	return ta.imageData
}

// GetImageDimensions returns image width and height
func (ta *TileAssembler) GetImageDimensions() (width, height int) {
	return ta.layout.imageWidth, ta.layout.imageHeight
}

// GetTileLayout returns the tile layout
func (ta *TileAssembler) GetTileLayout() *TileLayout {
	return ta.layout
}

// ValidateTileIndex checks if a tile index is valid
func (ta *TileAssembler) ValidateTileIndex(tileIdx int) error {
	if tileIdx < 0 {
		return fmt.Errorf("tile index cannot be negative: %d", tileIdx)
	}
	if tileIdx >= ta.layout.GetTileCount() {
		return fmt.Errorf("tile index %d out of range (0-%d)",
			tileIdx, ta.layout.GetTileCount()-1)
	}
	return nil
}
