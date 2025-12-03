package htj2k

// QuadPairDecoder implements quad-pair interleaved VLC decoding
// as specified in ISO/IEC 15444-15:2019 Clause 7.3.4
//
// Quad-pair interleaving defines the order in which CxtVLC codewords
// and U-VLC components are decoded from the VLC bit-stream.
//
// A quad-pair consists of two horizontally adjacent quads:
//   - First quad:  q1 = 2g
//   - Second quad: q2 = 2g+1
//
// Special cases:
// 1. Initial line-pair (q < QW): First row of quads in the block
// 2. When both quads in initial line-pair have ulf=1: Use Formula (4)
// 3. When first quad has uq1>2: Second quad uses simplified decoding

// QuadPairResult contains the decoded information for a quad-pair
type QuadPairResult struct {
	// First quad (q1 = 2g)
	Rho1  uint8  // Significance pattern (4 bits)
	ULF1  uint8  // Unsigned residual offset flag (0 or 1)
	Uq1   uint32 // Unsigned residual value (if ulf1=1)
	E1_1  uint8  // EMB pattern e1
	EMax1 uint8  // EMB pattern emax

	// Second quad (q2 = 2g+1)
	Rho2  uint8  // Significance pattern (4 bits)
	ULF2  uint8  // Unsigned residual offset flag (0 or 1)
	Uq2   uint32 // Unsigned residual value (if ulf2=1)
	E1_2  uint8  // EMB pattern e1
	EMax2 uint8  // EMB pattern emax

	// Metadata
	IsInitialLinePair bool // True if q1 < QW (first row)
	HasSecondQuad     bool // False if QW is odd and this is the last pair
}

// QuadPairDecoder decodes quad-pairs from the VLC bit-stream
type QuadPairDecoder struct {
	vlcDecoder  *VLCDecoder
	uvlcDecoder *UVLCDecoder
	context     *ContextComputer
	QW          int // Width in quads
}

// NewQuadPairDecoder creates a new quad-pair decoder
func NewQuadPairDecoder(vlcData []byte, widthInQuads, heightInQuads int) *QuadPairDecoder {
	// Create a bit reader from VLC data for U-VLC decoding
	bitReader := &VLCBitReader{
		decoder: NewVLCDecoder(vlcData),
	}

	return &QuadPairDecoder{
		vlcDecoder:  NewVLCDecoder(vlcData),
		uvlcDecoder: NewUVLCDecoder(bitReader),
		context:     NewContextComputer(widthInQuads*2, heightInQuads*2), // Convert to samples
		QW:          widthInQuads,
	}
}

// VLCBitReader adapts VLCDecoder to the BitReader interface for U-VLC
type VLCBitReader struct {
	decoder *VLCDecoder
}

func (v *VLCBitReader) ReadBit() (uint8, error) {
	bit, ok := v.decoder.readBits(1)
	if !ok {
		return 0, ErrInsufficientData
	}
	return uint8(bit), nil
}

func (v *VLCBitReader) ReadBitsLE(n int) (uint32, error) {
	// Read n bits in little-endian order (LSB first)
	var result uint32
	for i := 0; i < n; i++ {
		bit, err := v.ReadBit()
		if err != nil {
			return 0, err
		}
		result |= uint32(bit) << i
	}
	return result, nil
}

// DecodeQuadPair decodes a single quad-pair according to Clause 7.3.4
//
// Parameters:
//   - g: Quad-pair index (q1=2g, q2=2g+1)
//   - qy: Quad row (for determining initial line-pair)
//
// Returns:
//   - QuadPairResult containing decoded information for both quads
//   - error if decoding fails
func (d *QuadPairDecoder) DecodeQuadPair(g int, qy int) (*QuadPairResult, error) {
	result := &QuadPairResult{
		IsInitialLinePair: qy == 0, // Initial line-pair is first row
		HasSecondQuad:     (2*g + 1) < d.QW,
	}

	q1 := 2 * g       // First quad index
	q2 := 2*g + 1     // Second quad index
	qx1 := q1 % d.QW  // First quad x position
	qx2 := q2 % d.QW  // Second quad x position

	// Step 1: Decode first quad's CxtVLC codeword
	ctx1 := d.context.ComputeContext(qx1, qy, result.IsInitialLinePair)
	rho1, u_off1, e_k1, e_1_1, found := d.vlcDecoder.DecodeQuadWithContext(ctx1, result.IsInitialLinePair)
	if !found {
		return nil, ErrInsufficientData
	}

	result.Rho1 = rho1
	result.ULF1 = u_off1
	result.E1_1 = e_1_1
	result.EMax1 = e_k1

	// Update significance map for first quad
	d.context.UpdateQuadSignificance(qx1, qy, rho1)

	// Step 2: Decode first quad's U-VLC if ulf1=1
	if result.ULF1 == 1 {
		uq1, err := d.uvlcDecoder.DecodeUnsignedResidual()
		if err != nil {
			return nil, err
		}
		result.Uq1 = uq1
	} else {
		result.Uq1 = 0
	}

	// Step 3: If no second quad (odd width), return early
	if !result.HasSecondQuad {
		return result, nil
	}

	// Step 4: Decode second quad's CxtVLC codeword
	ctx2 := d.context.ComputeContext(qx2, qy, result.IsInitialLinePair)
	rho2, u_off2, e_k2, e_1_2, found := d.vlcDecoder.DecodeQuadWithContext(ctx2, result.IsInitialLinePair)
	if !found {
		return nil, ErrInsufficientData
	}

	result.Rho2 = rho2
	result.ULF2 = u_off2
	result.E1_2 = e_1_2
	result.EMax2 = e_k2

	// Update significance map for second quad
	d.context.UpdateQuadSignificance(qx2, qy, rho2)

	// Step 5: Decode second quad's U-VLC (conditional logic)
	if result.ULF2 == 1 {
		// Check special cases for initial line-pair
		if result.IsInitialLinePair && result.ULF1 == 1 && result.ULF2 == 1 {
			// Both quads in initial line-pair have ulf=1
			// Use Formula (4): u = 2 + u_pfx + u_sfx + 4*u_ext
			uq2, err := d.uvlcDecoder.DecodeUnsignedResidualInitialPair()
			if err != nil {
				return nil, err
			}
			result.Uq2 = uq2
		} else if result.ULF1 == 1 && result.Uq1 > 2 {
			// First quad has uq1>2, use simplified decoding
			// NOTE: uq1>2 means u_pfx>2, which means u_pfx=3 or u_pfx=5
			// (since u_pfx can only be {1,2,3,5})
			// This happens when the prefix has length ≥3
			uq2, err := d.uvlcDecoder.DecodeUnsignedResidualSecondQuad()
			if err != nil {
				return nil, err
			}
			result.Uq2 = uq2
		} else {
			// Normal U-VLC decoding using Formula (3)
			uq2, err := d.uvlcDecoder.DecodeUnsignedResidual()
			if err != nil {
				return nil, err
			}
			result.Uq2 = uq2
		}
	} else {
		result.Uq2 = 0
	}

	return result, nil
}

// DecodeAllQuadPairs decodes all quad-pairs in a block
//
// Parameters:
//   - heightInQuads: Number of quad rows
//
// Returns:
//   - Slice of QuadPairResult for all quad-pairs in scan order
//   - error if decoding fails
func (d *QuadPairDecoder) DecodeAllQuadPairs(heightInQuads int) ([]*QuadPairResult, error) {
	// Calculate total number of quad-pairs
	// Each row has ceil(QW/2) quad-pairs
	pairsPerRow := (d.QW + 1) / 2
	totalPairs := pairsPerRow * heightInQuads

	results := make([]*QuadPairResult, 0, totalPairs)

	// Scan by rows (quad-pairs are processed row by row)
	for qy := 0; qy < heightInQuads; qy++ {
		for g := 0; g < pairsPerRow; g++ {
			pair, err := d.DecodeQuadPair(g, qy)
			if err != nil {
				return nil, err
			}
			results = append(results, pair)
		}
	}

	return results, nil
}

// GetQuadInfo extracts individual quad information from quad-pair results
//
// This helper function converts quad-pair results into per-quad information
// for easier integration with existing block decoder logic
func GetQuadInfo(pair *QuadPairResult, quadIndex int) (rho uint8, ulf uint8, uq uint32, e1 uint8, emax uint8) {
	if quadIndex == 0 {
		// First quad (q1)
		return pair.Rho1, pair.ULF1, pair.Uq1, pair.E1_1, pair.EMax1
	} else {
		// Second quad (q2)
		return pair.Rho2, pair.ULF2, pair.Uq2, pair.E1_2, pair.EMax2
	}
}
