package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const downloadSnapshotVersion = 1
const maxDownloadSnapshotBytes = 64 << 20

type downloadSnapshot struct {
	SchemaVersion int                          `json:"schema_version"`
	Legacy        map[string]*DownloadProgress `json:"legacy"`
	Operations    map[string]*ModelOperation   `json:"operations"`
}

func (s *Server) writeDownloadState(path string, value any) error {
	if s.downloadStateWriter != nil {
		return s.downloadStateWriter(path, value)
	}
	return storage.WriteJSON(path, value)
}

// Persist identity and retry metadata, not only the current progress bar.
func (s *Server) saveDownloadsLocked() error {
	if s.config == nil || s.config.DataDir == "" {
		return nil
	}
	snapshot := downloadSnapshot{SchemaVersion: downloadSnapshotVersion, Legacy: s.downloadProgress, Operations: s.modelOperations}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if len(b) > maxDownloadSnapshotBytes {
		return fmt.Errorf("model operation history exceeds storage limit")
	}
	return s.writeDownloadState(filepath.Join(s.config.DataDir, "downloads.json"), snapshot)
}

func (s *Server) loadDownloads() error {
	path := filepath.Join(s.config.DataDir, "downloads.json")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxDownloadSnapshotBytes+1))
	closeErr := f.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}
	if len(data) > maxDownloadSnapshotBytes {
		return fmt.Errorf("download history exceeds storage limit")
	}
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(data, &envelope); err != nil || envelope == nil {
		return fmt.Errorf("invalid download history JSON")
	}
	var progress map[string]*DownloadProgress
	operations := map[string]*ModelOperation{}
	legacy := envelope["schema_version"] == nil
	if legacy {
		if err = json.Unmarshal(data, &progress); err != nil {
			return fmt.Errorf("invalid legacy download history: %w", err)
		}
	} else {
		var snapshot downloadSnapshot
		d := json.NewDecoder(bytes.NewReader(data))
		d.DisallowUnknownFields()
		if err = d.Decode(&snapshot); err != nil || snapshot.SchemaVersion != downloadSnapshotVersion {
			return fmt.Errorf("unsupported or corrupt model operation history; restore a compatible backup")
		}
		progress, operations = snapshot.Legacy, snapshot.Operations
	}
	bindings := map[string]bool{}
	for key, item := range operations {
		if key != itemID(item) {
			return fmt.Errorf("invalid model operation key")
		}
		if err := item.validate(); err != nil {
			return fmt.Errorf("invalid model operation history: %w", err)
		}
		binding := item.ActorID + "\x00" + item.RequestID
		if bindings[binding] {
			return fmt.Errorf("duplicate model operation request binding")
		}
		bindings[binding] = true
	}
	for key, item := range progress {
		if item == nil || key != item.FileName || key == "." || key == ".." || !isSafeModelID(strings.TrimSuffix(key, ".gguf")) {
			return fmt.Errorf("invalid download history identity")
		}
	}
	if legacy {
		// Validate everything before writing; preserve an exact, synced original.
		// Retrying a failed migration verifies the same digest-addressed backup.
		hash := sha256.Sum256(data)
		backupName := fmt.Sprintf("downloads-v0-%x.json", hash[:12])
		backup := filepath.Join(s.config.DataDir, backupName)
		original, readErr := os.ReadFile(backup)
		if readErr == nil {
			if !bytes.Equal(original, data) {
				return fmt.Errorf("legacy download backup does not match; preserve both files and recover manually")
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		} else {
			// Publish only a complete, synced backup. A crash while writing leaves
			// a staging file, never a poisoned final backup that blocks retry.
			out, err := os.CreateTemp(s.config.DataDir, ".downloads-backup-*")
			if err != nil {
				return err
			}
			defer os.Remove(out.Name())
			_, writeErr := out.Write(data)
			syncErr := out.Sync()
			closeErr := out.Close()
			if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
				return err
			}
			if err = os.Rename(out.Name(), backup); err != nil {
				return err
			}
		}
		if err = s.writeDownloadState(filepath.Join(s.config.DataDir, "downloads-migration.json"), map[string]any{"source_schema": 0, "destination_schema": downloadSnapshotVersion, "backup": backupName, "sha256": fmt.Sprintf("%x", hash), "source_bytes": len(data)}); err != nil {
			return err
		}
	}
	for key, item := range progress {
		if item.Status == "downloading" || item.Status == "finalizing" {
			item.Status = "failed"
			item.Speed = 0
			item.Error = "Service stopped. Downloaded data is retained; choose Resume to continue."
			if info, err := os.Stat(filepath.Join(s.config.ModelsDir, key) + ".tmp"); err == nil {
				item.BytesDone = info.Size()
			}
		}
	}
	for _, item := range operations {
		if item.active() {
			item.State = "interrupted"
			item.ErrorCode = "service_restarted"
			item.Message = "Transfer stopped when the service restarted. Inspect installed state, then explicitly Resume; no download restarted automatically."
		}
	}
	s.downloadProgress = progress
	s.modelOperations = operations
	if s.downloadProgress == nil {
		s.downloadProgress = map[string]*DownloadProgress{}
	}
	if err := s.recoverModelOperations(); err != nil {
		return err
	}
	return s.saveDownloadsLocked()
}

func itemID(item *ModelOperation) string {
	if item == nil {
		return ""
	}
	return item.ID
}
