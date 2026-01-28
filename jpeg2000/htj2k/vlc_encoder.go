package htj2k

import (
	"fmt"
	"math/bits"
)

// VLCEncoder implements full context-aware VLC encoding for HTJ2K
// Based on ISO/IEC 15444-15:2019 Annex F.3 and F.4
//
// Features:
// - Context-based CxtVLC encoding using OpenJPEG tables
// - Bit-stuffing and proper byte-stream formatting
// - BitStreamWriter interface for U-VLC integration
type VLCEncoder struct {
	// Bit packing state (matches emitVLCBits procedure in spec)
	vlcPos  int    // Current position in VLC buffer
	vlcBits int    // Number of bits in vlcTmp
	vlcTmp  uint8  // Temporary bit accumulator
	vlcLast uint8  // Last byte written (for bit-stuffing)
	vlcBuf  []byte // VLC byte buffer (written forwards, reversed later)

	// Encoding tables using hash maps for efficient lookup
	encodeMap0 map[encodeKey]VLCEncodeEntry // For initial row
	encodeMap1 map[encodeKey]VLCEncodeEntry // For non-initial rows
}

// VLCEncodeEntry represents a CxtVLC encoding table entry
type VLCEncodeEntry struct {
	Codeword uint8 // VLC codeword bits
	Length   uint8 // Codeword length in bits
	Valid    bool  // Whether this entry is valid
}

// encodeKey creates a unique key for encoding table lookup
type encodeKey struct {
	cq   uint8
	rho  uint8
	uOff uint8
	ek   uint8
	e1   uint8
}

// NewVLCEncoder creates a new context-aware VLC encoder
func NewVLCEncoder() *VLCEncoder {
	encoder := &VLCEncoder{
		vlcBuf:     make([]byte, 0, 4096),
		encodeMap0: make(map[encodeKey]VLCEncodeEntry),
		encodeMap1: make(map[encodeKey]VLCEncodeEntry),
	}

	// Initialize VLC packer state
	encoder.initVLCPacker()

	// Build encoding tables from VLC decode tables
	encoder.buildEncodeTables()

	return encoder
}

// initVLCPacker initializes the VLC bit packer state
// Implements the initVLCPacker procedure from Clause F.4
func (v *VLCEncoder) initVLCPacker() {
	v.vlcBits = 4
	v.vlcTmp = 15
	v.vlcBuf = append(v.vlcBuf, 255) // VLC_buf[0] = 255
	v.vlcPos = 1
	v.vlcLast = 255
}

// buildEncodeTables builds CxtVLC encoding tables from decode tables
func (v *VLCEncoder) buildEncodeTables() {
	// Build hash map from VLC_tbl0 (initial row)
	for _, entry := range VLC_tbl0 {
		key := encodeKey{
			cq:   entry.CQ,
			rho:  entry.Rho,
			uOff: entry.UOff,
			ek:   entry.EK,
			e1:   entry.E1,
		}
		v.encodeMap0[key] = VLCEncodeEntry{
			Codeword: entry.Cwd,
			Length:   entry.CwdLen,
			Valid:    true,
		}
	}

	// Build hash map from VLC_tbl1 (non-initial rows)
	for _, entry := range VLC_tbl1 {
		key := encodeKey{
			cq:   entry.CQ,
			rho:  entry.Rho,
			uOff: entry.UOff,
			ek:   entry.EK,
			e1:   entry.E1,
		}
		v.encodeMap1[key] = VLCEncodeEntry{
			Codeword: entry.Cwd,
			Length:   entry.CwdLen,
			Valid:    true,
		}
	}
}

// emitVLCBits writes bits to the VLC stream with bit-stuffing
// Implements the emitVLCBits procedure from Clause F.4
func (v *VLCEncoder) emitVLCBits(cwd uint32, length int) error {
	for length > 0 {
		// Extract LSB
		bit := cwd & 1
		cwd = cwd >> 1
		length--

		// Add bit to accumulator
		v.vlcTmp = v.vlcTmp | uint8(bit<<v.vlcBits)
		v.vlcBits++

		// Check for bit-stuffing condition
		// If last byte > 0x8F and current accumulator = 0x7F, stuff a bit
		if (v.vlcLast > 0x8F) && (v.vlcTmp == 0x7F) {
			v.vlcBits++
		}

		// Flush byte if accumulator is full
		if v.vlcBits == 8 {
			v.vlcBuf = append(v.vlcBuf, v.vlcTmp)
			v.vlcPos++
			v.vlcLast = v.vlcTmp
			v.vlcTmp = 0
			v.vlcBits = 0
		}
	}

	return nil
}

// WriteBits implements BitStreamWriter interface for U-VLC integration
func (v *VLCEncoder) WriteBits(bits uint32, length int) error {
	return v.emitVLCBits(bits, length)
}

// EncodeCxtVLC encodes a quad using context-based VLC
//
// Parameters:
//   - context: Context value (0-15)
//   - rho: Significance pattern (4 bits)
//   - uOff: Unsigned residual offset flag (0 or 1)
//   - ek: E_k value from EMB pattern
//   - e1: E_1 value from EMB pattern
//   - isFirstRow: True for initial line-pair
func (v *VLCEncoder) EncodeCxtVLC(context, rho, uOff, ek, e1 uint8, isFirstRow bool) error {
	// Create lookup key
	key := encodeKey{
		cq:   context,
		rho:  rho,
		uOff: uOff,
		ek:   ek,
		e1:   e1,
	}

	var entry VLCEncodeEntry
	var found bool

	if isFirstRow {
		entry, found = v.encodeMap0[key]
	} else {
		entry, found = v.encodeMap1[key]
	}

	if !found {
		// Fallback: search entry with matching (context, rho, uOff) and most EMB bits
		// Per ISO/IEC 15444-15:2019 Annex C: select entry with most set bits in (EK & ek_table) | (E1 & e1_table)
        if isFirstRow {
            maxBits := -1
            var best VLCEncodeEntry
            for _, tblEntry := range VLC_tbl0 {
                if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == uOff {
                    // Count matching EMB bits
                    ekMatch := ek & tblEntry.EK
                    e1Match := e1 & tblEntry.E1
                    matchBits := countBits(ekMatch) + countBits(e1Match)

                    if matchBits > maxBits {
                        maxBits = matchBits
                        best = VLCEncodeEntry{Codeword: tblEntry.Cwd, Length: tblEntry.CwdLen, Valid: true}
                    }
                }
            }
            if maxBits >= 0 {
                entry = best
                found = true
            }
        } else {
            maxBits := -1
            var best VLCEncodeEntry
            for _, tblEntry := range VLC_tbl1 {
                if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == uOff {
                    // Count matching EMB bits
                    ekMatch := ek & tblEntry.EK
                    e1Match := e1 & tblEntry.E1
                    matchBits := countBits(ekMatch) + countBits(e1Match)

                    if matchBits > maxBits {
                        maxBits = matchBits
                        best = VLCEncodeEntry{Codeword: tblEntry.Cwd, Length: tblEntry.CwdLen, Valid: true}
                    }
                }
            }
            if maxBits >= 0 {
                entry = best
                found = true
            }
        }
		if !found {
			return fmt.Errorf("no VLC entry found for context=%d, rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", context, rho, uOff, ek, e1)
		}
	}

	// Emit the codeword
	return v.emitVLCBits(uint32(entry.Codeword), int(entry.Length))
}

func (v *VLCEncoder) EncodeCxtVLCWithLen(context, rho, uOff, ek, e1 uint8, isFirstRow bool) (int, error) {
	// Create lookup key
	key := encodeKey{cq: context, rho: rho, uOff: uOff, ek: ek, e1: e1}
	var entry VLCEncodeEntry
	var found bool
	if isFirstRow {
		entry, found = v.encodeMap0[key]
	} else {
		entry, found = v.encodeMap1[key]
	}
	if !found {
		// Fallback: search entry with matching (context, rho, uOff) and most EMB bits
		// Per ISO/IEC 15444-15:2019 Annex C: select entry with most set bits in (EK & ek_table) | (E1 & e1_table)
		if isFirstRow {
			maxBits := -1
			var best VLCEncodeEntry
			for _, tblEntry := range VLC_tbl0 {
				if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == uOff {
					// Count matching EMB bits
					ekMatch := ek & tblEntry.EK
					e1Match := e1 & tblEntry.E1
					matchBits := countBits(ekMatch) + countBits(e1Match)

					if matchBits > maxBits {
						maxBits = matchBits
						best = VLCEncodeEntry{Codeword: tblEntry.Cwd, Length: tblEntry.CwdLen, Valid: true}
					}
				}
			}
			if maxBits >= 0 {
				entry = best
				found = true
			}
		} else {
			maxBits := -1
			var best VLCEncodeEntry
			for _, tblEntry := range VLC_tbl1 {
				if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == uOff {
					// Count matching EMB bits
					ekMatch := ek & tblEntry.EK
					e1Match := e1 & tblEntry.E1
					matchBits := countBits(ekMatch) + countBits(e1Match)

					if matchBits > maxBits {
						maxBits = matchBits
						best = VLCEncodeEntry{Codeword: tblEntry.Cwd, Length: tblEntry.CwdLen, Valid: true}
					}
				}
			}
			if maxBits >= 0 {
				entry = best
				found = true
			}
		}
		if !found {
			return 0, fmt.Errorf("no VLC entry found for context=%d, rho=0x%X, uOff=%d, ek=0x%X, e1=0x%X", context, rho, uOff, ek, e1)
		}
	}
	if err := v.emitVLCBits(uint32(entry.Codeword), int(entry.Length)); err != nil {
		return 0, err
	}
	return int(entry.Length), nil
}

// FlushForFusion finalizes the VLC stream and returns data for MEL/VLC fusion
// Returns: data bytes (including trailing 0xFF), last data byte, number of used bits in last byte
// Reference: OpenJPH ojph_block_encoder.cpp lines 420-446
func (v *VLCEncoder) FlushForFusion() ([]byte, uint8, int) {
	usedBits := 0
	lastByte := uint8(0)

	// Flush any remaining bits, padding with 1s
	if v.vlcBits > 0 {
		usedBits = v.vlcBits
		// Pad remaining bits with 1s to fill the byte
		for v.vlcBits < 8 {
			v.vlcTmp |= (1 << v.vlcBits)
			v.vlcBits++
		}
		lastByte = v.vlcTmp
		v.vlcBuf = append(v.vlcBuf, lastByte)
	}

	// Add trailing padding byte (0xFF) to ensure decoder can read 7 bits
	v.vlcBuf = append(v.vlcBuf, 0xFF)

	// Skip the first byte (initialization byte) and return the rest AS-IS
	if len(v.vlcBuf) <= 1 {
		return []byte{}, 0, 0
	}

	result := make([]byte, len(v.vlcBuf)-1)
	copy(result, v.vlcBuf[1:])

	// Get last data byte (before trailing 0xFF)
	if len(result) > 1 {
		lastByte = result[len(result)-2] // -2 because last is 0xFF padding
		if usedBits == 0 {
			usedBits = 8 // Full byte
		}
	}

	return result, lastByte, usedBits
}

// Flush flushes any pending bits and returns the VLC byte-stream
// The byte-stream is reversed as per spec requirements
func (v *VLCEncoder) Flush() []byte {
	// Flush any remaining bits, padding with 1s to byte boundary
	if v.vlcBits > 0 {
		// Pad remaining bits with 1s to fill the byte
		for v.vlcBits < 8 {
			v.vlcTmp |= (1 << v.vlcBits)
			v.vlcBits++
		}
		v.vlcBuf = append(v.vlcBuf, v.vlcTmp)
	}

	// Add trailing padding byte (0xFF) to ensure decoder can read 7 bits
	v.vlcBuf = append(v.vlcBuf, 0xFF)

	// Skip the first byte (255) and return the rest AS-IS (don't reverse)
	if len(v.vlcBuf) <= 1 {
		return []byte{}
	}

	// Return from byte 1 onwards without reversal
	result := make([]byte, len(v.vlcBuf)-1)
	copy(result, v.vlcBuf[1:])

	return result
}

// Reset resets the encoder state for encoding a new block
func (v *VLCEncoder) Reset() {
	v.vlcBuf = v.vlcBuf[:0]
	v.initVLCPacker()
}

// countBits counts the number of set bits in a uint8 value
func countBits(val uint8) int {
	return bits.OnesCount8(val)
}

// EncodeQuadVLCByEMB encodes a quad using OpenJPH-compatible EMB lookup
// emb = eps0 = mask of which samples have exponent == max_exponent
// Returns: (codeword_length, table_e_k, error)
// The table_e_k is used by the caller for MagSgn encoding: mn = Uq - ekBit
func (v *VLCEncoder) EncodeQuadVLCByEMB(context, rho, uOff, emb uint8, isFirstRow bool) (int, uint8, error) {
	tbl := VLC_tbl0
	if !isFirstRow {
		tbl = VLC_tbl1
	}

	if uOff == 0 || emb == 0 {
		// u_off = 0 or emb = 0: no EMB, find entry with u_off=0
		for _, tblEntry := range tbl {
			if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == 0 {
				if err := v.emitVLCBits(uint32(tblEntry.Cwd), int(tblEntry.CwdLen)); err != nil {
					return 0, 0, err
				}
				return int(tblEntry.CwdLen), 0, nil
			}
		}
	} else {
		// u_off = 1: find best match using OpenJPH condition: (emb & entry.EK) == entry.E1
		// Pick entry with highest popcount of EK
		maxEK := -1
		var bestEntry *VLCEntry
		for i := range tbl {
			tblEntry := &tbl[i]
			if tblEntry.CQ == context && tblEntry.Rho == rho && tblEntry.UOff == 1 {
				if (emb & tblEntry.EK) == tblEntry.E1 {
					ekBits := countBits(tblEntry.EK)
					if ekBits > maxEK {
						maxEK = ekBits
						bestEntry = tblEntry
					}
				}
			}
		}
		if bestEntry != nil {
			if err := v.emitVLCBits(uint32(bestEntry.Cwd), int(bestEntry.CwdLen)); err != nil {
				return 0, 0, err
			}
			return int(bestEntry.CwdLen), bestEntry.EK, nil
		}
	}

	return 0, 0, fmt.Errorf("no VLC entry for context=%d, rho=0x%X, uOff=%d, emb=0x%X", context, rho, uOff, emb)
}
