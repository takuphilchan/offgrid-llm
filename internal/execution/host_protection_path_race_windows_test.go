package execution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProtectionJunctionAppearingBetweenLocationProbesIsRejected(t *testing.T) {
	parent := protectionTestRoot(t)
	target := protectionTestDirectory(t, parent, "target")
	alias := filepath.Join(parent, "alias")
	calls := 0
	_, err := locateHostProtectionWith(alias, func(string) (string, error) {
		calls++
		createWorkingCopyTestJunction(t, alias, target)
		return "", os.ErrNotExist
	})
	if !errors.Is(err, ErrPath) || calls != 1 {
		t.Fatalf("reparse appearance retried/accepted: calls=%d err=%v", calls, err)
	}
}
