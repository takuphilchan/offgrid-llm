package audio

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestTarGz(t *testing.T, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatalf("write contents: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return path
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	archivePath := writeTestTarGz(t, "../escaped.txt", []byte("unsafe"))
	destination := filepath.Join(t.TempDir(), "destination")
	if err := extractTarGz(archivePath, destination); err == nil {
		t.Fatal("extractTarGz accepted a traversal path")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(destination), "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("archive escaped destination: %v", err)
	}
}

func TestExtractTarGzAllowsNestedFiles(t *testing.T) {
	archivePath := writeTestTarGz(t, "bin/tool", []byte("safe"))
	destination := t.TempDir()
	if err := extractTarGz(archivePath, destination); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "bin", "tool"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(contents) != "safe" {
		t.Fatalf("contents = %q, want safe", contents)
	}
}
