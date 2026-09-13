package integrations

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const managedManifestName = ".offgrid-managed.json"

type managedManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

type bundledFile struct {
	name        string
	destination string
	content     []byte
	unchanged   bool
}

func contentDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// The original Hermes bundle had no ownership manifest. Its README was
// updated when the actual Hermes 64K requirement became clear. Recognize only
// this exact first-party revision; arbitrary user edits still require --force.
func knownLegacyBundleFile(integrationID, name, digest string) bool {
	return integrationID == "hermes" && name == "README.md" &&
		digest == "772b9fccec0514f8eea928b1e6ca1a9b3a61848667f3587182b5cb8ee433abd4"
}

func readManagedManifest(path string) (managedManifest, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return managedManifest{Version: 1, Files: make(map[string]string)}, nil
	}
	if err != nil {
		return managedManifest{}, err
	}
	if !info.Mode().IsRegular() {
		return managedManifest{}, fmt.Errorf("refusing non-regular managed manifest %s", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return managedManifest{}, err
	}
	var manifest managedManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return managedManifest{}, fmt.Errorf("invalid managed manifest %s: %w", path, err)
	}
	if manifest.Version != 1 || manifest.Files == nil {
		return managedManifest{}, fmt.Errorf("unsupported managed manifest %s", path)
	}
	return manifest, nil
}

//go:embed plugins/hermes/offgrid/* plugins/openclaw/offgrid/*
var pluginBundles embed.FS

// InstallResult describes files written by InstallPluginBundle. External
// runtimes may require one follow-up command to register those files.
type InstallResult struct {
	Integration string   `json:"integration"`
	PluginID    string   `json:"plugin_id"`
	Path        string   `json:"path"`
	Files       []string `json:"files"`
	Next        []string `json:"next"`
}

// HermesHome returns the data directory used by Hermes Agent on the current
// platform. HERMES_HOME always wins so profiles and managed installations keep
// working without OffGrid duplicating Hermes' path configuration.
func HermesHome(userHome string) string {
	if configured := strings.TrimSpace(os.Getenv("HERMES_HOME")); configured != "" {
		return configured
	}
	if runtime.GOOS == "windows" {
		if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
			return filepath.Join(localAppData, "hermes")
		}
	}
	return filepath.Join(userHome, ".hermes")
}

// HermesPluginPath returns the first-party OffGrid model-provider location.
func HermesPluginPath(userHome string) string {
	return filepath.Join(HermesHome(userHome), "plugins", "model-providers", "offgrid")
}

// InstallPluginBundle installs the first-party provider bundle into the
// current user's integration directory. It never overwrites a different
// plugin silently; callers must explicitly pass overwrite for upgrades.
func InstallPluginBundle(integrationID, userHome string, overwrite bool) (InstallResult, error) {
	integrationID = strings.ToLower(strings.TrimSpace(integrationID))
	userHome = strings.TrimSpace(userHome)
	if userHome == "" {
		return InstallResult{}, fmt.Errorf("user home is required")
	}

	var source, target string
	var next []string
	switch integrationID {
	case "hermes":
		source = "plugins/hermes/offgrid"
		target = HermesPluginPath(userHome)
		next = []string{
			"Set OFFGRID_API_KEY=offgrid-local and OFFGRID_BASE_URL=http://127.0.0.1:11611/v1",
			"Run hermes doctor",
		}
	case "openclaw":
		source = "plugins/openclaw/offgrid"
		target = filepath.Join(userHome, ".offgrid", "integrations", "openclaw-offgrid")
		next = []string{
			fmt.Sprintf("openclaw plugins install --link %q", target),
			"Set OFFGRID_API_KEY=offgrid-local and OFFGRID_BASE_URL=http://127.0.0.1:11611/v1",
			"Run openclaw plugins inspect offgrid --runtime --json",
		}
	default:
		return InstallResult{}, fmt.Errorf("unsupported integration %q", integrationID)
	}

	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return InstallResult{}, fmt.Errorf("refusing to install through symbolic link %s", target)
		}
		if !info.IsDir() {
			return InstallResult{}, fmt.Errorf("plugin target is not a directory: %s", target)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return InstallResult{}, fmt.Errorf("inspect plugin target: %w", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("create plugin target: %w", err)
	}

	result := InstallResult{Integration: integrationID, PluginID: "offgrid", Path: target, Next: next}
	manifestPath := filepath.Join(target, managedManifestName)
	manifest, err := readManagedManifest(manifestPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("inspect %s plugin: %w", integrationID, err)
	}
	files := make([]bundledFile, 0, 4)
	err = fs.WalkDir(pluginBundles, source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, readErr := pluginBundles.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		name := filepath.Base(path)
		destination := filepath.Join(target, name)
		file := bundledFile{name: name, destination: destination, content: content}
		if info, statErr := os.Lstat(destination); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("refusing to overwrite non-regular plugin file %s", destination)
			}
			existing, readExistingErr := os.ReadFile(destination)
			if readExistingErr != nil {
				return readExistingErr
			}
			file.unchanged = bytes.Equal(existing, content)
			if !file.unchanged && !overwrite {
				digest := contentDigest(existing)
				if digest != manifest.Files[name] && !knownLegacyBundleFile(integrationID, name, digest) {
					return fmt.Errorf("refusing to overwrite modified plugin file %s (use --force to replace it)", destination)
				}
			}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		return InstallResult{}, fmt.Errorf("install %s plugin: %w", integrationID, err)
	}
	for _, file := range files {
		if !file.unchanged {
			if err := os.WriteFile(file.destination, file.content, 0o644); err != nil {
				return InstallResult{}, fmt.Errorf("install %s plugin: %w", integrationID, err)
			}
		}
		manifest.Files[file.name] = contentDigest(file.content)
		result.Files = append(result.Files, file.destination)
	}
	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return InstallResult{}, err
	}
	manifestContent = append(manifestContent, '\n')
	if err := os.WriteFile(manifestPath, manifestContent, 0o644); err != nil {
		return InstallResult{}, fmt.Errorf("record %s plugin ownership: %w", integrationID, err)
	}
	result.Files = append(result.Files, manifestPath)
	return result, nil
}
