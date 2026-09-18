package server

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"os"
	"path/filepath"
	"testing"
)

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
