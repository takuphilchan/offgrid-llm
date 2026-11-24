//go:build linux || darwin

package execution

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkingCopyOpenRejectsRacedFIFOWithoutWriter(t *testing.T) {
	directory := t.TempDir()
	name := filepath.Join(directory, "input")
	if err := os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	expected, err := root.Lstat("input")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if err = unix.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		file, err := openWorkingCopyFile(root, "input", expected)
		if file != nil {
			file.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("substituted FIFO was accepted")
		}
	case <-time.After(250 * time.Millisecond):
		// Unblock the pre-fix implementation before reporting the regression;
		// every handle and path remains owned by this disposable fixture.
		fd, openErr := unix.Open(name, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if openErr == nil {
			unix.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("fixture FIFO opener did not settle")
		}
		t.Fatal("worker file open waited for a FIFO writer after regular-file inspection")
	}
}
