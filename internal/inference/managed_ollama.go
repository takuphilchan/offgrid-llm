package inference

import (
	"errors"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ManagedOllamaConfig is trusted service installation metadata. It cannot be
// supplied by chat, model packages or generated output. Roots must already
// exist; this path never installs a runtime or imports/downloads a model.
type ManagedOllamaConfig struct {
	RuntimeDirectory             string            `json:"runtime_directory"`
	Executable                   string            `json:"executable"`
	RuntimeFiles                 map[string]string `json:"runtime_files"`
	ModelDirectory               string            `json:"model_directory"`
	StateDirectory               string            `json:"state_directory"`
	TrustedSingleUserHost        bool              `json:"trusted_single_user_host"`
	AllowUnauthenticatedLoopback bool              `json:"allow_unauthenticated_loopback"`
	NamespaceLauncher            string            `json:"namespace_launcher,omitempty"`
	NamespaceLauncherSHA256      string            `json:"namespace_launcher_sha256,omitempty"`
}

var managedOllamaName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}:[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

func validateManagedOllama(endpoint HTTPEndpoint) error {
	c := endpoint.ManagedOllama
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" || c == nil || endpoint.Remote || endpoint.Descriptor.Provider != ProviderOllama || endpoint.Descriptor.Format != FormatGGUF || endpoint.Descriptor.Backend != "local-ollama" || !c.TrustedSingleUserHost || !c.AllowUnauthenticatedLoopback {
		return errors.New("managed Ollama requires a supported ownership platform and explicit trusted single-user unauthenticated-loopback consent")
	}
	if runtime.GOOS == "linux" && (!filepath.IsAbs(c.NamespaceLauncher) || filepath.Clean(c.NamespaceLauncher) != c.NamespaceLauncher || !completeSHA256Digest(c.NamespaceLauncherSHA256)) || runtime.GOOS != "linux" && (c.NamespaceLauncher != "" || c.NamespaceLauncherSHA256 != "") {
		return errors.New("managed Ollama Linux namespace launcher metadata is missing or invalid for this platform")
	}
	u, err := url.Parse(endpoint.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || len(endpoint.AllowedIPs) != 1 || endpoint.AllowedIPs[0] != "127.0.0.1" || !managedOllamaName.MatchString(endpoint.Model) {
		return errors.New("managed Ollama requires a pinned IPv4 loopback origin and explicit legacy model tag")
	}
	if len(c.RuntimeFiles) == 0 || len(c.RuntimeFiles) > 256 || !managedRelativePath(c.Executable) || runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(c.Executable), ".exe") || c.RuntimeFiles[c.Executable] == "" {
		return errors.New("managed Ollama requires a bounded complete runtime file inventory")
	}
	seen := map[string]bool{}
	for name, digest := range c.RuntimeFiles {
		key := strings.ToLower(name)
		if !managedRelativePath(name) || !completeSHA256Digest(digest) || seen[key] {
			return errors.New("managed Ollama runtime inventory is invalid")
		}
		seen[key] = true
	}
	roots := []string{c.RuntimeDirectory, c.ModelDirectory, c.StateDirectory}
	for i, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root || len(root) > 2048 || strings.HasPrefix(root, `\\`) {
			return errors.New("managed Ollama requires canonical local absolute directories")
		}
		for j, other := range roots {
			if i == j {
				continue
			}
			rel, err := filepath.Rel(strings.ToLower(root), strings.ToLower(other))
			if err == nil && (rel == "." || filepath.IsLocal(rel)) {
				return errors.New("managed Ollama directories must be separate")
			}
		}
	}
	return nil
}

func managedRelativePath(name string) bool {
	if name == "" || len(name) > 512 || !filepath.IsLocal(filepath.FromSlash(name)) || strings.ContainsAny(name, `\:`) || strings.Contains(name, "//") || strings.Contains(name, "../") || strings.HasSuffix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return false
		}
	}
	return true
}

func cloneManagedOllama(c *ManagedOllamaConfig) *ManagedOllamaConfig {
	if c == nil {
		return nil
	}
	copy := *c
	copy.RuntimeFiles = make(map[string]string, len(c.RuntimeFiles))
	for path, digest := range c.RuntimeFiles {
		copy.RuntimeFiles[path] = digest
	}
	return &copy
}
