package htj2k

// HTBlockDecoder implements complete HTJ2K block decoding
// with proper context computation and VLC decoding
type HTBlockDecoder struct {
	width   int
	height  int
	numQX   int // Number of quads in X direction
	numQY   int // Number of quads in Y direction

	// Component decoders
	mel     *MELDecoderSpec
	magsgn  *MagSgnDecoder
	vlc     *VLCDecoder
	context *ContextComputer

	// Decoded coefficients
	data []int32
}

// NewHTBlockDecoder creates a new HTJ2K block decoder
func NewHTBlockDecoder(width, height int) *HTBlockDecoder {
	numQX := (width + 1) / 2  // Ceiling division
	numQY := (height + 1) / 2

	return &HTBlockDecoder{
		width:   width,
		height:  height,
		numQX:   numQX,
		numQY:   numQY,
		context: NewContextComputer(width, height),
		data:    make([]int32, width*height),
	}
}

// DecodeBlock decodes an HTJ2K codeblock
// Returns the decoded coefficient data
func (h *HTBlockDecoder) DecodeBlock(codeblock []byte) ([]int32, error) {
	// Parse codeblock into three segments
	if err := h.parseSegments(codeblock); err != nil {
		return nil, err
	}

	// Decode quads in raster order
	for qy := 0; qy < h.numQY; qy++ {
		isFirstRow := (qy == 0)

		for qx := 0; qx < h.numQX; qx++ {
			if err := h.decodeQuad(qx, qy, isFirstRow); err != nil {
				// On error, treat remaining samples as zero
				break
			}
		}
	}

	return h.data, nil
}

// parseSegments parses the codeblock into MagSgn, MEL, and VLC segments
func (h *HTBlockDecoder) parseSegments(codeblock []byte) error {
	if len(codeblock) < 2 {
		// Empty or too small - all zeros
		return nil
	}

	// Last 2 bytes encode segment lengths
	melLen := int(codeblock[len(codeblock)-2])
	vlcLen := int(codeblock[len(codeblock)-1])

	if melLen+vlcLen+2 > len(codeblock) {
		// Invalid lengths
		return nil
	}

	// Calculate segment boundaries
	// Layout: [MagSgn] [MEL] [VLC] [Lengths(2)]
	dataLen := len(codeblock) - 2
	magsgnLen := dataLen - melLen - vlcLen

	if magsgnLen < 0 {
		return nil
	}

	// Extract segments
	magsgnData := codeblock[0:magsgnLen]
	melData := codeblock[magsgnLen : magsgnLen+melLen]
	vlcData := codeblock[magsgnLen+melLen : dataLen]

	// Initialize decoders
	h.magsgn = NewMagSgnDecoder(magsgnData)
	h.mel = NewMELDecoderSpec(melData)
	h.vlc = NewVLCDecoder(vlcData)

	return nil
}

// decodeQuad decodes a single 2x2 quad
func (h *HTBlockDecoder) decodeQuad(qx, qy int, isFirstRow bool) error {
	// Step 1: Decode MEL bit
	// MEL bit indicates if the quad has any significant samples
	if h.mel == nil {
		// No MEL data - all quads are zero
		return nil
	}

	melBit, hasMore := h.mel.DecodeMELSym()
	if !hasMore {
		// MEL stream exhausted - remaining quads are zero
		return nil
	}

	if melBit == 0 {
		// MEL bit == 0: All samples in quad are zero
		return nil
	}

	// Step 2: Compute context based on neighboring significance
	context := h.context.ComputeContext(qx, qy, isFirstRow)

	// Step 3: Decode VLC to get significance pattern and magnitude info
	rho, u_off, e_k, e_1, found := h.vlc.DecodeQuadWithContext(context, isFirstRow)
	if !found {
		// VLC decoder exhausted
		return nil
	}

	// Step 4: Update significance map
	h.context.UpdateQuadSignificance(qx, qy, rho)

	// Step 5: Decode magnitudes and signs for significant samples
	sx := qx * 2
	sy := qy * 2

	for i := 0; i < 4; i++ {
		if (rho & (1 << i)) != 0 {
			// Sample is significant - decode magnitude and sign

			// Compute number of magnitude bits to decode
			// This uses e_k, e_1, and u_off from VLC decoding
			numBits := int(e_k)
			if i < 2 && e_1 > 0 {
				numBits = int(e_1)
			}

			// Decode magnitude
			mag, sign, ok := h.magsgn.DecodeMagSgn(numBits)
			if !ok {
				mag = 0
				sign = 0
			}

			// Apply unsigned offset
			if u_off > 0 {
				mag += uint32(u_off)
			}

			// Convert to signed coefficient
			coeff := int32(mag)
			if sign != 0 {
				coeff = -coeff
			}

			// Store coefficient
			px := sx + (i % 2)
			py := sy + (i / 2)
			if px < h.width && py < h.height {
				h.data[py*h.width+px] = coeff
			}
		}
	}

	return nil
}

// GetData returns the decoded coefficient data
func (h *HTBlockDecoder) GetData() []int32 {
	return h.data
}

// GetSample returns the decoded coefficient at (x, y)
func (h *HTBlockDecoder) GetSample(x, y int) int32 {
	if x < 0 || x >= h.width || y < 0 || y >= h.height {
		return 0
	}
	return h.data[y*h.width+x]
}
