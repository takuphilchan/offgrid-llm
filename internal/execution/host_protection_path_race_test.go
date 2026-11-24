package execution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProtectionDirectoryAppearsBetweenLocationProbes(t *testing.T) {
	parent := protectionTestRoot(t)
	directory := filepath.Join(parent, "new-registry")
	calls := 0
	location, err := locateHostProtectionWith(directory, func(path string) (string, error) {
		calls++
		if calls == 1 {
			if err := EnsurePrivateDirectory(directory); err != nil {
				t.Fatal(err)
			}
			return "", os.ErrNotExist
		}
		return canonicalPrivateLocation(path)
	})
	if err != nil {
		t.Fatalf("concurrent directory creation rejected: %v", err)
	}
	want, err := locateHostProtection(directory)
	if err != nil || location != want || location.Suffix != "" || calls != 2 {
		t.Fatalf("new directory identity was not verified: %+v %+v calls=%d err=%v", location, want, calls, err)
	}
}

func TestHostProtectionInconsistentLocationProbesRemainBounded(t *testing.T) {
	directory := protectionTestRoot(t)
	calls := 0
	_, err := locateHostProtectionWith(directory, func(string) (string, error) {
		calls++
		return "", os.ErrNotExist
	})
	if !errors.Is(err, ErrPath) || calls > 4 {
		t.Fatalf("unstable location not refused: calls=%d err=%v", calls, err)
	}
}

func TestHostProtectionFileAppearingBetweenLocationProbesIsRejected(t *testing.T) {
	directory := filepath.Join(protectionTestRoot(t), "file")
	_, err := locateHostProtectionWith(directory, func(string) (string, error) {
		if err := os.WriteFile(directory, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		return "", os.ErrNotExist
	})
	if !errors.Is(err, ErrPath) {
		t.Fatalf("non-directory accepted: %v", err)
	}
}
