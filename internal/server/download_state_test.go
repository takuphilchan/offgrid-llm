package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDownloadHistoryMigrationBacksUpAndPreservesPartialBytes(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"original.gguf":{"file_name":"original.gguf","status":"downloading","repository":"owner/repo","source_file":"sub/weights.gguf"}}`)
	if err := os.WriteFile(filepath.Join(dir, "downloads.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "original.gguf.tmp"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{DataDir: dir, ModelsDir: dir}}
	if err := s.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "downloads-v0-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatal("backup missing", backups, err)
	}
	b, _ := os.ReadFile(backups[0])
	if !bytes.Equal(b, legacy) {
		t.Fatal("original changed")
	}
	partial, _ := os.ReadFile(filepath.Join(dir, "original.gguf.tmp"))
	if string(partial) != "partial" {
		t.Fatal("partial bytes changed")
	}
	var snapshot downloadSnapshot
	b, _ = os.ReadFile(filepath.Join(dir, "downloads.json"))
	if json.Unmarshal(b, &snapshot) != nil || snapshot.SchemaVersion != 1 || snapshot.Legacy["original.gguf"].BytesDone != 7 {
		t.Fatal("bad migration")
	}
	// No dual write and no second migration on restart.
	if err = s.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(backups[0])
	if !bytes.Equal(b, legacy) {
		t.Fatal("backup was rewritten")
	}
}

func TestDownloadMigrationRetriesAfterPersistenceFailure(t *testing.T) {
	for _, failName := range []string{"downloads-migration.json", "downloads.json"} {
		t.Run(failName, func(t *testing.T) {
			dir := t.TempDir()
			original := []byte(`{}`)
			if err := os.WriteFile(filepath.Join(dir, "downloads.json"), original, 0600); err != nil {
				t.Fatal(err)
			}
			s := &Server{config: &config.Config{DataDir: dir, ModelsDir: dir}, downloadStateWriter: func(p string, v any) error {
				if filepath.Base(p) == failName {
					return syscall.ENOSPC
				}
				return storage.WriteJSON(p, v)
			}}
			if err := s.loadDownloads(); !errors.Is(err, syscall.ENOSPC) {
				t.Fatal("did not report full disk", err)
			}
			b, _ := os.ReadFile(filepath.Join(dir, "downloads.json"))
			if !bytes.Equal(b, original) {
				t.Fatal("failed migration replaced original")
			}
			s.downloadStateWriter = nil
			if err := s.loadDownloads(); err != nil {
				t.Fatal("cannot recover", err)
			}
		})
	}
}

func TestDownloadHistoryRejectsCorruptionAndUnknownVersionsWithoutWrites(t *testing.T) {
	for _, data := range []string{`null`, `{`, `{"schema_version":99,"legacy":{}}`, `{"schema_version":1,"legacy":{},"unexpected":true}`, `{"bad.gguf":null}`, `{"../outside":{"file_name":"../outside"}}`, `{"schema_version":1,"operations":{"bad":null}}`} {
		t.Run(data, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "downloads.json")
			os.WriteFile(p, []byte(data), 0600)
			s := &Server{config: &config.Config{DataDir: dir, ModelsDir: dir}}
			if err := s.loadDownloads(); err == nil {
				t.Fatal("invalid history accepted")
			}
			b, _ := os.ReadFile(p)
			if string(b) != data {
				t.Fatal("invalid history overwritten")
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 1 {
				t.Fatal("migration wrote before validation")
			}
		})
	}
}

func TestDownloadStateRetainsRetryIdentityAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := &Server{config: &config.Config{DataDir: dir, ModelsDir: dir}, downloadProgress: map[string]*DownloadProgress{"external.gguf": {FileName: "external.gguf", ModelID: "external", SourceFile: "path/weights.gguf", Repository: "author/repository", Status: "downloading", EnableKnowledge: true}}, downloadCancelFuncs: map[string]context.CancelFunc{}}
	if err := s.saveDownloadsLocked(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "external.gguf.tmp"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	recovered := &Server{config: s.config}
	if err := recovered.loadDownloads(); err != nil {
		t.Fatal(err)
	}
	item := recovered.downloadProgress["external.gguf"]
	if item.Status != "failed" || item.SourceFile != "path/weights.gguf" || item.Repository != "author/repository" || item.BytesDone != 7 || !item.EnableKnowledge {
		t.Fatal(item)
	}
}
