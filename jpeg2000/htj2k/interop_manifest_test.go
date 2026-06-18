package htj2k

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/codestream"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
)

type interopManifest struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Source        string                   `json:"source"`
	Notes         []string                 `json:"notes"`
	Fixtures      []interopManifestFixture `json:"fixtures"`
}

type interopManifestFixture struct {
	Name          string                           `json:"name"`
	Width         int                              `json:"width"`
	Height        int                              `json:"height"`
	Components    int                              `json:"components"`
	BitsAllocated int                              `json:"bitsAllocated"`
	BitsStored    int                              `json:"bitsStored"`
	Signed        bool                             `json:"signed"`
	SampleLayout  string                           `json:"sampleLayout"`
	ByteOrder     string                           `json:"byteOrder"`
	InputRaw      string                           `json:"inputRaw"`
	Codestreams   map[string]interopCodestreamFile `json:"codestreams"`
}

type interopCodestreamFile struct {
	Path             string `json:"path"`
	ReferenceEncoder string `json:"referenceEncoder"`
	Lossless         bool   `json:"lossless"`
	ProgressionOrder string `json:"progressionOrder"`
}

func TestHTJ2KInteropManifestDecodesReferenceCodestreams(t *testing.T) {
	dir := htj2kInteropFixtureDir()
	manifest := readInteropManifest(t, dir)
	if len(manifest.Fixtures) == 0 {
		t.Fatal("interop manifest must contain at least one fixture")
	}

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			raw := readInteropFile(t, dir, fixture.InputRaw)

			for codestreamName, codestreamFile := range fixture.Codestreams {
				codestreamName, codestreamFile := codestreamName, codestreamFile
				if !codestreamFile.Lossless {
					continue
				}
				t.Run(codestreamName, func(t *testing.T) {
					encoded := readInteropFile(t, dir, codestreamFile.Path)
					got := decodeHTJ2KCodestream(t, encoded)
					if len(got) != expectedRawLength(fixture) {
						t.Fatalf("decoded raw length = %d, want %d", len(got), expectedRawLength(fixture))
					}
					if !bytes.Equal(got, raw) {
						first := firstByteDiff(got, raw)
						t.Fatalf("decoded reference codestream differs at byte %d: got=%d want=%d", first, got[first], raw[first])
					}
				})
			}
		})
	}
}

func TestHTJ2KInteropManifestCodestreamProfiles(t *testing.T) {
	dir := htj2kInteropFixtureDir()
	manifest := readInteropManifest(t, dir)

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		for codestreamName, codestreamFile := range fixture.Codestreams {
			codestreamName, codestreamFile := codestreamName, codestreamFile
			t.Run(fixture.Name+"/"+codestreamName, func(t *testing.T) {
				encoded := readInteropFile(t, dir, codestreamFile.Path)
				cs, err := codestream.NewParser(encoded).Parse()
				if err != nil {
					t.Fatalf("parse reference codestream: %v", err)
				}
				assertHTJ2KProfile(t, fixture, codestreamFile, cs, encoded)
			})
		}
	}
}

func readInteropManifest(t *testing.T, dir string) interopManifest {
	t.Helper()
	data := readInteropFile(t, dir, "manifest.json")
	var manifest interopManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse interop manifest: %v", err)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("interop manifest schemaVersion = %d, want 1", manifest.SchemaVersion)
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.SampleLayout != "interleaved" {
			t.Fatalf("fixture %q sampleLayout = %q, want interleaved", fixture.Name, fixture.SampleLayout)
		}
		if fixture.BitsAllocated != 8 && fixture.BitsAllocated != 16 {
			t.Fatalf("fixture %q bitsAllocated = %d, want 8 or 16", fixture.Name, fixture.BitsAllocated)
		}
		if fixture.BitsStored != fixture.BitsAllocated {
			t.Fatalf("fixture %q bitsStored = %d, want bitsAllocated %d for image-level HTJ2K fixture",
				fixture.Name, fixture.BitsStored, fixture.BitsAllocated)
		}
		if fixture.BitsAllocated == 16 && fixture.ByteOrder != "little-endian" {
			t.Fatalf("fixture %q byteOrder = %q, want little-endian for 16-bit raw samples", fixture.Name, fixture.ByteOrder)
		}
		if fixture.BitsAllocated == 8 && fixture.ByteOrder != "none" {
			t.Fatalf("fixture %q byteOrder = %q, want none for 8-bit raw samples", fixture.Name, fixture.ByteOrder)
		}
	}
	return manifest
}

func readInteropFile(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read interop file %q: %v", name, err)
	}
	return data
}

func decodeHTJ2KCodestream(t *testing.T, encoded []byte) []byte {
	t.Helper()
	decoder := jpeg2000.NewDecoder()
	decoder.SetBlockDecoderFactory(func(width, height int, _ int) t2.BlockDecoder {
		return NewHTDecoder(width, height)
	})
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("decode HTJ2K codestream: %v", err)
	}
	return decoder.GetPixelData()
}

func assertHTJ2KProfile(t *testing.T, fixture interopManifestFixture, stream interopCodestreamFile, cs *codestream.Codestream, encoded []byte) {
	t.Helper()
	if cs.SIZ == nil {
		t.Fatal("missing SIZ segment")
	}
	if cs.COD == nil {
		t.Fatal("missing COD segment")
	}
	if got := int(cs.SIZ.Xsiz - cs.SIZ.XOsiz); got != fixture.Width {
		t.Fatalf("SIZ width = %d, want %d", got, fixture.Width)
	}
	if got := int(cs.SIZ.Ysiz - cs.SIZ.YOsiz); got != fixture.Height {
		t.Fatalf("SIZ height = %d, want %d", got, fixture.Height)
	}
	if got := int(cs.SIZ.Csiz); got != fixture.Components {
		t.Fatalf("SIZ components = %d, want %d", got, fixture.Components)
	}
	for i, component := range cs.SIZ.Components {
		if got := component.BitDepth(); got != fixture.BitsStored {
			t.Fatalf("SIZ component %d bit depth = %d, want %d", i, got, fixture.BitsStored)
		}
		if got := component.IsSigned(); got != fixture.Signed {
			t.Fatalf("SIZ component %d signed = %v, want %v", i, got, fixture.Signed)
		}
	}
	if cs.SIZ.Rsiz&0x4000 == 0 {
		t.Fatalf("SIZ.Rsiz must signal HTJ2K capability bit 14; got 0x%04X", cs.SIZ.Rsiz)
	}
	if !containsMarker(encoded, 0xFF50) {
		t.Fatal("HTJ2K reference codestream is missing CAP marker")
	}
	if cs.COD.Scod&0x40 != 0 {
		t.Fatalf("HTJ2K mode must not be signalled in Scod; got Scod=0x%02X", cs.COD.Scod)
	}
	if cs.COD.CodeBlockStyle&0x40 == 0 {
		t.Fatalf("HTJ2K mode must be signalled in code-block style; style=0x%02X", cs.COD.CodeBlockStyle)
	}
	if stream.ProgressionOrder == "RPCL" && cs.COD.ProgressionOrder != 2 {
		t.Fatalf("COD progression order = %d, want RPCL(2)", cs.COD.ProgressionOrder)
	}
	if stream.Lossless && cs.COD.Transformation != 1 {
		t.Fatalf("COD wavelet transform = %d, want reversible 5/3(1) for lossless fixture", cs.COD.Transformation)
	}
}

func expectedRawLength(fixture interopManifestFixture) int {
	return fixture.Width * fixture.Height * fixture.Components * ((fixture.BitsAllocated + 7) / 8)
}
