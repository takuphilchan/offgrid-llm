package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// Persist identity and retry metadata, not only the current progress bar.
func (s *Server) saveDownloadsLocked() error {
	if s.config == nil || s.config.DataDir == "" {
		return nil
	}
	return storage.WriteJSON(filepath.Join(s.config.DataDir, "downloads.json"), s.downloadProgress)
}

func (s *Server) loadDownloads() error {
	path := filepath.Join(s.config.DataDir, "downloads.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var progress map[string]*DownloadProgress
	if err = json.Unmarshal(data, &progress); err != nil {
		return fmt.Errorf("invalid download history: %w", err)
	}
	for key, item := range progress {
		if item == nil || key != item.FileName || !isSafeModelID(strings.TrimSuffix(key, ".gguf")) {
			return fmt.Errorf("invalid download history identity")
		}
		if item.Status == "downloading" || item.Status == "finalizing" {
			item.Status = "failed"
			item.Speed = 0
			item.Error = "Service stopped. Downloaded data is retained; choose Resume to continue."
			if info, err := os.Stat(filepath.Join(s.config.ModelsDir, key) + ".tmp"); err == nil {
				item.BytesDone = info.Size()
			}
		}
	}
	s.downloadProgress = progress
	if s.downloadProgress == nil {
		s.downloadProgress = map[string]*DownloadProgress{}
	}
	return s.saveDownloadsLocked()
}
