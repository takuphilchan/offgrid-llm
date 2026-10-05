package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
)

// PackageManifestVersion versions data, not executable adapter code. Importing a
// manifest never installs its runtime or establishes qualification/provenance.
const PackageManifestVersion = 1
const MaxPackageManifestBytes = 1 << 20
const MaxPackageBytes int64 = 32 << 30
const MaxPackageArtifacts = 2048

type ModelCapability string

const (
	CapabilityChat                 ModelCapability = "chat"
	CapabilityEmbeddings           ModelCapability = "embeddings"
	CapabilityTranscription        ModelCapability = "transcription"
	CapabilityStreamingRecognition ModelCapability = "streaming_recognition"
	CapabilitySynthesis            ModelCapability = "speech_synthesis"
	CapabilityIncrementalSynthesis ModelCapability = "incremental_synthesis"
)

type PackageRuntime struct {
	Adapter  string `json:"adapter"`
	Revision string `json:"revision"`
}

type ArtifactSource struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Path       string `json:"path"`
}

type PackageArtifact struct {
	Path    string          `json:"path"`
	Role    string          `json:"role"`
	Size    int64           `json:"size"`
	SHA256  string          `json:"sha256"`
	License string          `json:"license"`
	Source  *ArtifactSource `json:"source,omitempty"`
}

type PackageVoice struct {
	ID       string `json:"id"`
	Language string `json:"language"`
}

// Hardware evidence is a reference, not a self-certified readiness assertion.
type PackageHardwareProfile struct {
	ID          string `json:"id"`
	MemoryBytes int64  `json:"memory_bytes"`
	Evidence    string `json:"evidence"`
}

type ModelPackageManifest struct {
	SchemaVersion    int                      `json:"schema_version"`
	ID               string                   `json:"id"`
	Revision         string                   `json:"revision"`
	Name             string                   `json:"name"`
	Architecture     string                   `json:"architecture"`
	Runtime          PackageRuntime           `json:"runtime"`
	Capabilities     []ModelCapability        `json:"capabilities"`
	Languages        []string                 `json:"languages"`
	Voices           []PackageVoice           `json:"voices,omitempty"`
	SampleRates      []int                    `json:"sample_rates"`
	License          string                   `json:"license"`
	Artifacts        []PackageArtifact        `json:"artifacts"`
	HardwareProfiles []PackageHardwareProfile `json:"hardware_profiles,omitempty"`
}

var packageComponent = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)
var packageDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var sourceRevision = regexp.MustCompile(`^[a-f0-9]{40}([a-f0-9]{24})?$`)
var packageLanguage = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

type packageLayout struct {
	adapter      string
	required     map[string][]string
	capabilities []ModelCapability
}

var packageLayouts = map[string]packageLayout{
	"whisper":             {"whisper.cpp", map[string][]string{"weights": {".bin", ".gguf", ".ggml"}}, []ModelCapability{CapabilityTranscription}},
	"piper":               {"piper", map[string][]string{"weights": {".onnx"}, "config": {".json"}}, []ModelCapability{CapabilitySynthesis}},
	"zipformer-streaming": {"sherpa-onnx", map[string][]string{"encoder": {".onnx"}, "decoder": {".onnx"}, "joiner": {".onnx"}, "tokens": {".txt"}}, []ModelCapability{CapabilityTranscription, CapabilityStreamingRecognition}},
	"kokoro":              {"sherpa-onnx", map[string][]string{"weights": {".onnx"}, "voices": {".bin"}, "tokens": {".txt"}}, []ModelCapability{CapabilitySynthesis}},
	"qwen3-asr":           {"qwen3-asr", map[string][]string{"weights": {".safetensors"}, "config": {".json"}}, []ModelCapability{CapabilityTranscription}},
	"qwen3-tts":           {"qwen3-tts", map[string][]string{"weights": {".safetensors"}, "config": {".json"}}, []ModelCapability{CapabilitySynthesis, CapabilityIncrementalSynthesis}},
}

func repeatablePackageRole(architecture, role string) bool {
	return (architecture == "qwen3-asr" || architecture == "qwen3-tts") && (role == "weights" || role == "config" || role == "tokenizer" || role == "codec")
}

func DecodePackageManifest(r io.Reader) (ModelPackageManifest, error) {
	var m ModelPackageManifest
	b, err := io.ReadAll(io.LimitReader(r, MaxPackageManifestBytes+1))
	if err != nil {
		return m, err
	}
	if len(b) > MaxPackageManifestBytes {
		return m, fmt.Errorf("package manifest exceeds limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, fmt.Errorf("invalid package manifest: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("package manifest must contain one JSON object")
	}
	return m, m.Validate()
}

func validPackageComponent(s string) bool {
	return packageComponent.MatchString(s) && !strings.Contains(s, "..") && !strings.HasSuffix(s, ".") && !reservedPackageName(s)
}

func reservedPackageName(s string) bool {
	n := strings.ToLower(strings.SplitN(s, ".", 2)[0])
	if slices.Contains([]string{"con", "prn", "aux", "nul", "clock$"}, n) {
		return true
	}
	return len(n) == 4 && (strings.HasPrefix(n, "com") || strings.HasPrefix(n, "lpt")) && n[3] >= '0' && n[3] <= '9'
}

// Portable paths exclude Windows ADS, device names, trailing-dot aliases, and
// case collisions even when the import is performed on a case-sensitive OS.
func validArtifactPath(s string) bool {
	if len(s) == 0 || len(s) > 240 || strings.ContainsAny(s, `\:*?"<>|`) || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") || reservedPackageName(part) {
			return false
		}
		for _, c := range part {
			if c < 32 || c == 127 {
				return false
			}
		}
	}
	return true
}

func (m ModelPackageManifest) Validate() error {
	if m.SchemaVersion != PackageManifestVersion {
		return fmt.Errorf("unsupported package schema version %d", m.SchemaVersion)
	}
	if !validPackageComponent(m.ID) || !validPackageComponent(m.Revision) || slices.Contains([]string{"main", "master", "latest"}, m.Revision) {
		return fmt.Errorf("invalid package ID or immutable revision")
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 200 || strings.TrimSpace(m.License) == "" || len(m.License) > 256 {
		return fmt.Errorf("package name and license are required")
	}
	layout, ok := packageLayouts[m.Architecture]
	if !ok {
		return fmt.Errorf("unsupported package architecture %q", m.Architecture)
	}
	if m.Runtime.Adapter != layout.adapter || !validPackageComponent(m.Runtime.Revision) || slices.Contains([]string{"main", "master", "latest"}, m.Runtime.Revision) {
		return fmt.Errorf("a pinned %s runtime revision is required", layout.adapter)
	}
	if len(m.Capabilities) == 0 || len(m.Capabilities) > len(layout.capabilities) {
		return fmt.Errorf("invalid capabilities for %s", m.Architecture)
	}
	seenCaps := map[ModelCapability]bool{}
	for _, c := range m.Capabilities {
		if seenCaps[c] || !slices.Contains(layout.capabilities, c) {
			return fmt.Errorf("unsupported or duplicate capability %q", c)
		}
		seenCaps[c] = true
	}
	if !seenCaps[layout.capabilities[0]] {
		return fmt.Errorf("missing primary capability %q", layout.capabilities[0])
	}
	if len(m.Languages) == 0 || len(m.Languages) > 256 {
		return fmt.Errorf("supported languages are required")
	}
	langs := map[string]bool{}
	for _, l := range m.Languages {
		if !packageLanguage.MatchString(l) || langs[l] {
			return fmt.Errorf("invalid or duplicate language %q", l)
		}
		langs[l] = true
	}
	if len(m.SampleRates) == 0 || len(m.SampleRates) > 16 {
		return fmt.Errorf("sample rates are required")
	}
	rates := map[int]bool{}
	for _, rate := range m.SampleRates {
		if rate < 8000 || rate > 192000 || rates[rate] {
			return fmt.Errorf("invalid or duplicate sample rate")
		}
		rates[rate] = true
	}
	if len(m.Voices) > 1024 {
		return fmt.Errorf("too many voices")
	}
	voices := map[string]bool{}
	for _, voice := range m.Voices {
		if voice.ID == "" || len(voice.ID) > 128 || voices[voice.ID] || !langs[voice.Language] {
			return fmt.Errorf("invalid voice or unsupported voice language")
		}
		voices[voice.ID] = true
	}
	if seenCaps[CapabilitySynthesis] && len(m.Voices) == 0 {
		return fmt.Errorf("synthesis package must declare voices")
	}
	if len(m.Artifacts) == 0 || len(m.Artifacts) > MaxPackageArtifacts {
		return fmt.Errorf("invalid artifact count")
	}
	paths, roles := map[string]bool{}, map[string]int{}
	var total int64
	for _, a := range m.Artifacts {
		p := strings.ToLower(a.Path)
		if !validArtifactPath(a.Path) || p == "manifest.json" || p == "installation.json" || paths[p] {
			return fmt.Errorf("unsafe or duplicate artifact path %q", a.Path)
		}
		paths[p] = true
		if a.Size <= 0 || a.Size > MaxPackageBytes-total || !packageDigest.MatchString(a.SHA256) || strings.TrimSpace(a.License) == "" || len(a.License) > 256 {
			return fmt.Errorf("invalid size, digest, or license for %q", a.Path)
		}
		total += a.Size
		allowed, required := layout.required[a.Role]
		if !required {
			switch a.Role {
			case "license":
				allowed = []string{".txt", ".md", ""}
			case "phonemizer_data":
				allowed = []string{".bin", ".dat", ".txt", ""}
			case "config":
				allowed = []string{".json"}
			case "tokenizer":
				allowed = []string{".json", ".txt", ".model", ".merges", ".vocab"}
			case "codec":
				allowed = []string{".json", ".txt", ".model", ".bin", ".safetensors"}
			default:
				return fmt.Errorf("unsupported artifact role %q", a.Role)
			}
		}
		if !slices.Contains(allowed, strings.ToLower(path.Ext(a.Path))) {
			return fmt.Errorf("unsupported artifact extension for %q", a.Role)
		}
		roles[a.Role]++
		if required && roles[a.Role] != 1 && !repeatablePackageRole(m.Architecture, a.Role) {
			return fmt.Errorf("duplicate artifact role %q", a.Role)
		}
		if a.Source != nil {
			u, err := url.Parse(a.Source.Repository)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(a.Source.Repository) > 2048 || !sourceRevision.MatchString(a.Source.Revision) || !validArtifactPath(a.Source.Path) {
				return fmt.Errorf("artifact source requires credential-free HTTPS and an immutable revision")
			}
		}
	}
	for role := range layout.required {
		if roles[role] != 1 && !repeatablePackageRole(m.Architecture, role) {
			return fmt.Errorf("missing required artifact role %q", role)
		}
	}
	// A path cannot simultaneously be an artifact and a directory.
	for p := range paths {
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			if paths[dir] {
				return fmt.Errorf("artifact directory conflict")
			}
		}
	}
	if len(m.HardwareProfiles) > 64 {
		return fmt.Errorf("too many hardware profiles")
	}
	for _, p := range m.HardwareProfiles {
		if !validPackageComponent(p.ID) || p.MemoryBytes <= 0 || strings.TrimSpace(p.Evidence) == "" || len(p.Evidence) > 1024 {
			return fmt.Errorf("invalid hardware evidence reference")
		}
	}
	return nil
}
