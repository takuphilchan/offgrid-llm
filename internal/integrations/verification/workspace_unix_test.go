//go:build linux || darwin

package verification

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSpecialFilesAndPermissionChangesFail(t *testing.T) {
	for _, kind := range []string{"fifo", "source-permissions", "root-permissions"} {
		t.Run(kind, func(t *testing.T) {
			v, root := testVerifier(t, NoChanges, []byte("original"))
			var err error
			switch kind {
			case "fifo":
				err = syscall.Mkfifo(filepath.Join(root, "pipe"), 0600)
			case "source-permissions":
				err = os.Chmod(filepath.Join(root, "source.txt"), 0644)
			case "root-permissions":
				err = os.Chmod(root, v.baseline["."].mode.Perm()^0001)
			}
			if err != nil {
				t.Fatal(err)
			}
			if r := v.Finish(background, "completed"); r.State != "failed" {
				t.Fatal(r)
			}
		})
	}
}
