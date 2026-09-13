package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHermesPluginBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, ".hermes"))
	result, err := InstallPluginBundle("hermes", home, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.PluginID != "offgrid" || len(result.Files) < 2 {
		t.Fatalf("unexpected install result: %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(result.Path, "__init__.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !stringsContainAll(string(content), "name=\"offgrid\"", "register_provider") {
		t.Fatalf("unexpected Hermes plugin:\n%s", content)
	}
}

func TestInstallPluginBundleIsIdempotentWhenFilesAreUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, ".hermes"))
	if _, err := InstallPluginBundle("hermes", home, false); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPluginBundle("hermes", home, false); err != nil {
		t.Fatalf("second unchanged install failed: %v", err)
	}
}

func TestInstallHermesPluginUpgradesKnownLegacyReadme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, ".hermes"))
	result, err := InstallPluginBundle("hermes", home, false)
	if err != nil {
		t.Fatal(err)
	}
	legacy := "# OffGrid provider for Hermes Agent\n\nThis plugin registers `offgrid` as a Hermes model provider. It talks directly\nto OffGrid's OpenAI-compatible API.\n\nSet `OFFGRID_BASE_URL` to the reachable OffGrid `/v1` URL and set\n`OFFGRID_API_KEY` to `offgrid-local` when the local service does not require a\ntoken. OffGrid requires at least an 8,192-token runtime window for this\nintegration and recommends 65,536 tokens for long-running agent sessions.\n"
	if digest := contentDigest([]byte(legacy)); !knownLegacyBundleFile("hermes", "README.md", digest) {
		t.Fatalf("legacy README digest changed: %s", digest)
	}
	readmePath := filepath.Join(result.Path, "README.md")
	if err := os.WriteFile(readmePath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(result.Path, managedManifestName)); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPluginBundle("hermes", home, false); err != nil {
		t.Fatalf("known first-party revision should upgrade: %v", err)
	}
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "64,000-token") {
		t.Fatalf("legacy README was not upgraded: %s", content)
	}
	if _, err := os.Stat(filepath.Join(result.Path, managedManifestName)); err != nil {
		t.Fatalf("managed ownership manifest was not written: %v", err)
	}
}

func TestInstallPluginBundleUpgradesUnmodifiedManagedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, ".hermes"))
	result, err := InstallPluginBundle("hermes", home, false)
	if err != nil {
		t.Fatal(err)
	}
	readmePath := filepath.Join(result.Path, "README.md")
	oldContent := []byte("# Prior managed revision\n")
	if err := os.WriteFile(readmePath, oldContent, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(result.Path, managedManifestName)
	manifest, err := readManagedManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Files["README.md"] = contentDigest(oldContent)
	content, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPluginBundle("hermes", home, false); err != nil {
		t.Fatalf("unchanged managed revision should upgrade: %v", err)
	}
}

func TestInstallPluginBundleProtectsModifiedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", filepath.Join(home, ".hermes"))
	result, err := InstallPluginBundle("hermes", home, false)
	if err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(result.Path, "__init__.py")
	if err := os.WriteFile(plugin, []byte("# locally modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPluginBundle("hermes", home, false); err == nil || !strings.Contains(err.Error(), "modified plugin file") {
		t.Fatalf("expected modified-file protection, got %v", err)
	}
	if _, err := InstallPluginBundle("hermes", home, true); err != nil {
		t.Fatalf("forced install failed: %v", err)
	}
}

func TestInstallOpenClawPluginBundle(t *testing.T) {
	home := t.TempDir()
	result, err := InstallPluginBundle("openclaw", home, false)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(result.Path, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !stringsContainAll(string(content), `const id = "offgrid"`, "definePluginEntry", `model.type === "embedding"`) {
		t.Fatalf("unexpected OpenClaw plugin:\n%s", content)
	}
	if _, err := InstallPluginBundle("openclaw", home, false); err != nil {
		t.Fatalf("unchanged second install failed: %v", err)
	}
}

func stringsContainAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
