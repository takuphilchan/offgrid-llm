//go:build windows

package computer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstalledApplicationPathValidation(t *testing.T) {
	dir := canonicalApplicationFixtureDir(t)
	path := filepath.Join(dir, "ordinary.exe")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{dir, path} {
		if err := validateInstalledApplicationPath(value); err != nil {
			t.Fatalf("valid local object rejected: %v", err)
		}
	}
	for _, value := range []string{"", "ordinary.exe", `C:ordinary.exe`, `\\host\share\app.exe`, `\\?\` + path, path + ":stream", filepath.Join(dir, "missing.exe")} {
		if validateInstalledApplicationPath(value) == nil {
			t.Errorf("accepted unsafe/missing path %q", value)
		}
	}
}

func TestInstalledApplicationPathRejectsRedirectedObjects(t *testing.T) {
	dir := canonicalApplicationFixtureDir(t)
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(real, "app.exe")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ target, link, check string }{
		{path, filepath.Join(dir, "link.exe"), filepath.Join(dir, "link.exe")},
		{real, filepath.Join(dir, "redirect"), filepath.Join(dir, "redirect", "app.exe")},
	} {
		if err := os.Symlink(tc.target, tc.link); err != nil {
			t.Skipf("symlink fixture unavailable: %v", err)
		}
		if validateInstalledApplicationPath(tc.check) == nil {
			t.Fatal("redirected application path accepted")
		}
	}
}

func TestInstalledApplicationPathRejectsJunction(t *testing.T) {
	// Unlike symbolic links, a junction fixture needs no elevated privilege.
	// Both paths belong to this test; no installed application is touched.
	dir := canonicalApplicationFixtureDir(t)
	real, link := filepath.Join(dir, "real"), filepath.Join(dir, "redirect")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "app.exe"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Fatalf("create isolated junction fixture: %v %s", err, output)
	}
	defer os.Remove(link)
	if validateInstalledApplicationPath(link) == nil || validateInstalledApplicationPath(filepath.Join(link, "app.exe")) == nil {
		t.Fatal("junction redirection was accepted")
	}
}
