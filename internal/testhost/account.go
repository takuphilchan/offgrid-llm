// Package testhost supplies isolated native-host state to integration tests.
// No fixture borrows an installed account registry or changes existing ACLs.
package testhost

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func Account(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		// Keep compiler caches at their original locations before isolating HOME.
		// Otherwise helper builds populate the disposable application profile with
		// read-only module trees, and its cleanup fails even after a passing test.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, "go", "env", "-json", "GOPATH", "GOMODCACHE", "GOCACHE").Output()
		if err != nil {
			t.Fatalf("resolve compiler cache paths before isolating account: %v", err)
		}
		var paths map[string]string
		if err := json.Unmarshal(output, &paths); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"GOPATH", "GOMODCACHE", "GOCACHE"} {
			if paths[key] == "" {
				t.Fatalf("compiler environment omitted %s", key)
			}
			t.Setenv(key, paths[key])
		}
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if execution.CheckPrivateAncestry(base) != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		home, err = filepath.EvalSymlinks(home)
		if err != nil {
			t.Fatal(err)
		}
		if err = execution.CheckPrivateAncestry(home); err != nil {
			t.Fatalf("isolated host ancestor unavailable: %v", err)
		}
		base, err = os.MkdirTemp(home, ".offgrid-test-host-")
		if err != nil {
			t.Fatal(err)
		}
		owned, err := os.Lstat(base)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			current, err := os.Lstat(base)
			if os.IsNotExist(err) {
				return
			}
			if err != nil || filepath.Dir(base) != home || !strings.HasPrefix(filepath.Base(base), ".offgrid-test-host-") || !os.SameFile(owned, current) {
				t.Errorf("isolated host cleanup identity changed: %v", err)
				return
			}
			if err := os.RemoveAll(base); err != nil {
				t.Error(err)
			}
		})
	}
	if err := execution.ProtectPrivatePath(base); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(base, "config")
	if err := execution.EnsurePrivateDirectory(config); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", config)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("OFFGRID_TEST_ACCOUNT_ROOT", base)
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", base)
		if err := os.MkdirAll(filepath.Join(base, "Library", "Application Support"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func PrivateDirectory(t *testing.T) string {
	t.Helper()
	base := os.Getenv("OFFGRID_TEST_ACCOUNT_ROOT")
	if !filepath.IsAbs(base) || execution.CheckPrivateAncestry(base) != nil || execution.CheckPrivatePath(base) != nil {
		t.Fatal("Account(t) must create isolated private host state first")
	}
	directory, err := os.MkdirTemp(base, "host-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		current, err := os.Lstat(directory)
		if os.IsNotExist(err) {
			return
		}
		if err != nil || filepath.Dir(directory) != base || !strings.HasPrefix(filepath.Base(directory), "host-fixture-") || !os.SameFile(owned, current) {
			t.Errorf("isolated private fixture identity changed: %v", err)
			return
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	if err := execution.ProtectPrivatePath(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}
