package codestream

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Parser parses JPEG 2000 codestreams
type Parser struct {
	data   []byte
	offset int
}

// NewParser creates a new codestream parser
func NewParser(data []byte) *Parser {
	return &Parser{
		data:   data,
		offset: 0,
	}
}

// Parse parses the entire codestream
func (p *Parser) Parse() (*Codestream, error) {
	cs := &Codestream{
		Data: p.data,
	}

	// Read SOC marker
	marker, err := p.readMarker()
	if err != nil {
		return nil, fmt.Errorf("failed to read SOC: %w", err)
	}
	if marker != MarkerSOC {
		return nil, fmt.Errorf("expected SOC marker (0x%04X), got 0x%04X", MarkerSOC, marker)
	}

	// Parse main header
	if err := p.parseMainHeader(cs); err != nil {
		return nil, fmt.Errorf("failed to parse main header: %w", err)
	}

	// Parse tiles
	for {
		marker, err := p.peekMarker()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if marker == MarkerEOC {
			// End of codestream
			_, _ = p.readMarker() // consume EOC
			break
		}

		if marker == MarkerSOT {
			tile, err := p.parseTile(cs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tile: %w", err)
			}
			cs.Tiles = append(cs.Tiles, tile)
		} else {
			return nil, fmt.Errorf("unexpected marker in tile sequence: 0x%04X (%s)", marker, MarkerName(marker))
		}
	}

	return cs, nil
}

// parseMainHeader parses the main header segments
func (p *Parser) parseMainHeader(cs *Codestream) error {
	for {
		marker, err := p.peekMarker()
		if err != nil {
			return err
		}

		// Main header ends when we hit SOT or EOC
		if marker == MarkerSOT || marker == MarkerEOC {
			break
		}

		// Read the marker
		marker, err = p.readMarker()
		if err != nil {
			return err
		}

		// Parse segment based on marker type
		switch marker {
		case MarkerSIZ:
			siz, err := p.parseSIZ()
			if err != nil {
				return fmt.Errorf("failed to parse SIZ: %w", err)
			}
			cs.SIZ = siz

		case MarkerCOD:
			cod, err := p.parseCOD()
			if err != nil {
				return fmt.Errorf("failed to parse COD: %w", err)
			}
			cs.COD = cod

		case MarkerQCD:
			qcd, err := p.parseQCD()
			if err != nil {
				return fmt.Errorf("failed to parse QCD: %w", err)
			}
			cs.QCD = qcd

		case MarkerRGN:
			rgn, err := p.parseRGN()
			if err != nil {
				return fmt.Errorf("failed to parse RGN: %w", err)
			}
			cs.RGN = append(cs.RGN, *rgn)

		case MarkerCOM:
			com, err := p.parseCOM()
			if err != nil {
				return fmt.Errorf("failed to parse COM: %w", err)
			}
			cs.COM = append(cs.COM, *com)

		case MarkerMCT:
			seg, err := p.parseMCT()
			if err != nil {
				return fmt.Errorf("failed to parse MCT: %w", err)
			}
			cs.MCT = append(cs.MCT, *seg)

		case MarkerMCC:
			seg, err := p.parseMCC()
			if err != nil {
				return fmt.Errorf("failed to parse MCC: %w", err)
			}
			cs.MCC = append(cs.MCC, *seg)

		case MarkerMCO:
			seg, err := p.parseMCO()
			if err != nil {
				return fmt.Errorf("failed to parse MCO: %w", err)
			}
			cs.MCO = append(cs.MCO, *seg)

		default:
			// Skip unknown segments
			if err := p.skipSegment(); err != nil {
				return fmt.Errorf("failed to skip segment 0x%04X: %w", marker, err)
			}
		}
	}

	// Verify required segments
	if cs.SIZ == nil {
		return fmt.Errorf("missing required SIZ segment")
	}
	if cs.COD == nil {
		return fmt.Errorf("missing required COD segment")
	}
	if cs.QCD == nil {
		return fmt.Errorf("missing required QCD segment")
	}

	return nil
}

// parseTile parses a single tile
func (p *Parser) parseTile(cs *Codestream) (*Tile, error) {
	tileStart := p.offset
	// Read SOT
	marker, err := p.readMarker()
	if err != nil {
		return nil, err
	}
	if marker != MarkerSOT {
		return nil, fmt.Errorf("expected SOT, got 0x%04X", marker)
	}

	sot, err := p.parseSOT()
	if err != nil {
		return nil, err
	}

	tile := &Tile{
		Index: int(sot.Isot),
		SOT:   sot,
	}

	// Parse tile-part header
	for {
		marker, err := p.peekMarker()
		if err != nil {
			return nil, err
		}

		if marker == MarkerSOD {
			// Start of data - tile header complete
			_, _ = p.readMarker() // consume SOD
			break
		}

		marker, err = p.readMarker()
		if err != nil {
			return nil, err
		}

		switch marker {
		case MarkerCOD:
			cod, err := p.parseCOD()
			if err != nil {
				return nil, err
			}
			tile.COD = cod

		case MarkerQCD:
			qcd, err := p.parseQCD()
			if err != nil {
				return nil, err
			}
			tile.QCD = qcd

		case MarkerRGN:
			// Parse tile-part RGN (tile-specific ROI)
			rgn, err := p.parseRGN()
			if err != nil {
				return nil, err
			}
			tile.RGN = append(tile.RGN, rgn)

		case MarkerMCT:
			seg, err := p.parseMCT()
			if err != nil {
				return nil, err
			}
			if cs != nil {
				cs.MCT = append(cs.MCT, *seg)
			}

		case MarkerMCC:
			seg, err := p.parseMCC()
			if err != nil {
				return nil, err
			}
			if cs != nil {
				cs.MCC = append(cs.MCC, *seg)
			}

		case MarkerMCO:
			seg, err := p.parseMCO()
			if err != nil {
				return nil, err
			}
			if cs != nil {
				cs.MCO = append(cs.MCO, *seg)
			}

		default:
			// Skip unknown tile-part header segments
			if err := p.skipSegment(); err != nil {
				return nil, err
			}
		}
	}

	// Read tile data using Psot length when available.
	tile.Data = p.readTileDataWithLength(tileStart, sot.Psot)

	return tile, nil
}

// parseRGN parses the RGN marker segment (ROI)
// Baseline assumption: Csiz <= 256, so Crgn fits in 1 byte.
func (p *Parser) parseRGN() (*RGNSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if length < 5 {
		return nil, fmt.Errorf("invalid RGN length: %d", length)
	}

	crgn, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	srgn, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	sprgn, err := p.readUint8()
	if err != nil {
		return nil, err
	}

	// Skip remaining bytes if any
	remain := int(length) - 5
	if remain > 0 {
		if _, err := p.read(make([]byte, remain)); err != nil {
			return nil, err
		}
	}

	return &RGNSegment{
		Crgn:  uint16(crgn),
		Srgn:  srgn,
		SPrgn: sprgn,
	}, nil
}

// parseSIZ parses the SIZ marker segment
func (p *Parser) parseSIZ() (*SIZSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	siz := &SIZSegment{}

	if siz.Rsiz, err = p.readUint16(); err != nil {
		return nil, err
	}
	if siz.Xsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.Ysiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XTsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YTsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XTOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YTOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.Csiz, err = p.readUint16(); err != nil {
		return nil, err
	}

	// Read component sizing information
	siz.Components = make([]ComponentSize, siz.Csiz)
	for i := range siz.Components {
		if siz.Components[i].Ssiz, err = p.readUint8(); err != nil {
			return nil, err
		}
		if siz.Components[i].XRsiz, err = p.readUint8(); err != nil {
			return nil, err
		}
		if siz.Components[i].YRsiz, err = p.readUint8(); err != nil {
			return nil, err
		}
	}

	// Verify length
	expectedLength := 38 + 3*int(siz.Csiz)
	if int(length) != expectedLength {
		return nil, fmt.Errorf("SIZ segment length mismatch: expected %d, got %d", expectedLength, length)
	}

	return siz, nil
}

// parseCOD parses the COD marker segment
func (p *Parser) parseCOD() (*CODSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	cod := &CODSegment{}

	if cod.Scod, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.ProgressionOrder, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.NumberOfLayers, err = p.readUint16(); err != nil {
		return nil, err
	}
	if cod.MultipleComponentTransform, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.NumberOfDecompositionLevels, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.CodeBlockWidth, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.CodeBlockHeight, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.CodeBlockStyle, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.Transformation, err = p.readUint8(); err != nil {
		return nil, err
	}

	// Read precinct sizes if Scod bit 0 is set
	if (cod.Scod & 0x01) != 0 {
		numLevels := int(cod.NumberOfDecompositionLevels) + 1
		cod.PrecinctSizes = make([]PrecinctSize, numLevels)
		for i := 0; i < numLevels; i++ {
			ppxppy, err := p.readUint8()
			if err != nil {
				return nil, err
			}
			cod.PrecinctSizes[i].PPx = ppxppy & 0x0F
			cod.PrecinctSizes[i].PPy = ppxppy >> 4
		}
	}

	_ = length // length validation skipped for now

	return cod, nil
}

// parseQCD parses the QCD marker segment
func (p *Parser) parseQCD() (*QCDSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	qcd := &QCDSegment{}

	if qcd.Sqcd, err = p.readUint8(); err != nil {
		return nil, err
	}

	// Read quantization step size values
	dataLength := int(length) - 3 // length includes itself (2) and Sqcd (1)
	qcd.SPqcd = make([]byte, dataLength)
	if _, err := p.read(qcd.SPqcd); err != nil {
		return nil, err
	}

	return qcd, nil
}

// parseCOM parses the COM marker segment
func (p *Parser) parseCOM() (*COMSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	com := &COMSegment{}

	if com.Rcom, err = p.readUint16(); err != nil {
		return nil, err
	}

	dataLength := int(length) - 4 // length includes itself (2) and Rcom (2)
	com.Data = make([]byte, dataLength)
	if _, err := p.read(com.Data); err != nil {
		return nil, err
	}

	return com, nil
}

func (p *Parser) parseMCT() (*MCTSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 6 {
		return nil, fmt.Errorf("invalid MCT length")
	}
	zmct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if zmct != 0 {
		return nil, fmt.Errorf("unsupported Zmct=%d", zmct)
	}
	imct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	ymct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if ymct != 0 {
		return nil, fmt.Errorf("unsupported Ymct=%d", ymct)
	}
	idx := uint8(imct & 0xFF)
	at := uint8((imct >> 8) & 0x3)
	et := uint8((imct >> 10) & 0x3)
	dataLen := payloadLen - 6
	buf := make([]byte, dataLen)
	if _, err := p.read(buf); err != nil {
		return nil, err
	}
	return &MCTSegment{Index: idx, ElementType: MCTElementType(et), ArrayType: MCTArrayType(at), Data: buf}, nil
}

func (p *Parser) parseMCC() (*MCCSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 7 {
		return nil, fmt.Errorf("invalid MCC length")
	}
	zmcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if zmcc != 0 {
		return nil, fmt.Errorf("unsupported Zmcc=%d", zmcc)
	}
	idx, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	ymcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if ymcc != 0 {
		return nil, fmt.Errorf("unsupported Ymcc=%d", ymcc)
	}
	qmcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if qmcc == 0 {
		return nil, fmt.Errorf("invalid MCC collections")
	}

	collectionType, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	nmcci, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	compBytes := 1
	if (nmcci & 0x8000) != 0 {
		compBytes = 2
	}
	numComps := nmcci & 0x7FFF
	comps := make([]uint16, numComps)
	for i := 0; i < int(numComps); i++ {
		var v uint16
		if compBytes == 1 {
			b, err := p.readUint8()
			if err != nil {
				return nil, err
			}
			v = uint16(b)
		} else {
			v, err = p.readUint16()
			if err != nil {
				return nil, err
			}
		}
		comps[i] = v
	}

	mmcci, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	outCompBytes := 1
	if (mmcci & 0x8000) != 0 {
		outCompBytes = 2
	}
	outCompsCount := mmcci & 0x7FFF
	outComps := make([]uint16, outCompsCount)
	for i := 0; i < int(outCompsCount); i++ {
		var v uint16
		if outCompBytes == 1 {
			b, err := p.readUint8()
			if err != nil {
				return nil, err
			}
			v = uint16(b)
		} else {
			v, err = p.readUint16()
			if err != nil {
				return nil, err
			}
		}
		outComps[i] = v
	}

	b0, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	b1, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	b2, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	tmcc := (uint32(b0) << 16) | (uint32(b1) << 8) | uint32(b2)
	reversible := ((tmcc >> 16) & 0x1) != 0
	decoIdx := uint8(tmcc & 0xFF)
	offIdx := uint8((tmcc >> 8) & 0xFF)

	consumed := 2 + 1 + 2 + 2 + 1 + 2 + (compBytes * int(numComps)) + 2 + (outCompBytes * int(outCompsCount)) + 3
	remain := payloadLen - consumed
	if remain > 0 {
		buf := make([]byte, remain)
		if _, err := p.read(buf); err != nil {
			return nil, err
		}
	}

	return &MCCSegment{
		Index:              idx,
		CollectionType:     collectionType,
		NumComponents:      numComps,
		ComponentIDs:       comps,
		OutputComponentIDs: outComps,
		DecorrelateIndex:   decoIdx,
		OffsetIndex:        offIdx,
		Reversible:         reversible,
	}, nil
}

func (p *Parser) parseMCO() (*MCOSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 1 {
		return nil, fmt.Errorf("invalid MCO length")
	}
	numStages, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	stageCount := int(numStages)
	stages := make([]uint8, 0, stageCount)
	for i := 0; i < stageCount; i++ {
		v, err := p.readUint8()
		if err != nil {
			return nil, err
		}
		stages = append(stages, v)
	}
	remain := payloadLen - (1 + stageCount)
	if remain > 0 {
		buf := make([]byte, remain)
		if _, err := p.read(buf); err != nil {
			return nil, err
		}
	}
	return &MCOSegment{NumStages: numStages, StageIndices: stages}, nil
}

// parseSOT parses the SOT marker segment
func (p *Parser) parseSOT() (*SOTSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	if length != 10 {
		return nil, fmt.Errorf("invalid SOT segment length: %d", length)
	}

	sot := &SOTSegment{}

	if sot.Isot, err = p.readUint16(); err != nil {
		return nil, err
	}
	if sot.Psot, err = p.readUint32(); err != nil {
		return nil, err
	}
	if sot.TPsot, err = p.readUint8(); err != nil {
		return nil, err
	}
	if sot.TNsot, err = p.readUint8(); err != nil {
		return nil, err
	}

	return sot, nil
}

// Helper methods for reading data

func (p *Parser) readMarker() (uint16, error) {
	return p.readUint16()
}

func (p *Parser) peekMarker() (uint16, error) {
	if p.offset+2 > len(p.data) {
		return 0, io.EOF
	}
	marker := binary.BigEndian.Uint16(p.data[p.offset : p.offset+2])
	return marker, nil
}

func (p *Parser) readUint8() (uint8, error) {
	if p.offset+1 > len(p.data) {
		return 0, io.EOF
	}
	val := p.data[p.offset]
	p.offset++
	return val, nil
}

func (p *Parser) readUint16() (uint16, error) {
	if p.offset+2 > len(p.data) {
		return 0, io.EOF
	}
	val := binary.BigEndian.Uint16(p.data[p.offset : p.offset+2])
	p.offset += 2
	return val, nil
}

func (p *Parser) readUint32() (uint32, error) {
	if p.offset+4 > len(p.data) {
		return 0, io.EOF
	}
	val := binary.BigEndian.Uint32(p.data[p.offset : p.offset+4])
	p.offset += 4
	return val, nil
}

func (p *Parser) read(buf []byte) (int, error) {
	if p.offset+len(buf) > len(p.data) {
		return 0, io.EOF
	}
	n := copy(buf, p.data[p.offset:p.offset+len(buf)])
	p.offset += n
	return n, nil
}

func (p *Parser) skipSegment() error {
	length, err := p.readUint16()
	if err != nil {
		return err
	}
	// length includes the 2 bytes for length itself
	skip := int(length) - 2
	if p.offset+skip > len(p.data) {
		return io.EOF
	}
	p.offset += skip
	return nil
}

func (p *Parser) readTileData() []byte {
	start := p.offset

	// Read until we hit a marker (0xFF followed by non-0xFF)
	for p.offset < len(p.data) {
		if p.data[p.offset] == 0xFF && p.offset+1 < len(p.data) {
			next := p.data[p.offset+1]
			// Check if this is a valid marker (not 0xFF00)
			if next != 0x00 && next >= 0x4F {
				// This is a marker, stop here
				break
			}
		}
		p.offset++
	}

	return p.data[start:p.offset]
}

func (p *Parser) readTileDataWithLength(tileStart int, psot uint32) []byte {
	if psot == 0 {
		return p.readTileData()
	}
	remaining := int(psot) - (p.offset - tileStart)
	if remaining <= 0 {
		return []byte{}
	}
	if p.offset+remaining > len(p.data) {
		remaining = len(p.data) - p.offset
	}
	start := p.offset
	p.offset += remaining
	return p.data[start:p.offset]
}
