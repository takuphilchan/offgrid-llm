//go:build !windows

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestEndpointConfigurationReplacementCannotBlockOnFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "endpoints.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"endpoints":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
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
		file, err := openObservedInferenceEndpointFile(path, before)
		if file != nil {
			file.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("replaced FIFO accepted as configuration")
		}
	case <-time.After(time.Second):
		// Unblock a regressed opener so the test does not leak a goroutine.
		fd, _ := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if fd >= 0 {
			unix.Close(fd)
		}
		t.Fatal("configuration opener blocked before its type check")
	}
}
