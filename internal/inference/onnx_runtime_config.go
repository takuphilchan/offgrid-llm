package inference

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// This installation-owned inventory is never supplied by a model package or
// a request. Every interpreter, extension, dependency and worker byte is pinned.
type ONNXEmbeddingRuntimeConfig struct {
	SchemaVersion           int               `json:"schema_version"`
	RunnerRevision          string            `json:"runner_revision"`
	RuntimeDirectory        string            `json:"runtime_directory"`
	Python                  string            `json:"python"`
	Script                  string            `json:"script"`
	RuntimeFiles            map[string]string `json:"runtime_files"`
	DependencyVersions      map[string]string `json:"dependency_versions"`
	StateDirectory          string            `json:"state_directory"`
	NamespaceLauncher       string            `json:"namespace_launcher,omitempty"`
	NamespaceLauncherSHA256 string            `json:"namespace_launcher_sha256,omitempty"`
}

func DecodeONNXEmbeddingRuntimeConfig(reader io.Reader) (ONNXEmbeddingRuntimeConfig, error) {
	var config ONNXEmbeddingRuntimeConfig
	raw, err := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
	if err != nil {
		return config, err
	}
	if len(raw) > 4<<20 || jsonwire.Validate(raw) != nil {
		return config, errors.New("ONNX runtime configuration is invalid or oversized")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) < 8 || len(fields) > 10 {
		return config, errors.New("ONNX runtime configuration requires exact installation fields")
	}
	allowed := map[string]bool{"schema_version": true, "runner_revision": true, "runtime_directory": true, "python": true, "script": true, "runtime_files": true, "dependency_versions": true, "state_directory": true, "namespace_launcher": true, "namespace_launcher_sha256": true}
	for name := range fields {
		if !allowed[name] {
			return config, errors.New("ONNX runtime configuration contains an unsupported field")
		}
	}
	for _, name := range []string{"schema_version", "runner_revision", "runtime_directory", "python", "script", "runtime_files", "dependency_versions", "state_directory"} {
		if len(fields[name]) == 0 || bytes.Equal(fields[name], []byte("null")) {
			return config, errors.New("ONNX runtime configuration has missing or ambiguous fields")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}
	return config, config.Validate()
}

func (c ONNXEmbeddingRuntimeConfig) Validate() error {
	if c.SchemaVersion != 1 || c.RunnerRevision == "" || len(c.RunnerRevision) > 96 || len(c.RuntimeFiles) < 2 || len(c.RuntimeFiles) > 16384 || len(c.DependencyVersions) > 32 {
		return errors.New("ONNX runtime requires a bounded pinned installation inventory")
	}
	for _, directory := range []string{c.RuntimeDirectory, c.StateDirectory} {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || len(directory) > 2048 {
			return errors.New("ONNX runtime directories must be canonical absolute paths")
		}
	}
	if c.RuntimeDirectory == c.StateDirectory || strings.HasPrefix(c.StateDirectory+string(filepath.Separator), c.RuntimeDirectory+string(filepath.Separator)) || strings.HasPrefix(c.RuntimeDirectory+string(filepath.Separator), c.StateDirectory+string(filepath.Separator)) {
		return errors.New("ONNX runtime and writable state must be separate trees")
	}
	for name, digest := range c.RuntimeFiles {
		if !safeONNXRuntimeRelative(name) || !completeSHA256Digest(digest) {
			return errors.New("ONNX runtime file pin is invalid")
		}
	}
	if !safeONNXRuntimeRelative(c.Python) || !safeONNXRuntimeRelative(c.Script) || c.Python == c.Script || c.RuntimeFiles[c.Python] == "" || c.RuntimeFiles[c.Script] == "" {
		return errors.New("ONNX interpreter and worker must belong to the pinned runtime inventory")
	}
	if runtime.GOOS == "windows" && c.RuntimeFiles[strings.TrimSuffix(c.Python, path.Ext(c.Python))+"._pth"] == "" {
		return errors.New("ONNX Python requires a pinned adjacent isolated search-path file")
	}
	if c.NamespaceLauncher != "" || c.NamespaceLauncherSHA256 != "" {
		if !filepath.IsAbs(c.NamespaceLauncher) || !completeSHA256Digest(c.NamespaceLauncherSHA256) {
			return errors.New("ONNX namespace launcher requires an absolute path and complete pin")
		}
	}
	_, err := (TrustedNativeWorker{Provider: ProviderONNX, DependencyVersions: c.DependencyVersions}).dependencyIdentity()
	return err
}

func safeONNXRuntimeRelative(value string) bool {
	if value == "" || len(value) > 512 || strings.ContainsAny(value, `\:*?"<>|`) {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
			return false
		}
	}
	return true
}
