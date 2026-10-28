package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// This inventory is written by the installation/evaluation owner, never an API,
// profile, skill or model. It selects evidence; it cannot create qualification.
// Paths are private installation paths and are not projected to clients.
type agentToolInstallation struct {
	ModelID          string                       `json:"model_id"`
	Fingerprint      models.CapabilityFingerprint `json:"fingerprint"`
	Scope            string                       `json:"scope"`
	SuiteSHA256      string                       `json:"suite_sha256"`
	CapabilitySHA256 string                       `json:"capability_sha256"`
	Platform         string                       `json:"platform"`
	RuntimeFiles     map[string]string            `json:"runtime_files"`
}

func (i agentToolInstallation) target() models.CapabilityEvidenceTarget {
	return models.CapabilityEvidenceTarget{Fingerprint: i.Fingerprint, Scope: i.Scope,
		SuiteSHA256: i.SuiteSHA256, CapabilitySHA256: i.CapabilitySHA256, Platform: i.Platform}
}

func readAgentToolInstallations(path string) ([]agentToolInstallation, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("native agent installation inventory must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("native agent installation inventory must be a regular file")
	}
	const limit = 256 << 10
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || len(raw) > limit || jsonwire.Validate(raw) != nil {
		return nil, errors.New("invalid native agent installation inventory")
	}
	// encoding/json folds struct field names case-insensitively. Require
	// canonical keys so alternate casing cannot overwrite a reviewed value.
	var keys any
	if json.Unmarshal(raw, &keys) != nil || !agentInstallationKeys(keys, 0) {
		return nil, errors.New("native agent installation keys must be canonical")
	}
	var document struct {
		Version int                     `json:"version"`
		Models  []agentToolInstallation `json:"models"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&document) != nil || d.Decode(new(any)) != io.EOF || document.Version != 1 || len(document.Models) > 64 {
		return nil, errors.New("unsupported native agent installation inventory")
	}
	seen := map[string]bool{}
	for _, entry := range document.Models {
		key := entry.ModelID + "/" + strconv.Itoa(entry.Fingerprint.ContextWindow)
		if seen[key] || strings.TrimSpace(entry.ModelID) == "" || len(entry.ModelID) > 256 || strings.ContainsAny(entry.ModelID, "\x00\r\n") {
			return nil, errors.New("invalid or duplicate native agent model installation")
		}
		seen[key] = true
		if _, err := entry.Fingerprint.ID(); err != nil {
			return nil, err
		}
		if entry.Fingerprint.Adapter != inference.NativeAgentToolAdapter || entry.Fingerprint.RuntimeSHA256 == "" || entry.Fingerprint.RuntimeConfigSHA256 == "" || entry.Fingerprint.PreprocessSHA256 != "" || entry.Fingerprint.ProjectorSHA256 != "" {
			return nil, errors.New("native agent installation requires a measured text runtime")
		}
		if _, ok := models.NativeWorkspaceCasesForScope(entry.Scope); !ok || entry.SuiteSHA256 != models.NativeWorkspaceSuiteSHA256ForScope(entry.Scope) || entry.CapabilitySHA256 != agents.NativeWorkspaceCapabilitySHA256() || entry.Platform != runtime.GOOS+"-"+runtime.GOARCH {
			return nil, errors.New("native agent installation scope or platform does not match")
		}
		if len(entry.RuntimeFiles) < 2 || len(entry.RuntimeFiles) > 256 || entry.RuntimeFiles["executable"] == "" || entry.RuntimeFiles["offgrid-service"] == "" {
			return nil, errors.New("native agent runtime inventory is incomplete")
		}
		for _, path := range entry.RuntimeFiles {
			if !filepath.IsAbs(path) || filepath.Clean(path) != path {
				return nil, errors.New("runtime inventory requires clean absolute paths")
			}
		}
	}
	return document.Models, nil
}

func agentInstallationKeys(value any, depth int) bool {
	if depth > 8 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key != strings.ToLower(key) || !agentInstallationKeys(child, depth+1) {
				return false
			}
		}
	case []any:
		for _, child := range v {
			if !agentInstallationKeys(child, depth+1) {
				return false
			}
		}
	}
	return true
}
