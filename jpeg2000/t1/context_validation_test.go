package t1

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestOpenJPEGLUTAlignment(t *testing.T) {
	zc := parseOpenJPEGLUT(t, "lut_ctxno_zc", 2048)
	sc := parseOpenJPEGLUT(t, "lut_ctxno_sc", 256)
	spb := parseOpenJPEGLUT(t, "lut_spb", 256)

	for i, v := range zc {
		if lut_ctxno_zc[i] != uint8(v) {
			t.Fatalf("lut_ctxno_zc[%d] mismatch: got %d want %d", i, lut_ctxno_zc[i], v)
		}
	}
	for i, v := range sc {
		if lut_ctxno_sc[i] != uint8(v) {
			t.Fatalf("lut_ctxno_sc[%d] mismatch: got 0x%X want 0x%X", i, lut_ctxno_sc[i], v)
		}
	}
	for i, v := range spb {
		if lut_spb[i] != v {
			t.Fatalf("lut_spb[%d] mismatch: got %d want %d", i, lut_spb[i], v)
		}
	}
}

func TestZeroCodingContextLogic(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{name: "NoNeighbors", flags: 0, expected: 0},
		{name: "WestOnly", flags: T1_SIG_W, expected: 5},
		{name: "NorthOnly", flags: T1_SIG_N, expected: 3},
		{name: "WestEast", flags: T1_SIG_W | T1_SIG_E, expected: 8},
		{name: "NorthSouth", flags: T1_SIG_N | T1_SIG_S, expected: 4},
		{name: "CardinalFour", flags: T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E, expected: 8},
		{
			name:     "AllNeighbors",
			flags:    T1_SIG_N | T1_SIG_S | T1_SIG_W | T1_SIG_E | T1_SIG_NW | T1_SIG_NE | T1_SIG_SW | T1_SIG_SE,
			expected: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getZeroCodingContext(tc.flags, 0); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestMagnitudeRefinementContext(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{name: "NoNeighbors", flags: 0, expected: 14},
		{name: "OneNeighbor", flags: T1_SIG_W, expected: 15},
		{name: "TwoNeighbors", flags: T1_SIG_W | T1_SIG_E, expected: 15},
		{name: "ThreeNeighbors", flags: T1_SIG_W | T1_SIG_E | T1_SIG_N, expected: 16},
		{name: "FourNeighbors", flags: T1_SIG_W | T1_SIG_E | T1_SIG_N | T1_SIG_S, expected: 16},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getMagRefinementContext(tc.flags); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestSignCodingContextExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{
			name:     "AllPositive",
			flags:    T1_SIG_E | T1_SIG_W | T1_SIG_N | T1_SIG_S,
			expected: 0xD,
		},
		{
			name:     "EastWestNegative",
			flags:    T1_SIG_E | T1_SIG_W | T1_SIGN_E | T1_SIGN_W,
			expected: 0xC,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getSignCodingContext(tc.flags); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestContextConstants(t *testing.T) {
	tests := []struct {
		name  string
		start int
		end   int
		count int
	}{
		{name: "ZeroCoding", start: CTX_ZC_START, end: CTX_ZC_END, count: 9},
		{name: "SignCoding", start: CTX_SC_START, end: CTX_SC_END, count: 5},
		{name: "MagnitudeRefinement", start: CTX_MR_START, end: CTX_MR_END, count: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.end-tt.start+1; got != tt.count {
				t.Fatalf("context count mismatch: got %d want %d", got, tt.count)
			}
		})
	}

	if NUM_CONTEXTS != 19 {
		t.Fatalf("NUM_CONTEXTS = %d, want 19", NUM_CONTEXTS)
	}
	if CTX_RL != 17 {
		t.Fatalf("CTX_RL = %d, want 17", CTX_RL)
	}
	if CTX_UNI != 18 {
		t.Fatalf("CTX_UNI = %d, want 18", CTX_UNI)
	}
}

func TestStateFlagDefinitions(t *testing.T) {
	if T1_SIG != 0x0001 {
		t.Fatalf("T1_SIG = 0x%04X, want 0x0001", T1_SIG)
	}
	if T1_REFINE != 0x0002 {
		t.Fatalf("T1_REFINE = 0x%04X, want 0x0002", T1_REFINE)
	}
	if T1_VISIT != 0x0004 {
		t.Fatalf("T1_VISIT = 0x%04X, want 0x0004", T1_VISIT)
	}

	expected := []struct {
		flag uint32
		name string
	}{
		{T1_SIG_N, "T1_SIG_N"},
		{T1_SIG_S, "T1_SIG_S"},
		{T1_SIG_W, "T1_SIG_W"},
		{T1_SIG_E, "T1_SIG_E"},
		{T1_SIG_NW, "T1_SIG_NW"},
		{T1_SIG_NE, "T1_SIG_NE"},
		{T1_SIG_SW, "T1_SIG_SW"},
		{T1_SIG_SE, "T1_SIG_SE"},
	}

	var mask uint32
	for _, n := range expected {
		if n.flag == 0 {
			t.Fatalf("%s flag is zero", n.name)
		}
		if n.flag&(n.flag-1) != 0 {
			t.Fatalf("%s flag is not a single bit: 0x%X", n.name, n.flag)
		}
		mask |= n.flag
	}

	if T1_SIG_NEIGHBORS != mask {
		t.Fatalf("T1_SIG_NEIGHBORS = 0x%X, want 0x%X", T1_SIG_NEIGHBORS, mask)
	}
}

func parseOpenJPEGLUT(t *testing.T, name string, expectedLen int) []int {
	t.Helper()
	source := readOpenJPEGFile(t, "t1_luts.h")
	start := strings.Index(source, name)
	if start == -1 {
		t.Fatalf("OpenJPEG %s not found", name)
	}
	section := source[start:]
	open := strings.Index(section, "{")
	if open == -1 {
		t.Fatalf("OpenJPEG %s opening brace not found", name)
	}
	close := strings.Index(section, "};")
	if close == -1 {
		t.Fatalf("OpenJPEG %s closing brace not found", name)
	}
	section = section[open+1 : close]

	re := regexp.MustCompile(`0x[0-9a-fA-F]+|\d+`)
	matches := re.FindAllString(section, -1)
	values := make([]int, 0, len(matches))
	for _, match := range matches {
		value, err := strconv.ParseInt(match, 0, 32)
		if err != nil {
			t.Fatalf("invalid %s value %q: %v", name, match, err)
		}
		values = append(values, int(value))
	}
	if expectedLen > 0 && len(values) != expectedLen {
		t.Fatalf("OpenJPEG %s length = %d, want %d", name, len(values), expectedLen)
	}
	return values
}

func readOpenJPEGFile(t *testing.T, name string) string {
	t.Helper()
	root := findRepoRoot(t)
	path := filepath.Join(root, "fo-dicom-codec-code", "Native", "Common", "OpenJPEG", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenJPEG file %s: %v", path, err)
	}
	return string(data)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file location")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found while locating repo root")
	return ""
}
