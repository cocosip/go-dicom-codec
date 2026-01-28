package htj2k

// MEL (Adaptive Run-Length Coding) - Based on ISO/IEC 15444-15:2019
// Reference: Clause 7.3.3 - MEL symbol decoding procedure

// MEL_E is the exponent table for MEL decoding
// Table 2 from ISO/IEC 15444-15:2019
var MEL_E = [13]int{
	0, // k=0
	0, // k=1
	0, // k=2
	1, // k=3
	1, // k=4
	1, // k=5
	2, // k=6
	2, // k=7
	2, // k=8
	3, // k=9
	3, // k=10
	4, // k=11
	5, // k=12
}

// MELDecoderSpec implements the MEL decoder based on ISO/IEC 15444-15 specification
type MELDecoderSpec struct {
	// Input data
	data []byte
	pos  int

	// Bit-level reading
	bitBuffer byte
	bitPos    int
	bitLimit  int
	lastByte  byte

	// State machine variables (from spec)
	MEL_k   int // Current state (0-12)
	MEL_run int // Run length
	MEL_one int // One flag
}

// NewMELDecoderSpec creates a new spec-compliant MEL decoder
func NewMELDecoderSpec(data []byte) *MELDecoderSpec {
	mel := &MELDecoderSpec{
		data:     data,
		pos:      0,
		bitPos:   8, // Will load first byte on first read
		bitLimit: 8,
		lastByte: 0x00,
	}
	mel.initMELDecoder()
	return mel
}

// initMELDecoder initializes the MEL decoder state
// Procedure: initMELDecoder from ISO/IEC 15444-15:2019
func (m *MELDecoderSpec) initMELDecoder() {
	m.MEL_k = 0
	m.MEL_run = 0
	m.MEL_one = 0
}

// DecodeMELSym decodes the next MEL symbol
// Returns: (symbol, hasMore)
// symbol: 0 = continue run (all-zero context), 1 = end run (has significant samples)
// Procedure: decodeMELSym from ISO/IEC 15444-15:2019 Clause 7.3.3
func (m *MELDecoderSpec) DecodeMELSym() (int, bool) {
	// if (MEL_run == 0) and (MEL_one == 0)
	if m.MEL_run == 0 && m.MEL_one == 0 {
		eval := MEL_E[m.MEL_k]
		bit, ok := m.importMELBit()
		if !ok {
			return 0, false
		}

		if bit == 1 {
			// MEL_run = 1 << eval
			m.MEL_run = 1 << eval
			// MEL_k = min(12, MEL_k + 1)
			m.MEL_k = min(12, m.MEL_k+1)
		} else {
			// MEL_run = 0
			m.MEL_run = 0
			// while (eval > 0)
			for eval > 0 {
				bit, ok = m.importMELBit()
				if !ok {
					return 0, false
				}
				// MEL_run = 2 * MEL_run + bit
				m.MEL_run = 2*m.MEL_run + bit
				eval--
			}
			// MEL_k = max(0, MEL_k - 1)
			m.MEL_k = max(0, m.MEL_k-1)
			// MEL_one = 1
			m.MEL_one = 1
		}
	}

	// if (MEL_run > 0)
	if m.MEL_run > 0 {
		m.MEL_run--
		return 0, true // Continue run
	} else {
		m.MEL_one = 0
		return 1, true // End run
	}
}

// importMELBit reads a single bit from the MEL bit-stream
func (m *MELDecoderSpec) importMELBit() (int, bool) {
	// Load next byte if needed
	if m.bitPos >= m.bitLimit {
		if m.pos >= len(m.data) {
			return 0, false
		}
		b := m.data[m.pos]
		m.pos++
		if m.lastByte == 0xFF {
			m.bitBuffer = (b & 0x7F) << 1
			m.bitLimit = 7
		} else {
			m.bitBuffer = b
			m.bitLimit = 8
		}
		m.bitPos = 0
		m.lastByte = b
	}

	// Extract bit (MSB first)
	bit := (m.bitBuffer >> (m.bitLimit - 1 - m.bitPos)) & 1
	m.bitPos++

	return int(bit), true
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
