package storage

import (
	"io"
	"strings"
	"testing"
)

type measuredManifestReader struct {
	reader io.Reader
	bytes  int
}

func (r *measuredManifestReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytes += n
	return n, err
}

func TestBackupManifestInventoryIsBoundedBeforeAllocation(t *testing.T) {
	data := `{"files":[` + strings.Repeat(`{"path":"x"},`, 100000) + `{"path":"x"}]}`
	reader := &measuredManifestReader{reader: strings.NewReader(data)}
	if _, err := decodeBackupManifest(reader, 1); err == nil {
		t.Fatal("invented inventory accepted")
	}
	if reader.bytes > 4096 {
		t.Fatal("read a caller-sized inventory before enforcing entry count", reader.bytes)
	}
}

func TestBackupManifestStrictFieldsAndTrailingInputAreBounded(t *testing.T) {
	for _, data := range []string{`{"files":[],"files":[]}`, `{"files":[{"path":"x","unknown":true}]}`, `{"files":[],"unknown":[` + strings.Repeat(`{},`, 100000) + `{}]}`, `{"files":[]}[` + strings.Repeat(`{},`, 100000) + `{}]`} {
		reader := &measuredManifestReader{reader: strings.NewReader(data)}
		if _, err := decodeBackupManifest(reader, 1); err == nil {
			t.Fatal("invalid manifest accepted")
		}
		if reader.bytes > 4096 {
			t.Fatal("decoded unrelated oversized value before rejection", reader.bytes)
		}
	}
	for _, data := range []string{`{"files":[]}`, `{"files":null}`} {
		if manifest, err := decodeBackupManifest(strings.NewReader(data), 0); err != nil || len(manifest.Files) != 0 {
			t.Fatal("legacy empty inventory changed", err)
		}
	}
}
