package htj2k

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestVLCTablesMatchOpenJPH(t *testing.T) {
	tbl0 := parseOpenJPHTable(t, "table0.h")
	tbl1 := parseOpenJPHTable(t, "table1.h")

	if len(tbl0) != len(VLCTbl0) {
		t.Fatalf("table0 entry count mismatch: openjph=%d go=%d", len(tbl0), len(VLCTbl0))
	}
	for i := range tbl0 {
		if tbl0[i] != VLCTbl0[i] {
			t.Fatalf("table0[%d] mismatch: openjph=%+v go=%+v", i, tbl0[i], VLCTbl0[i])
		}
	}

	if len(tbl1) != len(VLCTbl1) {
		t.Fatalf("table1 entry count mismatch: openjph=%d go=%d", len(tbl1), len(VLCTbl1))
	}
	for i := range tbl1 {
		if tbl1[i] != VLCTbl1[i] {
			t.Fatalf("table1[%d] mismatch: openjph=%+v go=%+v", i, tbl1[i], VLCTbl1[i])
		}
	}
}

func parseOpenJPHTable(t *testing.T, filename string) []VLCEntry {
	t.Helper()
	path := openJPHTablePath(t, filename)

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	pattern := regexp.MustCompile(`0x[0-9A-Fa-f]+|\d+`)
	entries := make([]VLCEntry, 0, 512)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		nums := pattern.FindAllString(line, -1)
		if len(nums) != 7 {
			continue
		}
		vals := make([]uint8, 7)
		for i, n := range nums {
			vals[i] = parseUint8(t, n)
		}
		entries = append(entries, VLCEntry{
			CQ:     vals[0],
			Rho:    vals[1],
			UOff:   vals[2],
			EK:     vals[3],
			E1:     vals[4],
			Cwd:    vals[5],
			CwdLen: vals[6],
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}

	return entries
}

func parseUint8(t *testing.T, value string) uint8 {
	t.Helper()
	parsed, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		t.Fatalf("parse number %q: %v", value, err)
	}
	if parsed < 0 || parsed > 0xFF {
		t.Fatalf("number out of uint8 range: %q", value)
	}
	return uint8(parsed)
}

func openJPHTablePath(t *testing.T, filename string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(file)
	return filepath.Join(pkgDir, "..", "..", "fo-dicom-codec-code", "Native", "Common", "OpenJPH", "coding", filename)
}
