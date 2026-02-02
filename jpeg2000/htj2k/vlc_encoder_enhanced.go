package htj2k

// Enhanced VLC encoder functionality for HTJ2K block encoding
// This file provides additional methods for VLC encoder integration
// Reference: OpenJPH ojph_block_encoder.cpp

import (
	"fmt"
	"math/bits"
)

// VLCReverseWriter implements reverse (backward) bit writing for VLC segment
// In HTJ2K, the VLC segment is written from back to front and then reversed
// This matches OpenJPH's rev_struct behavior for encoding
type VLCReverseWriter struct {
	buf       []byte // Buffer written forwards, to be reversed
	pos       int    // Current position
	tmp       uint8  // Bit accumulator
	bits      int    // Number of bits in accumulator
	lastByte  uint8  // Last byte written (for unstuffing check)
	isVLC     bool   // True for VLC segment, affects unstuffing rules
}

// NewVLCReverseWriter creates a new reverse VLC writer
func NewVLCReverseWriter() *VLCReverseWriter {
	return &VLCReverseWriter{
		buf:   make([]byte, 0, 4096),
		bits:  4, // Start with 4 bits (0xF padding)
		tmp:   0xF, // Initialize with 0xF
		isVLC: true,
	}
}

// WriteBits writes bits to the VLC stream in reverse order
// Bits are accumulated LSB-first and flushed in reverse order
func (w *VLCReverseWriter) WriteBits(value uint32, numBits int) error {
	if numBits < 0 || numBits > 32 {
		return fmt.Errorf("invalid bit count: %d", numBits)
	}

	for numBits > 0 {
		// Take LSB from value
		bit := uint8(value & 1)
		value >>= 1
		numBits--

		// Add to accumulator
		w.tmp |= (bit << w.bits)
		w.bits++

		// Check for VLC unstuffing condition
		// If last byte > 0x8F and accumulator = 0x7F, insert stuffing bit
		if w.isVLC && w.lastByte > 0x8F && w.tmp == 0x7F {
			w.bits++ // Insert stuffing bit (automatically 0)
		}

		// Flush byte when full
		if w.bits >= 8 {
			w.buf = append(w.buf, w.tmp)
			w.pos++
			w.lastByte = w.tmp
			w.tmp = 0
			w.bits = 0
		}
	}

	return nil
}

// Flush finalizes the VLC stream and returns reversed bytes
// Returns the VLC segment ready for inclusion in codeblock
func (w *VLCReverseWriter) Flush() []byte {
	// Flush any remaining bits (pad with 1s)
	if w.bits > 0 {
		// Pad to byte boundary with 1s
		for w.bits < 8 {
			w.tmp |= (1 << w.bits)
			w.bits++
		}
		w.buf = append(w.buf, w.tmp)
	}

	// Add trailing 0xFF byte for decoder safety
	w.buf = append(w.buf, 0xFF)

	// Skip first initialization byte and return rest AS-IS (don't reverse)
	// OpenJPH writes VLC forward and decoder reads backward
	if len(w.buf) <= 1 {
		return []byte{}
	}

	result := make([]byte, len(w.buf)-1)
	copy(result, w.buf[1:])
	return result
}

// Reset resets the writer for a new block
func (w *VLCReverseWriter) Reset() {
	w.buf = w.buf[:0]
	w.pos = 0
	w.tmp = 0xF
	w.bits = 4
	w.lastByte = 0
}

// GetLength returns current written length in bytes
func (w *VLCReverseWriter) GetLength() int {
	length := len(w.buf)
	if w.bits > 0 {
		length++ // Count partial byte
	}
	return length
}

// EncodeQuadVLC encodes a single quad using VLC tables
// This is the main entry point for quad encoding
//
// Parameters:
//   qx, qy: Quad coordinates
//   rho: Significance pattern (4 bits)
//   ek, e1: EMB patterns
//   uOff: U-offset flag (1 if u != 0)
//   context: VLC context (0-7)
//   isFirstRow: True for initial row (y=0)
//   encoder: VLCEncoder to use
//
// Returns: Number of bits written, or error
func EncodeQuadVLC(qx, qy int, rho, ek, e1, uOff, context uint8,
	isFirstRow bool, encoder *VLCEncoder) (int, error) {

	// Use encoder's context-aware encoding
	return encoder.EncodeCxtVLCWithLen(context, rho, uOff, ek, e1, isFirstRow)
}

// ComputeQuadEMB computes EMB patterns (ek, e1) for a quad
// Reference: ITU-T T.814 Annex C
//
// EMB (Exponent and Mantissa Bits) patterns indicate which samples
// in the quad have exponent bits (ek) and mantissa MSB bits (e1)
//
// Parameters:
//   samples: 4 sample values in quad [TL, TR, BL, BR]
//   rho: Significance pattern (determines which samples to check)
//
// Returns: ek (4 bits), e1 (4 bits)
func ComputeQuadEMB(samples [4]int32, rho uint8) (ek, e1 uint8) {
	for i := 0; i < 4; i++ {
		if (rho>>i)&1 == 0 {
			continue // Sample not significant
		}

		mag := uint32(samples[i])
		if samples[i] < 0 {
			mag = uint32(-samples[i])
		}

		if mag == 0 {
			continue
		}

		// Find magnitude exponent (position of MSB, 0-indexed)
		// For mag=7 (0b111), MSB is at position 2
		// bits.Len32(7) = 3, so exp = 3-1 = 2
		exp := bits.Len32(mag) - 1

		// ek bit indicates if exponent > 0
		if exp > 0 {
			ek |= (1 << i)
		}

		// e1 bit indicates if bit at position (exp-1) is set
		// For mag=7 (0b111), exp=2, check bit 1: (7>>1)&1 = 1
		if exp > 0 && ((mag>>(exp-1))&1) != 0 {
			e1 |= (1 << i)
		}
	}

	return ek, e1
}

// ComputeQuadRho computes significance pattern for a quad
// Returns 4-bit pattern where bit i indicates if sample i is non-zero
//
// Sample order: [0]=TL, [1]=TR, [2]=BL, [3]=BR
func ComputeQuadRho(samples [4]int32) uint8 {
	rho := uint8(0)
	for i := 0; i < 4; i++ {
		if samples[i] != 0 {
			rho |= (1 << i)
		}
	}
	return rho
}

// ExtractQuadSamples extracts 4 samples for a quad from coefficient array
// Returns samples in order: [TL, TR, BL, BR]
func ExtractQuadSamples(data []int32, width, qx, qy int) [4]int32 {
	samples := [4]int32{0, 0, 0, 0}

	// Top-left (TL)
	x0 := qx * 2
	y0 := qy * 2
	if x0 < width && y0*width+x0 < len(data) {
		samples[0] = data[y0*width+x0]
	}

	// Top-right (TR)
	x1 := x0 + 1
	if x1 < width && y0*width+x1 < len(data) {
		samples[1] = data[y0*width+x1]
	}

	// Bottom-left (BL)
	y1 := y0 + 1
	if x0 < width && y1*width+x0 < len(data) {
		samples[2] = data[y1*width+x0]
	}

	// Bottom-right (BR)
	if x1 < width && y1*width+x1 < len(data) {
		samples[3] = data[y1*width+x1]
	}

	return samples
}

// EncodeQuadPair encodes a pair of horizontally adjacent quads
// This is the basic unit of HTJ2K encoding
//
// Parameters:
//   qx: X-coordinate of first quad (must be even)
//   qy: Y-coordinate
//   data: Coefficient array
//   width: Block width
//   context: Context computer
//   vlcEnc: VLC encoder
//   melEnc: MEL encoder
//   msEnc: MagSgn encoder
//   expPred: Exponent predictor (optional, can be nil)
//   uvlcEnc: UVLC encoder (optional, can be nil)
//
// Returns: error if any
func EncodeQuadPair(qx, qy int, data []int32, width int,
	context *ContextComputer, vlcEnc *VLCEncoder,
	melEnc *MELEncoder, msEnc *MagSgnEncoder,
	expPred *ExponentPredictorComputer, uvlcEnc *UVLCEncoder) error {

	qw := (width + 1) / 2
	isFirstRow := (qy == 0)
	hasSecondQuad := (qx+1 < qw)

	type quadStats struct {
		samples  [4]int32
		rho      uint8
		sigCount int
		maxE     int
		eps0     uint8
		uOff     uint8
		tableEK  uint8
		Uq       int
		uq       int
	}

	// computeBasicStats extracts rho, maxE, sigCount from samples.
	// eps0/uOff are NOT set here — they depend on Kq which is computed later.
	computeBasicStats := func(qxi int) quadStats {
		var s quadStats
		s.samples = ExtractQuadSamples(data, width, qxi, qy)
		s.rho = ComputeQuadRho(s.samples)
		for i := 0; i < 4; i++ {
			if (s.rho>>i)&1 != 0 {
				s.sigCount++
				val := s.samples[i]
				mag := uint32(val)
				if val < 0 {
					mag = uint32(-val)
				}
				eQ := 0
				if mag > 0 {
					eQ = bits.Len32(mag)
				}
				if eQ > s.maxE {
					s.maxE = eQ
				}
			}
		}
		return s
	}

	// computeEps0AndUOff sets eps0 and uOff based on Kq prediction.
	// Per OpenJPH: eps0 is only computed when uq > 0 (Uq > Kq).
	// When uq == 0, eps0=0 and uOff=0 (all exponents ≤ Kq, no offset needed).
	computeEps0AndUOff := func(s *quadStats, Kq int) {
		if s.rho == 0 {
			return
		}
		s.Uq = s.maxE
		if Kq > s.Uq {
			s.Uq = Kq
		}
		s.uq = s.Uq - Kq

		if s.uq > 0 {
			// eps0: which significant samples have exp == maxE (== Uq when uq > 0)
			for i := 0; i < 4; i++ {
				if (s.rho>>i)&1 != 0 {
					mag := uint32(s.samples[i])
					if s.samples[i] < 0 {
						mag = uint32(-s.samples[i])
					}
					eQ := 0
					if mag > 0 {
						eQ = bits.Len32(mag)
					}
					if eQ == s.maxE {
						s.eps0 |= (1 << i)
					}
				}
			}
			if s.eps0 > 0 {
				s.uOff = 1
			}
		}
		// When uq == 0: eps0=0, uOff=0 (default zero values)
	}

	s1 := computeBasicStats(qx)

	// --- Step 1: Compute Kq1 BEFORE VLC encoding (predictor uses neighbors, not current quad) ---
	Kq1 := 0
	if expPred != nil {
		Kq1 = expPred.ComputePredictor(qx, qy)
	}
	computeEps0AndUOff(&s1, Kq1)

	// --- Step 2: MEL + VLC for first quad ---
	ctx1 := context.ComputeContext(qx, qy, isFirstRow)

	if ctx1 == 0 {
		if s1.rho == 0 {
			melEnc.EncodeBit(0)
		} else {
			melEnc.EncodeBit(1)
		}
	}

	if s1.rho != 0 {
		_, tableEK1, err := vlcEnc.EncodeQuadVLCByEMB(ctx1, s1.rho, s1.uOff, s1.eps0, isFirstRow)
		if err != nil {
			return fmt.Errorf("VLC encode quad (%d,%d): %w", qx, qy, err)
		}
		s1.tableEK = tableEK1
		context.UpdateQuadSignificance(qx, qy, s1.rho)
	}

	// Store exponents AFTER processing Q1, so Q2's predictor can use Q1's values
	if expPred != nil {
		expPred.SetQuadExponents(qx, qy, s1.maxE, s1.sigCount)
	}

	// --- Step 3: MEL + VLC for second quad (if exists) ---
	var s2 quadStats
	if hasSecondQuad {
		s2 = computeBasicStats(qx + 1)

		Kq2 := 0
		if expPred != nil {
			Kq2 = expPred.ComputePredictor(qx+1, qy)
		}
		computeEps0AndUOff(&s2, Kq2)

		ctx2 := context.ComputeContext(qx+1, qy, isFirstRow)

		if ctx2 == 0 {
			if s2.rho == 0 {
				melEnc.EncodeBit(0)
			} else {
				melEnc.EncodeBit(1)
			}
		}

		// Encode VLC for quad 2
		// For context=0, only encode VLC if rho!=0 (MEL bit=1)
		// For context!=0, always encode VLC (even if rho=0)
		shouldEncodeVLC := (ctx2 != 0) || (s2.rho != 0)

		if shouldEncodeVLC {
			_, tableEK2, err := vlcEnc.EncodeQuadVLCByEMB(ctx2, s2.rho, s2.uOff, s2.eps0, isFirstRow)
			if err != nil {
				return fmt.Errorf("VLC encode quad (%d,%d): %w", qx+1, qy, err)
			}
			s2.tableEK = tableEK2
			context.UpdateQuadSignificance(qx+1, qy, s2.rho)
		}

		if expPred != nil {
			expPred.SetQuadExponents(qx+1, qy, s2.maxE, s2.sigCount)
		}
	}

	// --- Step 4: MEL event + UVLC encoding (after both VLCs) ---
	// For initial pairs with both uOff=1, choose melEvent (0=mode3, 1=mode4).
	// Mode 3 has limited range for u1 when u0 is large. Mode 4 adds +2 bias.
	// Try mode=3 first; if no table entry matches, use mode=4.
	bothUOff := hasSecondQuad && s1.uOff == 1 && s2.uOff == 1

	if uvlcEnc != nil {
		uOff0 := s1.uOff
		uOff1 := uint8(0)
		if hasSecondQuad {
			uOff1 = s2.uOff
		}
		u0 := s1.uq
		u1 := 0
		if hasSecondQuad {
			u1 = s2.uq
		}

		melEvent := 0
		if isFirstRow && bothUOff {
			if !uvlcEnc.HasTableEntry(uOff0, uOff1, u0, u1, true, 0) {
				melEvent = 1
			}
			melEnc.EncodeBit(melEvent)
		}

		if err := uvlcEnc.EncodePair(uOff0, uOff1, u0, u1, isFirstRow, melEvent); err != nil {
			return fmt.Errorf("UVLC encode pair (%d,%d): %w", qx, qy, err)
		}
	} else if isFirstRow && bothUOff {
		melEnc.EncodeBit(0)
	}

	// --- Step 5: MagSgn encoding ---
	if s1.rho != 0 {
		for i := 0; i < 4; i++ {
			if (s1.rho>>i)&1 != 0 {
				val := s1.samples[i]
				mag := uint32(val)
				sign := 0
				if val < 0 {
					mag = uint32(-val)
					sign = 1
				}
				ekBit := int((s1.tableEK >> i) & 1)
				mn := s1.Uq - ekBit
				if mn < 0 {
					mn = 0
				}
				magLower := mag & ((1 << mn) - 1)
				msEnc.EncodeMagSgn(magLower, sign, mn)
			}
		}
	}

	if hasSecondQuad && s2.rho != 0 {
		for i := 0; i < 4; i++ {
			if (s2.rho>>i)&1 != 0 {
				val := s2.samples[i]
				mag := uint32(val)
				sign := 0
				if val < 0 {
					mag = uint32(-val)
					sign = 1
				}
				ekBit := int((s2.tableEK >> i) & 1)
				mn := s2.Uq - ekBit
				if mn < 0 {
					mn = 0
				}
				magLower := mag & ((1 << mn) - 1)
				msEnc.EncodeMagSgn(magLower, sign, mn)
			}
		}
	}

	return nil
}

// countLeadingZeros counts leading zero bits in a uint32
func countLeadingZeros(x uint32) int {
	if x == 0 {
		return 32
	}
	n := 0
	if x <= 0x0000FFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFF {
		n += 1
	}
	return n
}
