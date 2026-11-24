package inference

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestONNXConfigOpenerDoesNotBlockOnFIFOReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		file, err := openONNXRuntimeConfigFile(path)
		if file != nil {
			info, statErr := file.Stat()
			file.Close()
			if statErr != nil {
				err = statErr
			} else if info.Mode().IsRegular() {
				t.Error("FIFO became regular")
			}
		}
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lower ONNX config opener blocked on FIFO")
	}
	if _, err := LoadONNXEmbeddingRuntimeConfig(path); err == nil {
		t.Fatal("FIFO accepted as config")
	}
}
