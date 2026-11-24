package storage

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func TestBackupZIPDirectoryBounds(t *testing.T) {
	fixture := makeBackupFixture(t, "state.txt", []byte("data"), nil)
	good, _ := os.ReadFile(fixture)
	if err := validateBackupZIPDirectory(bytes.NewReader(good), int64(len(good))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"incorrect count", func(b []byte) {
			binary.LittleEndian.PutUint16(b[len(b)-14:], 3)
			binary.LittleEndian.PutUint16(b[len(b)-12:], 3)
		}},
		{"oversized index", func(b []byte) { binary.LittleEndian.PutUint32(b[len(b)-10:], uint32(maxBackupDirectoryBytes+1)) }},
		{"offset overflow", func(b []byte) { binary.LittleEndian.PutUint32(b[len(b)-6:], 0xfffffffe) }},
		{"multiple disks", func(b []byte) { binary.LittleEndian.PutUint16(b[len(b)-18:], 1) }},
		{"missing ZIP64", func(b []byte) {
			binary.LittleEndian.PutUint16(b[len(b)-14:], 65535)
			binary.LittleEndian.PutUint16(b[len(b)-12:], 65535)
		}},
		{"extra central record", func(b []byte) {
			binary.LittleEndian.PutUint16(b[len(b)-14:], 1)
			binary.LittleEndian.PutUint16(b[len(b)-12:], 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := append([]byte(nil), good...)
			tc.mutate(bad)
			if err := validateBackupZIPDirectory(bytes.NewReader(bad), int64(len(bad))); err == nil {
				t.Fatal("invalid bounds accepted")
			}
		})
	}
	if err := validateBackupZIPDirectory(bytes.NewReader(good), maxBackupArchiveBytes+1); err == nil {
		t.Fatal("archive size overflow accepted")
	}
}

func TestBackupZIP64DirectoryPreflight(t *testing.T) {
	// Force the standard writer's real ZIP64 count path without large file data.
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for i := 0; i < 65536; i++ {
		if _, err := w.CreateHeader(&zip.FileHeader{Name: "f", Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupZIPDirectory(bytes.NewReader(data.Bytes()), int64(data.Len())); err != nil {
		t.Fatal("valid bounded ZIP64 rejected", err)
	}
	// Preflight counts actual records as well as the untrusted ZIP64 total.
	bad := append([]byte(nil), data.Bytes()...)
	locator := len(bad) - 22 - 20
	offset := binary.LittleEndian.Uint64(bad[locator+8 : locator+16])
	binary.LittleEndian.PutUint64(bad[offset+24:offset+32], maxBackupFiles+2)
	binary.LittleEndian.PutUint64(bad[offset+32:offset+40], maxBackupFiles+2)
	if err := validateBackupZIPDirectory(bytes.NewReader(bad), int64(len(bad))); err == nil {
		t.Fatal("excessive ZIP64 count accepted")
	}
}
