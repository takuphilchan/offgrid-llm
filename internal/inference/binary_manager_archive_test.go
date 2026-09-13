package inference

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLlamaArchivePathRejectsTraversal(t *testing.T) {
	stage := t.TempDir()
	for _, entry := range []string{"../escape", "llama-b10516/../../escape", "llama-b10516/../llama-server", "/absolute", "C:/outside", "other-root/lib.so"} {
		if _, err := llamaArchivePath(stage, "b10516", entry); err == nil {
			t.Errorf("accepted unsafe archive path %q", entry)
		}
	}
	for _, entry := range []string{"llama-b10516/llama-server", "llama-server"} {
		got, err := llamaArchivePath(stage, "b10516", entry)
		if err != nil || got != filepath.Join(stage, "llama-server") {
			t.Errorf("safe path %q resolved to %q, %v", entry, got, err)
		}
	}
}

func TestPinnedLlamaWindowsZipCanBeExtracted(t *testing.T) {
	archive := strings.TrimSpace(os.Getenv("OFFGRID_TEST_LLAMA_ZIP"))
	if archive == "" {
		t.Skip("set OFFGRID_TEST_LLAMA_ZIP to exercise the pinned Windows release artifact")
	}
	stage := t.TempDir()
	if err := extractLlamaArchive(archive, stage, "b10516", "llama-b10516-bin-win-cpu-x64.zip"); err != nil {
		t.Fatal(err)
	}
	if !regularFile(filepath.Join(stage, "llama-server.exe")) {
		t.Fatal("extracted archive does not contain llama-server.exe")
	}
}

func TestPinnedLlamaArchiveCanBeExtracted(t *testing.T) {
	archive := strings.TrimSpace(os.Getenv("OFFGRID_TEST_LLAMA_ARCHIVE"))
	if archive == "" {
		t.Skip("set OFFGRID_TEST_LLAMA_ARCHIVE to exercise the pinned release artifact")
	}
	stage := t.TempDir()
	if err := extractLlamaArchive(archive, stage, "b10516", "llama-b10516-bin-ubuntu-x64.tar.gz"); err != nil {
		t.Fatal(err)
	}
	server := filepath.Join(stage, "llama-server")
	if !regularFile(server) {
		t.Fatalf("extracted archive does not contain %s", server)
	}
	if !llamaServerSupportsRequiredFlags(server) {
		t.Fatal("extracted pinned llama-server lacks required flags")
	}
}
