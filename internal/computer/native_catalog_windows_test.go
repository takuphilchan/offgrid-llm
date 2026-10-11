//go:build windows && amd64

package computer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRegisteredApplicationRejectsCommandsAndRemotePaths(t *testing.T) {
	for _, value := range []string{`cmd.exe /c start`, `C:\Tools\app.exe --run`, `"C:\Tools\app.exe" --run`, `\\host\share\app.exe`, `\\?\C:\Tools\app.exe`, `C:\Tools\a.exe:evil.exe`, `C:\Tools\script.ps1`, "C:\\app.exe\x00"} {
		if _, err := registeredApplicationPath(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{`C:\Tools\Unknown Product\program.exe`, `"C:\Tools\Unknown Product\program.exe"`} {
		if got, err := registeredApplicationPath(value); err != nil || got != `C:\Tools\Unknown Product\program.exe` {
			t.Fatal(got, err)
		}
	}
}

func TestRegisteredApplicationUsesExecutableTypeNotProductAllowlist(t *testing.T) {
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "System32", "notepad.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture unavailable")
	}
	if !ordinaryGUIExecutable(path, "Previously Unknown Editor") {
		t.Fatal("ordinary GUI registration rejected")
	}
	if ordinaryGUIExecutable(filepath.Join(root, "System32", "cmd.exe"), "Friendly Editor") {
		t.Fatal("console interpreter accepted")
	}
	if ordinaryGUIExecutable(path, "Uninstall editor") {
		t.Fatal("uninstall entry accepted")
	}
}

func TestInstalledApplicationCatalogReadOnly(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_APPLICATION_DISCOVERY") != "1" {
		t.Skip("opt-in read-only catalog inspection")
	}
	apps, err := installedNativeApplications()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	packaged := 0
	for _, app := range apps {
		key := app.path
		if app.packageIdentity != nil {
			packaged++
			key = app.packageIdentity.AUMID
		}
		if seen[key] || checkInstalledApplication(app) != nil {
			t.Fatal("invalid catalog entry")
		}
		seen[key] = true
	}
	t.Logf("validated %d registered GUI applications (%d packaged); no applications launched", len(apps), packaged)
	if len(apps) == 0 {
		t.Fatal("empty installed catalog")
	}
}
