package htj2k

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const defaultHTJ2KInteropFixtureDir = "test-data/htj2k/interop"

func TestDecodeFoDicomHTJ2KInteropFixtures(t *testing.T) {
	outDir := htj2kInteropFixtureDir()
	manifest := readInteropManifest(t, outDir)

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			raw := readInteropFile(t, outDir, fixture.InputRaw)
			for name, codestreamFile := range fixture.Codestreams {
				name, codestreamFile := name, codestreamFile
				if !codestreamFile.Lossless {
					continue
				}
				t.Run(name, func(t *testing.T) {
					codestream := readInteropFile(t, outDir, codestreamFile.Path)
					got := decodeHTJ2KCodestream(t, codestream)
					if first := firstByteDiff(got, raw); first >= 0 {
						t.Fatalf("decoded fo-dicom.Codecs fixture differs at byte %d: got=%d want=%d", first, got[first], raw[first])
					}
				})
			}
		})
	}
}

func htj2kInteropFixtureDir() string {
	if dir := os.Getenv("HTJ2K_INTEROP_FIXTURE_DIR"); dir != "" {
		return dir
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return defaultHTJ2KInteropFixtureDir
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", defaultHTJ2KInteropFixtureDir))
}

func firstByteDiff(a, b []byte) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return limit
	}
	return -1
}
