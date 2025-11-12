package t2

import (
	"testing"
)

// TestProgressionOrder tests progression order string representation
func TestProgressionOrder(t *testing.T) {
	tests := []struct {
		order    ProgressionOrder
		expected string
	}{
		{ProgressionLRCP, "LRCP"},
		{ProgressionRLCP, "RLCP"},
		{ProgressionRPCL, "RPCL"},
		{ProgressionPCRL, "PCRL"},
		{ProgressionCPRL, "CPRL"},
		{ProgressionOrder(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.order.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestTagTreeCreation tests tag tree creation
func TestTagTreeCreation(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"1x1", 1, 1},
		{"2x2", 2, 2},
		{"4x4", 4, 4},
		{"8x8", 8, 8},
		{"Non-square 4x2", 4, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTagTree(tt.width, tt.height)

			if tree == nil {
				t.Fatal("NewTagTree returned nil")
			}

			if tree.Width() != tt.width {
				t.Errorf("Width = %d, want %d", tree.Width(), tt.width)
			}

			if tree.Height() != tt.height {
				t.Errorf("Height = %d, want %d", tree.Height(), tt.height)
			}

			if tree.GetNumLevels() == 0 {
				t.Error("Number of levels is 0")
			}
		})
	}
}

// TestTagTreeReset tests tag tree reset functionality
func TestTagTreeReset(t *testing.T) {
	tree := NewTagTree(4, 4)

	// Set some values using SetValue
	tree.SetValue(0, 0, 5)
	tree.SetValue(1, 1, 10)
	tree.SetValue(2, 2, 15)

	// Verify values are set
	if tree.GetValue(0, 0) != 5 {
		t.Errorf("GetValue(0, 0) = %d, want 5", tree.GetValue(0, 0))
	}

	// Reset
	tree.Reset()

	// Verify all values are reset to 0
	if tree.GetValue(0, 0) != 0 {
		t.Errorf("After reset, GetValue(0, 0) = %d, want 0", tree.GetValue(0, 0))
	}
	if tree.GetValue(1, 1) != 0 {
		t.Errorf("After reset, GetValue(1, 1) = %d, want 0", tree.GetValue(1, 1))
	}
	if tree.GetValue(2, 2) != 0 {
		t.Errorf("After reset, GetValue(2, 2) = %d, want 0", tree.GetValue(2, 2))
	}
}

// TestPacketIteratorLRCP tests LRCP progression order iteration
func TestPacketIteratorLRCP(t *testing.T) {
	// Small test case: 1 component, 2 layers, 2 resolutions
	pi := NewPacketIterator(
		1,    // numComponents
		2,    // numLayers
		2,    // numResolutions
		64,   // tileWidth
		64,   // tileHeight
		32,   // precinctWidth
		32,   // precinctHeight
		ProgressionLRCP,
	)

	packets := []struct {
		layer, resolution, component, px, py int
	}{}

	for {
		l, r, c, px, py, ok := pi.Next()
		if !ok {
			break
		}
		packets = append(packets, struct {
			layer, resolution, component, px, py int
		}{l, r, c, px, py})
	}

	// Should have: 2 layers * 2 resolutions * 1 component * 4 precincts = 16 packets
	expectedCount := 16
	if len(packets) != expectedCount {
		t.Errorf("Got %d packets, want %d", len(packets), expectedCount)
	}

	// Verify first packet is at (0,0,0,0,0) - layer 0, resolution 0, component 0, precinct (0,0)
	if len(packets) > 0 {
		first := packets[0]
		if first.layer != 0 || first.resolution != 0 || first.component != 0 ||
			first.px != 0 || first.py != 0 {
			t.Errorf("First packet = (%d,%d,%d,%d,%d), want (0,0,0,0,0)",
				first.layer, first.resolution, first.component, first.px, first.py)
		}
	}

	// Verify last packet
	if len(packets) > 0 {
		last := packets[len(packets)-1]
		if last.layer != 1 { // Last layer
			t.Errorf("Last packet layer = %d, want 1", last.layer)
		}
	}
}

// TestPacketIteratorRLCP tests RLCP progression order iteration
func TestPacketIteratorRLCP(t *testing.T) {
	pi := NewPacketIterator(
		1, 2, 2,
		64, 64,
		32, 32,
		ProgressionRLCP,
	)

	count := 0
	for {
		_, _, _, _, _, ok := pi.Next()
		if !ok {
			break
		}
		count++
	}

	expectedCount := 16
	if count != expectedCount {
		t.Errorf("Got %d packets, want %d", count, expectedCount)
	}
}

// TestPacketIteratorMultipleComponents tests iteration with multiple components
func TestPacketIteratorMultipleComponents(t *testing.T) {
	// RGB image: 3 components
	pi := NewPacketIterator(
		3,  // numComponents (RGB)
		1,  // numLayers
		1,  // numResolutions
		32, // tileWidth
		32, // tileHeight
		32, // precinctWidth (1 precinct)
		32, // precinctHeight
		ProgressionLRCP,
	)

	count := 0
	components := make(map[int]bool)
	for {
		_, _, c, _, _, ok := pi.Next()
		if !ok {
			break
		}
		count++
		components[c] = true
	}

	// Should have 1 layer * 1 resolution * 3 components * 1 precinct = 3 packets
	expectedCount := 3
	if count != expectedCount {
		t.Errorf("Got %d packets, want %d", count, expectedCount)
	}

	// Should have seen all 3 components
	if len(components) != 3 {
		t.Errorf("Saw %d components, want 3", len(components))
	}
}

// TestPacketIteratorEdgeCases tests edge cases
func TestPacketIteratorEdgeCases(t *testing.T) {
	t.Run("Single packet", func(t *testing.T) {
		pi := NewPacketIterator(1, 1, 1, 32, 32, 32, 32, ProgressionLRCP)
		count := 0
		for {
			_, _, _, _, _, ok := pi.Next()
			if !ok {
				break
			}
			count++
		}
		if count != 1 {
			t.Errorf("Got %d packets, want 1", count)
		}
	})

	t.Run("Multiple precincts", func(t *testing.T) {
		// 4x4 precincts
		pi := NewPacketIterator(1, 1, 1, 128, 128, 32, 32, ProgressionLRCP)
		count := 0
		for {
			_, _, _, _, _, ok := pi.Next()
			if !ok {
				break
			}
			count++
		}
		// 1 layer * 1 resolution * 1 component * 16 precincts = 16
		if count != 16 {
			t.Errorf("Got %d packets, want 16", count)
		}
	})
}

// TestPrecinctCalculation tests precinct calculation
func TestPrecinctCalculation(t *testing.T) {
	tests := []struct {
		name                                     string
		tileWidth, tileHeight                    int
		precinctWidth, precinctHeight            int
		expectedPrecinctX, expectedPrecinctY     int
	}{
		{"Exact fit", 64, 64, 32, 32, 2, 2},
		{"Non-exact fit", 100, 100, 32, 32, 4, 4},
		{"One precinct", 32, 32, 64, 64, 1, 1},
		{"Large tile", 512, 512, 64, 64, 8, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numPrecinctX := (tt.tileWidth + tt.precinctWidth - 1) / tt.precinctWidth
			numPrecinctY := (tt.tileHeight + tt.precinctHeight - 1) / tt.precinctHeight

			if numPrecinctX != tt.expectedPrecinctX {
				t.Errorf("numPrecinctX = %d, want %d", numPrecinctX, tt.expectedPrecinctX)
			}

			if numPrecinctY != tt.expectedPrecinctY {
				t.Errorf("numPrecinctY = %d, want %d", numPrecinctY, tt.expectedPrecinctY)
			}
		})
	}
}

// TestPacketStructure tests packet structure
func TestPacketStructure(t *testing.T) {
	packet := &Packet{
		HeaderPresent:  true,
		LayerIndex:     0,
		ResolutionLevel: 2,
		ComponentIndex: 0,
		PrecinctIndex:  5,
		CodeBlockIncls: []CodeBlockIncl{
			{Included: true, FirstInclusion: true, NumPasses: 3, DataLength: 100},
			{Included: true, FirstInclusion: false, NumPasses: 2, DataLength: 50},
		},
	}

	if !packet.HeaderPresent {
		t.Error("HeaderPresent should be true")
	}

	if len(packet.CodeBlockIncls) != 2 {
		t.Errorf("CodeBlockIncls length = %d, want 2", len(packet.CodeBlockIncls))
	}

	if packet.CodeBlockIncls[0].NumPasses != 3 {
		t.Errorf("First code-block NumPasses = %d, want 3", packet.CodeBlockIncls[0].NumPasses)
	}
}

// TestPrecinctStructure tests precinct structure
func TestPrecinctStructure(t *testing.T) {
	precinct := &Precinct{
		Index:  0,
		X0:     0,
		Y0:     0,
		X1:     64,
		Y1:     64,
		Width:  64,
		Height: 64,
	}

	if precinct.Width != 64 || precinct.Height != 64 {
		t.Errorf("Precinct size = %dx%d, want 64x64", precinct.Width, precinct.Height)
	}

	expectedArea := precinct.Width * precinct.Height
	actualArea := (precinct.X1 - precinct.X0) * (precinct.Y1 - precinct.Y0)
	if actualArea != expectedArea {
		t.Errorf("Precinct area mismatch: %d vs %d", actualArea, expectedArea)
	}
}
