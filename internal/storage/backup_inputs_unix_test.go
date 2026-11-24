//go:build linux || darwin

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestBackupArchiveFIFOIsRejectedWithoutBlocking(t *testing.T) {
	for _, replace := range []bool{false, true} {
		root := t.TempDir()
		path := filepath.Join(root, "archive")
		ctx := context.Background()
		if replace {
			if err := os.WriteFile(path, make([]byte, 40), 0600); err != nil {
				t.Fatal(err)
			}
			ctx = context.WithValue(ctx, backupBoundaryKey{}, backupBoundaryHook(func(stage, name string) error {
				if stage != "archive_source_observed" {
					return nil
				}
				if err := os.Remove(name); err != nil {
					return err
				}
				return unix.Mkfifo(name, 0600)
			}))
		} else if err := unix.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := VerifyBackup(ctx, path); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("FIFO accepted")
			}
		case <-time.After(time.Second):
			// Unblock an old blocking-open regression before failing the test.
			fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0600)
			if err == nil {
				unix.Close(fd)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			t.Fatal("archive input blocked on FIFO")
		}
	}
}

func TestBackupFollowUpRejectsReplacedWorkspaceRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage != "backup_published" {
			return nil
		}
		if err := os.Rename(root, root+".original"); err != nil {
			return err
		}
		return os.Mkdir(root, 0700)
	}))
	called := false
	manifest, err := BackupWorkspaceThen(ctx, root, filepath.Join(parent, "backup.zip"), "v1", BackupWriteOptions{}, func(*BackupManifest) error { called = true; return nil })
	if called || err == nil || manifest == nil {
		t.Fatal("follow-up entered replacement workspace", called, err)
	}
}

func TestBackupSourceFIFOSubstitutionDoesNotBlock(t *testing.T) {
	root, output := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("regular source"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage != "backup_source_observed" {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		return unix.Mkfifo(path, 0600)
	}))
	done := make(chan error, 1)
	go func() {
		_, err := BackupWorkspace(ctx, root, filepath.Join(output, "backup.zip"), "v1")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO source accepted")
		}
	case <-time.After(time.Second):
		fd, err := unix.Open(source, unix.O_WRONLY|unix.O_NONBLOCK, 0600)
		if err == nil {
			unix.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("source substitution blocked archive creation")
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected source left an archive", entries, err)
	}
}
