//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkspaceKeyFileReplacementCannotBlockOnFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity")
	if err := os.WriteFile(path, []byte("isolated original key"), 0600); err != nil {
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
		file, err := openWorkspaceKeyFile(path, before)
		if file != nil {
			file.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("replacement FIFO accepted as a key file")
		}
	case <-time.After(time.Second):
		// Release a blocking-open regression before failing; all paths are owned
		// by this disposable test and no key contents are exposed.
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0600)
		if err == nil {
			unix.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("replacement FIFO blocked key-file opening")
	}
	if _, err := readWorkspaceKeyFile(path); err == nil {
		t.Fatal("static FIFO accepted as a key file")
	}
}
