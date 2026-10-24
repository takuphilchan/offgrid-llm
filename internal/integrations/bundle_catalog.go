package integrations

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type bundleDefinition struct {
	source string
	path   func(string) string
	next   func(string) []string
}

// Trusted executable bundles are registered here, never supplied by an HTTP
// response or arbitrary user metadata.
var bundledProviders = map[string]bundleDefinition{
	"hermes-verifier": {"plugins/hermes/offgrid-verifier", func(home string) string { return filepath.Join(HermesHome(home), "plugins", "offgrid-verifier") }, func(string) []string {
		return []string{"Review this executable hook plugin, then explicitly enable: hermes plugins enable offgrid-verifier", "Verification is active only for offgrid hermes run --verify-contract <file.json>"}
	}},
	"hermes": {"plugins/hermes/offgrid", HermesPluginPath, func(string) []string {
		return []string{"Run offgrid hermes install to configure and verify the provider"}
	}},
	"openclaw": {"plugins/openclaw/offgrid", func(home string) string { return filepath.Join(home, ".offgrid", "integrations", "openclaw-offgrid") }, func(path string) []string {
		return []string{fmt.Sprintf("openclaw plugins install --link %q", path), "Run offgrid openclaw install to configure and verify the provider"}
	}},
}

func PluginBundlePath(id, home string) (string, error) {
	b, ok := bundledProviders[id]
	if !ok {
		return "", fmt.Errorf("unsupported integration %q", id)
	}
	return b.path(home), nil
}

// InspectPluginBundle compares executable files with the bundled trusted
// revision; mere file existence is not evidence of a compatible provider.
func InspectPluginBundle(id, path string) (string, error) {
	b, ok := bundledProviders[id]
	if !ok {
		return "", fmt.Errorf("unsupported integration %q", id)
	}
	var combined []byte
	err := fs.WalkDir(pluginBundles, b.source, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(b.source, name)
		if err != nil {
			return err
		}
		target := filepath.Join(path, rel)
		if err = privatePath(target, false); err != nil {
			return err
		}
		want, err := pluginBundles.ReadFile(name)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if contentDigest(got) != contentDigest(want) {
			return fmt.Errorf("provider bundle revision differs; review and update the managed provider")
		}
		combined = append(combined, []byte(name+":"+contentDigest(got)+"\n")...)
		return nil
	})
	if err != nil {
		return "", err
	}
	return contentDigest(combined), nil
}
