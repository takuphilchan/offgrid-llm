//go:build linux || darwin

package main

import (
	"os"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func makeBrowserStoragePublic(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0644)
	if info.IsDir() {
		mode = 0755
	}
	if err = os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	if execution.CheckPrivatePath(path) == nil {
		t.Fatal("fixture did not expose broad access")
	}
}
