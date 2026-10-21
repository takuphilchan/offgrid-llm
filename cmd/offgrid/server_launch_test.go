package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBackgroundServeUsesInjectedLauncherAndDisposableLogRoot(t *testing.T) {
	previousHome, previousHealth, previousLaunch := offgridServerHomeDir, offgridServerHealth, launchOffgridServe
	t.Cleanup(func() {
		offgridServerHomeDir, offgridServerHealth, launchOffgridServe = previousHome, previousHealth, previousLaunch
	})
	home := t.TempDir()
	offgridServerHomeDir = func() (string, error) { return home, nil }
	launches, probes := 0, 0
	launchOffgridServe = func(executable string, log io.Writer) (func() error, error) {
		want, err := os.Executable()
		if err != nil || executable != want {
			t.Fatalf("launch executable = %q, want %q: %v", executable, want, err)
		}
		launches++
		return func() error { return nil }, nil
	}
	offgridServerHealth = func() error { probes++; return nil }
	if err := startOffgridServerInBackground(); err != nil {
		t.Fatal(err)
	}
	if launches != 1 || probes != 1 {
		t.Fatalf("launches=%d health probes=%d", launches, probes)
	}
	if _, err := os.Stat(filepath.Join(home, ".offgrid", "logs", "server.log")); err != nil {
		t.Fatalf("disposable log path was not created: %v", err)
	}
}
