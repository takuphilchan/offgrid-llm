package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscardDownloadAdmissionAndPersistence(t *testing.T) {
	for _, status := range []string{"downloading", "finalizing", "complete", "cancelled", "interrupted", "failed"} {
		t.Run(status, func(t *testing.T) {
			dir := t.TempDir()
			file := "model.gguf"
			s := &Server{config: &config.Config{DataDir: dir, ModelsDir: dir}, downloadProgress: map[string]*DownloadProgress{file: {FileName: file, Status: status}}}
			os.WriteFile(filepath.Join(dir, file+".tmp"), []byte("partial"), 0600)
			os.WriteFile(filepath.Join(dir, file), []byte("installed"), 0600)
			call := func() int {
				w := httptest.NewRecorder()
				s.handleDiscardDownload(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"file_name":"model.gguf"}`)))
				return w.Code
			}
			allowed := status == "cancelled" || status == "interrupted" || status == "failed"
			if !allowed {
				if call() != 409 {
					t.Fatal("active/complete admitted")
				}
				return
			}
			s.downloadCancelFuncs = map[string]context.CancelFunc{file: func() {}}
			if call() != 409 {
				t.Fatal("worker still admitted")
			}
			delete(s.downloadCancelFuncs, file)
			s.downloadStateWriter = func(string, any) error { return errors.New("disk full") }
			if call() != 500 || s.downloadProgress[file] == nil {
				t.Fatal("failed persistence lost record")
			}
			s.downloadStateWriter = nil
			if call() != 200 || s.downloadProgress[file] != nil {
				t.Fatal("retry failed")
			}
			data, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil || string(data) != "installed" {
				t.Fatal("installed model changed")
			}
			var saved downloadSnapshot
			data, err = os.ReadFile(filepath.Join(dir, "downloads.json"))
			if err != nil || json.Unmarshal(data, &saved) != nil || saved.Legacy[file] != nil {
				t.Fatal("history not durable")
			}
			if call() != 200 {
				t.Fatal("repeat not idempotent")
			}
		})
	}
}
func TestDiscardDownloadRejectsMalformedRequests(t *testing.T) {
	s := &Server{}
	for _, body := range []string{`{}`, `{"file_name":"../model.gguf"}`, `{"file_name":"model.gguf","extra":1}`, `{"file_name":"model.gguf"} {}`} {
		w := httptest.NewRecorder()
		s.handleDiscardDownload(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
}
