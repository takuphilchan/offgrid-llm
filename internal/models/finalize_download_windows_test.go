package models

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFinalizeDownloadWindowsSharingLock(t *testing.T) {
	for _, mode := range []string{"released", "persistent", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "model.gguf")
			file, err := os.Create(path + ".tmp")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			_, _ = file.WriteString("downloaded-model")
			// A real Windows handle without FILE_SHARE_DELETE reproduces the
			// reported failure; mocks on Linux cannot establish this behavior.
			if err := os.Rename(path+".tmp", path); !isDownloadSharingError(err) {
				t.Fatalf("expected sharing violation, got %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "released" {
				time.AfterFunc(150*time.Millisecond, func() { _ = file.Close() })
			}
			if mode == "cancelled" {
				time.AfterFunc(50*time.Millisecond, cancel)
			}
			err = renameDownload(ctx, path+".tmp", path, os.Rename, 300*time.Millisecond)
			if mode == "released" {
				if err != nil {
					t.Fatal(err)
				}
				assertDownloadedContent(t, filepath.Dir(path), filepath.Base(path), []byte("downloaded-model"))
			} else {
				if mode == "persistent" && !errors.Is(err, ErrFinalizeDownload) {
					t.Fatalf("error=%v", err)
				}
				if mode == "cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("error=%v", err)
				}
				if _, err := os.Stat(path + ".tmp"); err != nil {
					t.Fatal("lost downloaded bytes", err)
				}
				_ = file.Close()
				if err := finalizeDownload(context.Background(), path+".tmp", path); err != nil {
					t.Fatal("could not retry promotion", err)
				}
			}
		})
	}
}
