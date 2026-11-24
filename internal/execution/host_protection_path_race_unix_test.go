//go:build !windows

package execution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProtectionSymlinkAppearingBetweenLocationProbesIsRejected(t *testing.T) {
	parent := protectionTestRoot(t)
	target := protectionTestDirectory(t, parent, "target")
	alias := filepath.Join(parent, "alias")
	calls := 0
	_, err := locateHostProtectionWith(alias, func(string) (string, error) {
		calls++
		if err := os.Symlink(target, alias); err != nil {
			t.Fatal(err)
		}
		return "", os.ErrNotExist
	})
	if !errors.Is(err, ErrPath) || calls != 1 {
		t.Fatalf("symlink appearance retried/accepted: calls=%d err=%v", calls, err)
	}
}
