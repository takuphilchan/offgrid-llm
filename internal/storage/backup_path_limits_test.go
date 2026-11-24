package storage

import (
	"context"
	"strings"
	"testing"
)

func TestBackupPathIndexRejectsDeepAndOversizedNames(t *testing.T) {
	for _, name := range []string{strings.Repeat("a/", 32760) + "a", strings.Repeat("a/", maxBackupPathDepth) + "a", strings.Repeat("a", maxBackupPathBytes+1), "invalid\xff"} {
		paths := backupPaths{}
		if err := paths.add(name); err == nil || paths.entries != 0 || paths.bytes != 0 {
			t.Fatal("unbounded path entered index", err, paths.entries, paths.bytes)
		}
		if !strings.Contains(name, "\xff") {
			archive := makeBackupFixture(t, name, []byte("x"), nil)
			if _, err := VerifyBackup(context.Background(), archive); err == nil {
				t.Fatal("deep central-directory path accepted")
			}
		}
	}
}

func TestBackupPathIndexHasCumulativeBudgets(t *testing.T) {
	for _, paths := range []*backupPaths{{entries: maxBackupPathEntries}, {bytes: maxBackupPathIndexBytes - 1}} {
		if err := paths.add("new.txt"); err == nil {
			t.Fatal("path index budget exceeded")
		}
		if paths.root.children != nil {
			t.Fatal("index allocated after budget rejection")
		}
	}
	paths := backupPaths{}
	name := strings.Repeat("a/", maxBackupPathDepth-1) + "z"
	if err := paths.add(name); err != nil {
		t.Fatal("bounded path rejected", err)
	}
	if paths.entries != maxBackupPathDepth || paths.bytes != 2*maxBackupPathDepth {
		t.Fatal("components were stored as growing prefixes", paths.entries, paths.bytes)
	}
}
