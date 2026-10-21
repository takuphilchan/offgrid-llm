package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestLanguagePackagesDoNotRequireSpeechRuntimeForInventory(t *testing.T) {
	language := models.PackageState{ID: "offline-mlx", Manifest: &models.ModelPackageManifest{Capabilities: []models.ModelCapability{models.CapabilityChat}}}
	if hasSpeechPackage([]models.PackageState{language}) {
		t.Fatal("language-only package triggered speech runtime discovery")
	}
	for _, capability := range []models.ModelCapability{models.CapabilityTranscription, models.CapabilitySynthesis} {
		speech := models.PackageState{ID: "speech", Manifest: &models.ModelPackageManifest{Capabilities: []models.ModelCapability{capability}}}
		if !hasSpeechPackage([]models.PackageState{language, speech}) {
			t.Fatalf("speech package with %q did not trigger runtime discovery", capability)
		}
	}
}

func TestInstalledLanguagePackageInventoryDoesNotOpenSpeechRuntime(t *testing.T) {
	s, _, _ := newOperationFixture(t)
	source := t.TempDir()
	manifest := models.ModelPackageManifest{
		SchemaVersion: 1, ID: "offline-mlx", Revision: "fixture-1", Name: "Offline MLX fixture", Architecture: "mlx-text",
		ModelFamily: "qwen2.5", Runtime: models.PackageRuntime{Adapter: "mlx-lm", Revision: "fixture-1"},
		Capabilities: []models.ModelCapability{models.CapabilityChat}, Languages: []string{"en"}, License: "CC0-1.0",
		Engine: models.EngineMLX, Format: models.FormatMLXSafetensors, Backend: "metal", RunnerRevision: "fixture-1", AdapterRevision: "fixture-1", ContextLength: 4096,
	}
	for _, artifact := range []struct{ path, role string }{{"weights.safetensors", "weights"}, {"config.json", "config"}, {"tokenizer.json", "tokenizer"}} {
		data := []byte(artifact.role)
		if err := os.WriteFile(filepath.Join(source, artifact.path), data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		manifest.Artifacts = append(manifest.Artifacts, models.PackageArtifact{Path: artifact.path, Role: artifact.role, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), License: "CC0-1.0"})
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "manifest.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.registry.Packages().ImportDirectory(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	response := modelRequest(t, s, "GET", "/api/v2/models?category=language", nil)
	if response.Code != 200 {
		t.Fatalf("language inventory required an unrelated speech runtime: %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Models []models.InstalledModel `json:"models"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 1 || result.Models[0].ID != manifest.ID || result.Models[0].Readiness.RuntimeReady || result.Models[0].Readiness.Qualified {
		t.Fatalf("native language package was lost or falsely qualified: %#v", result.Models)
	}
}
