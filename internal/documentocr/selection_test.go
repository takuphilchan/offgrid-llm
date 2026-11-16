package documentocr

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestOCRSelectionIsOrderedBoundedAndExplicit(t *testing.T) {
	for _, bad := range []string{"", "eng+", "+eng", "eng+eng", "eng+sn", "eng+fra+deu+spa", "ENG", "eng+../fra", "eng fra", "eng\x00"} {
		if _, ok := LookupSelection(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
	eng, _ := LookupLanguage("eng")
	single, ok := LookupSelection("eng")
	if !ok || single.SHA256 != eng.SHA256 {
		t.Fatal("changed single-pack identity")
	}
	first, ok := LookupSelection("chi_sim+fra+eng")
	second, _ := LookupSelection("eng+fra+chi_sim")
	if !ok || first.SHA256 == second.SHA256 || first.Qualification != "unverified" {
		t.Fatal("selection order or qualification lost")
	}
	for _, bad := range [][]byte{nil, []byte("OGM1"), make([]byte, 100), append([]byte("OGM1"), 255, 255, 255, 255)} {
		if ValidateModels("eng+fra", bad) {
			t.Fatal("accepted malformed bundle")
		}
	}
}

func TestActualMixedOCRBundlesRejectCorruptionAndSubstitution(t *testing.T) {
	dir := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if dir == "" {
		t.Skip("requires explicitly supplied pinned language packs")
	}
	data := [][]byte{}
	for _, id := range []string{"chi_sim", "fra", "eng"} {
		b, err := os.ReadFile(filepath.Join(dir, id+".traineddata"))
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, b)
	}
	encoded, err := Models("chi_sim+fra+eng", data)
	if err != nil || !ValidateModels("chi_sim+fra+eng", encoded) {
		t.Fatal(err)
	}
	if ValidateModels("fra+chi_sim+eng", encoded) {
		t.Fatal("accepted reordered packs")
	}
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:len(b)-1] },
		func(b []byte) []byte { return append(b, 0) },
		func(b []byte) []byte { b[len(b)-1] ^= 1; return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[12:16], 0xffffffff); return b },
		func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:12], 0xffffffff); return b },
		func(b []byte) []byte { b[16] = 'x'; return b },
	} {
		if ValidateModels("chi_sim+fra+eng", mutate(bytes.Clone(encoded))) {
			t.Fatal("accepted malformed bundle")
		}
	}
	if _, err := Models("eng+fra", data[:2]); err == nil {
		t.Fatal("accepted mismatched pins")
	}
	single, err := Models("eng", data[2:])
	if err != nil || !bytes.Equal(single, data[2]) {
		t.Fatal("single-pack compatibility", err)
	}
}
