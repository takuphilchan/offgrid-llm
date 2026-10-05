package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func testPackageManifest(architecture string) ModelPackageManifest {
	l := packageLayouts[architecture]
	m := ModelPackageManifest{SchemaVersion: 1, ID: "test-" + architecture, Revision: "fixture-1", Name: "Synthetic test bytes, not an inference model", Architecture: architecture, Runtime: PackageRuntime{l.adapter, "fixture-1"}, Capabilities: append([]ModelCapability{}, l.capabilities...), Languages: []string{"en"}, SampleRates: []int{16000}, License: "CC0-1.0"}
	if architecture == "piper" || architecture == "kokoro" || architecture == "qwen3-tts" {
		m.Voices = []PackageVoice{{"default", "en"}}
	}
	for role, extensions := range l.required {
		data := []byte(role)
		digest := sha256.Sum256(data)
		m.Artifacts = append(m.Artifacts, PackageArtifact{Path: role + extensions[0], Role: role, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), License: "CC0-1.0"})
	}
	return m
}

func TestPackageManifestSupportedLayouts(t *testing.T) {
	for arch := range packageLayouts {
		t.Run(arch, func(t *testing.T) {
			m := testPackageManifest(arch)
			b, _ := json.Marshal(m)
			if _, err := DecodePackageManifest(strings.NewReader(string(b))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPackageManifestRejectsInvalidDeclarations(t *testing.T) {
	cases := map[string]func(*ModelPackageManifest){
		"version":              func(m *ModelPackageManifest) { m.SchemaVersion = 2 },
		"unknown architecture": func(m *ModelPackageManifest) { m.Architecture = "remote-code" },
		"runtime":              func(m *ModelPackageManifest) { m.Runtime.Adapter = "llama.cpp" },
		"unpinned runtime":     func(m *ModelPackageManifest) { m.Runtime.Revision = "main" },
		"unpinned model":       func(m *ModelPackageManifest) { m.Revision = "latest" },
		"chat capability":      func(m *ModelPackageManifest) { m.Capabilities = []ModelCapability{CapabilityChat} },
		"fake streaming":       func(m *ModelPackageManifest) { m.Capabilities = append(m.Capabilities, CapabilityStreamingRecognition) },
		"missing dependencies": func(m *ModelPackageManifest) { m.Artifacts = m.Artifacts[:1] },
		"digest":               func(m *ModelPackageManifest) { m.Artifacts[0].SHA256 = "bad" },
		"size":                 func(m *ModelPackageManifest) { m.Artifacts[0].Size = MaxPackageBytes + 1 },
		"license":              func(m *ModelPackageManifest) { m.Artifacts[0].License = "" },
		"voice":                func(m *ModelPackageManifest) { m.Voices[0].Language = "fr" },
		"rate":                 func(m *ModelPackageManifest) { m.SampleRates = []int{0} },
		"duplicate":            func(m *ModelPackageManifest) { m.Artifacts = append(m.Artifacts, m.Artifacts[0]) },
		"case collision": func(m *ModelPackageManifest) {
			a := m.Artifacts[0]
			a.Path = strings.ToUpper(a.Path)
			m.Artifacts = append(m.Artifacts, a)
		},
		"executable": func(m *ModelPackageManifest) { m.Artifacts[0].Path = "model.py" },
		"source": func(m *ModelPackageManifest) {
			m.Artifacts[0].Source = &ArtifactSource{"https://example.org/model", "main", "model.onnx"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := testPackageManifest("piper")
			mutate(&m)
			if m.Validate() == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestPackageManifestRejectsUnsafePaths(t *testing.T) {
	for _, p := range []string{"../model.bin", "/model.bin", `C:\model.bin`, `sub\model.bin`, "x/../../model.bin", "x//model.bin", "x/./model.bin", "x:stream.bin", "NUL.bin", "com1/model.bin", "model.bin.", " model.bin", "model\x00.bin", "manifest.json"} {
		t.Run(p, func(t *testing.T) {
			m := testPackageManifest("whisper")
			m.Artifacts[0].Path = p
			if m.Validate() == nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
}

func TestPackageManifestDecoderLimits(t *testing.T) {
	m := testPackageManifest("whisper")
	b, _ := json.Marshal(m)
	for _, data := range []string{string(b) + " {}", strings.Repeat(" ", MaxPackageManifestBytes+1), strings.TrimSuffix(string(b), "}") + `,"trust_remote_code":true}`} {
		if _, err := DecodePackageManifest(strings.NewReader(data)); err == nil {
			t.Fatal("accepted invalid JSON document")
		}
	}
}
