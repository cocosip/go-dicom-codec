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

	// Process first quad in pair
	samples1 := ExtractQuadSamples(data, width, qx, qy)
	rho1 := ComputeQuadRho(samples1)

	// Compute quad statistics for exponent prediction
	// Use bits.Len32(mag) to match decoder's MagnitudeExponent function
	sigCount1 := 0
	maxE1 := 0
	for i := 0; i < 4; i++ {
		if (rho1>>i)&1 != 0 {
			sigCount1++
			val := samples1[i]
			mag := uint32(val)
			if val < 0 {
				mag = uint32(-val)
			}
			eQ := 0
			if mag > 0 {
				eQ = bits.Len32(mag)
			}
			if eQ > maxE1 {
				maxE1 = eQ
			}
		}
	}

	// MEL: encode quad significance
	if rho1 == 0 {
		melEnc.EncodeBit(0) // All-zero quad
	} else {
		melEnc.EncodeBit(1) // Has significant samples

		// Compute context for VLC
		prevVLC := uint16(0)
		if qx > 0 {
			prevVLC = context.GetQuadVLC(qx-1, qy)
		}

		var ctx uint8
		if isFirstRow {
			ctx = context.ComputeInitialRowContext(qx, prevVLC)
		} else {
			ctx = context.ComputeSubsequentRowContext(qx, qy, prevVLC)
		}

		// Compute eps0: mask of which samples have exponent == maxE
		// Per OpenJPH: eps0 bit i = (e_q[i] == e_qmax)
		eps0 := uint8(0)
		if maxE1 > 0 {
			for i := 0; i < 4; i++ {
				if (rho1>>i)&1 != 0 {
					mag := uint32(samples1[i])
					if samples1[i] < 0 {
						mag = uint32(-samples1[i])
					}
					eQ := 0
					if mag > 0 {
						eQ = bits.Len32(mag)
					}
					if eQ == maxE1 {
						eps0 |= (1 << i)
					}
				}
			}
		}

		// Determine u_offset: 1 if eps0 > 0
		uOff1 := uint8(0)
		if eps0 > 0 {
			uOff1 = 1
		}

		// Encode VLC using OpenJPH-compatible EMB lookup
		// Returns codeword length and table's EK for MagSgn encoding
		vlcLen1, tableEK1, err := vlcEnc.EncodeQuadVLCByEMB(ctx, rho1, uOff1, eps0, isFirstRow)
		if err != nil {
			return fmt.Errorf("VLC encode quad (%d,%d): %w", qx, qy, err)
		}

		// Store VLC result for context computation
		vlcResult1 := (uint16(tableEK1) << 12) | (uint16(eps0) << 8) |
			(uint16(rho1) << 4) | (uint16(uOff1) << 3) | uint16(vlcLen1)
		context.SetQuadVLC(qx, qy, vlcResult1)

		// Phase 3: Exponent Prediction and UVLC encoding
		Kq1 := 0
		if expPred != nil {
			expPred.SetQuadExponents(qx, qy, maxE1, sigCount1)
			Kq1 = expPred.ComputePredictor(qx, qy)
		}

		// Uq = max(maxE, Kq)
		Uq1 := maxE1
		if Kq1 > Uq1 {
			Uq1 = Kq1
		}
		uq1 := Uq1 - Kq1

		// Encode u_q using UVLC if > 2
		if uq1 > 2 && uvlcEnc != nil {
			if err := uvlcEnc.EncodeUVLC(uq1, isFirstRow); err != nil {
				return fmt.Errorf("UVLC encode quad (%d,%d): %w", qx, qy, err)
			}
		}

		// Encode magnitude and sign for significant samples
		// mn = Uq - ekBit (using TABLE's e_k for MagSgn bit count)
		for i := 0; i < 4; i++ {
			if (rho1>>i)&1 != 0 {
				val := samples1[i]
				mag := uint32(val)
				sign := 0
				if val < 0 {
					mag = uint32(-val)
					sign = 1
				}

				ekBit := int((tableEK1 >> i) & 1)
				mn := Uq1 - ekBit
				if mn < 0 {
					mn = 0
				}

				magLower := mag & ((1 << mn) - 1)
				msEnc.EncodeMagSgn(magLower, sign, mn)
			}
		}
	}

	// Process second quad in pair (if exists)
	if qx+1 < qw {
		samples2 := ExtractQuadSamples(data, width, qx+1, qy)
		rho2 := ComputeQuadRho(samples2)

		// Compute quad statistics for exponent prediction
		// Use bits.Len32(mag) to match decoder's MagnitudeExponent function
		sigCount2 := 0
		maxE2 := 0
		for i := 0; i < 4; i++ {
			if (rho2>>i)&1 != 0 {
				sigCount2++
				val := samples2[i]
				mag := uint32(val)
				if val < 0 {
					mag = uint32(-val)
				}
				eQ := 0
				if mag > 0 {
					eQ = bits.Len32(mag)
				}
				if eQ > maxE2 {
					maxE2 = eQ
				}
			}
		}

		if rho2 == 0 {
			melEnc.EncodeBit(0)
		} else {
			melEnc.EncodeBit(1)

			// Compute context using first quad's result
			prevVLC := context.GetQuadVLC(qx, qy)
			var ctx uint8
			if isFirstRow {
				ctx = context.ComputeInitialRowContext(qx+1, prevVLC)
			} else {
				ctx = context.ComputeSubsequentRowContext(qx+1, qy, prevVLC)
			}

			// Compute eps0: mask of which samples have exponent == maxE
			// Per OpenJPH: eps0 bit i = (e_q[i] == e_qmax)
			eps0 := uint8(0)
			if maxE2 > 0 {
				for i := 0; i < 4; i++ {
					if (rho2>>i)&1 != 0 {
						mag := uint32(samples2[i])
						if samples2[i] < 0 {
							mag = uint32(-samples2[i])
						}
						eQ := 0
						if mag > 0 {
							eQ = bits.Len32(mag)
						}
						if eQ == maxE2 {
							eps0 |= (1 << i)
						}
					}
				}
			}

			// Determine u_offset: 1 if eps0 > 0
			uOff2 := uint8(0)
			if eps0 > 0 {
				uOff2 = 1
			}

			// Encode VLC using OpenJPH-compatible EMB lookup
			vlcLen2, tableEK2, err := vlcEnc.EncodeQuadVLCByEMB(ctx, rho2, uOff2, eps0, isFirstRow)
			if err != nil {
				return fmt.Errorf("VLC encode quad (%d,%d): %w", qx+1, qy, err)
			}

			vlcResult2 := (uint16(tableEK2) << 12) | (uint16(eps0) << 8) |
				(uint16(rho2) << 4) | (uint16(uOff2) << 3) | uint16(vlcLen2)
			context.SetQuadVLC(qx+1, qy, vlcResult2)

			// Phase 3: Exponent Prediction and UVLC encoding for second quad
			Kq2 := 0
			if expPred != nil {
				expPred.SetQuadExponents(qx+1, qy, maxE2, sigCount2)
				Kq2 = expPred.ComputePredictor(qx+1, qy)
			}

			// Uq = max(maxE, Kq)
			Uq2 := maxE2
			if Kq2 > Uq2 {
				Uq2 = Kq2
			}
			uq2 := Uq2 - Kq2

			// Encode u_q using UVLC if > 2
			if uq2 > 2 && uvlcEnc != nil {
				if err := uvlcEnc.EncodeUVLC(uq2, isFirstRow); err != nil {
					return fmt.Errorf("UVLC encode quad (%d,%d): %w", qx+1, qy, err)
				}
			}

			// Encode magnitude and sign for significant samples
			// mn = Uq - ekBit (using TABLE's e_k for MagSgn bit count)
			for i := 0; i < 4; i++ {
				if (rho2>>i)&1 != 0 {
					val := samples2[i]
					mag := uint32(val)
					sign := 0
					if val < 0 {
						mag = uint32(-val)
						sign = 1
					}

					ekBit := int((tableEK2 >> i) & 1)
					mn := Uq2 - ekBit
					if mn < 0 {
						mn = 0
					}

					magLower := mag & ((1 << mn) - 1)
					msEnc.EncodeMagSgn(magLower, sign, mn)
				}
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
