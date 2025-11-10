package common

// WriteHuffmanTable writes a Huffman table to the JPEG stream
// class: 0 for DC, 1 for AC
// id: table ID (0 or 1)
func WriteHuffmanTable(writer *Writer, class byte, id byte, table *HuffmanTable) error {
	// Calculate total number of values
	totalValues := 0
	for _, count := range table.Bits {
		totalValues += count
	}

	// Create DHT segment data
	data := make([]byte, 1+16+totalValues)
	data[0] = (class << 4) | id // Table class and ID

	// Write bit counts (16 bytes)
	for i := 0; i < 16; i++ {
		data[1+i] = byte(table.Bits[i])
	}

	// Write symbol values
	copy(data[17:], table.Values)

	// Write DHT segment
	return writer.WriteSegment(MarkerDHT, data)
}
