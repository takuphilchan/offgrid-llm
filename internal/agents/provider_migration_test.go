package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Provider metadata is additive: old clients, profiles, and jobs continue to
// identify the same model while the legacy file inventory resolves its engine.
func TestLegacyModelProfileAndTaskSurviveProviderMetadataMigration(t *testing.T) {
	root := t.TempDir()
	modelRoot := filepath.Join(root, "models")
	if err := os.MkdirAll(modelRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelRoot, "offline.gguf"), []byte("legacy fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(modelRoot)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.GetChatModel("offline"); err != nil {
		t.Fatalf("legacy model stopped being selectable: %v", err)
	}
	listed := registry.ListModels()
	if len(listed) != 1 || listed[0].ID != "offline" || listed[0].Engine != models.EngineLlamaCPP || listed[0].Format != models.FormatGGUF {
		t.Fatalf("legacy model provider projection changed: %#v", listed)
	}

	dataRoot := filepath.Join(root, "data")
	manager := NewManagerWithPersistence(nil, nil, nil, dataRoot)
	profile, err := manager.SaveProfile("workspace", "alice", "", 0, ProfileSettings{Name: "Legacy", Model: "offline", MaxIterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultAgentConfig()
	config.Profile = profile
	task, err := NewRunner(manager, nil, nil).Create("Summarize this", "offline", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	manager = NewManagerWithPersistence(nil, nil, nil, dataRoot)
	if err := manager.StorageError(); err != nil {
		t.Fatal(err)
	}
	restoredProfile, err := manager.GetProfile("workspace", "alice", profile.ID, profile.Revision)
	if err != nil || restoredProfile.Digest != profile.Digest || restoredProfile.Settings.Model != "offline" || len(restoredProfile.Settings.Delegates) != 0 {
		t.Fatalf("profile revision or authority changed: %#v, %v", restoredProfile, err)
	}
	restoredTask, ok := manager.GetTask(task.ID)
	if !ok || restoredTask.Model != "offline" || restoredTask.Config.Profile == nil || restoredTask.Config.Profile.Digest != profile.Digest {
		t.Fatalf("saved task lost its pinned model/profile: %#v", restoredTask)
	}

	var oldClientModel api.Model
	if err := json.Unmarshal([]byte(`{"id":"offline","object":"model","created":1,"owned_by":"offgrid-llm"}`), &oldClientModel); err != nil {
		t.Fatal(err)
	}
	if oldClientModel.ID != "offline" || oldClientModel.Engine != "" {
		t.Fatalf("legacy API response changed semantics: %#v", oldClientModel)
	}
}
