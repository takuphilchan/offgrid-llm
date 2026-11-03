package audio

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// Distinct model packages must share the installed family engine. Changing
// weights must change the data path and lease, never the executable selection.
func TestSpeechModelsShareFamilyRuntime(t *testing.T) {
	for _, family := range []string{"whisper", "piper"} {
		t.Run(family, func(t *testing.T) {
			store, root, ids := fixturePackages(t)
			states, err := store.List()
			if err != nil {
				t.Fatal(err)
			}
			var original models.ModelPackageManifest
			for _, state := range states {
				if state.ID == ids[family] {
					original = *state.Manifest
				}
			}
			second := original
			second.ID += "-second"
			second.Name += " second fixture"
			if _, err := store.Import(context.Background(), second, func(_ context.Context, a models.PackageArtifact) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(a.Role)), nil
			}); err != nil {
				t.Fatal(err)
			}
			legacy, err := NewEngine(Config{DataDir: t.TempDir(), WhisperPath: "shared-whisper", PiperPath: "shared-piper"})
			if err != nil {
				t.Fatal(err)
			}
			router := NewPackageRuntime(store, root, t.TempDir(), legacy)
			defer router.Close()
			capability := models.CapabilityTranscription
			if family == "piper" {
				capability = models.CapabilitySynthesis
			}
			for _, manifest := range []models.ModelPackageManifest{original, second} {
				err := router.withEngine(context.Background(), manifest.ID, capability, func(_ context.Context, engine *Engine, managed bool) error {
					if !managed || engine.whisperPath != "shared-whisper" || engine.piperPath != "shared-piper" {
						t.Fatal("model selection changed the shared runtime")
					}
					for _, artifact := range manifest.Artifacts {
						if artifact.Role != "weights" {
							continue
						}
						actual := engine.runtime.WhisperModelPath
						if family == "piper" {
							actual = engine.runtime.PiperModelPath
						}
						want := filepath.Join(root, models.PackageDirectory, manifest.ID, manifest.Revision, artifact.Path)
						if actual != want {
							t.Fatalf("wrong model path: %q, want %q", actual, want)
						}
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Remove(original.ID, original.Revision); err != nil {
				t.Fatalf("old model lease retained: %v", err)
			}
			if err := store.Remove(second.ID, second.Revision); err != models.ErrPackageInUse {
				t.Fatalf("selected model lost its lease: %v", err)
			}
		})
	}
}
