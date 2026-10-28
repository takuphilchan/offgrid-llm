package agents

import (
	"context"
	"testing"
)

func TestAgentAdapterPinSurvivesRestartWithoutChangingLegacyJobs(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	legacy, err := r.Create("fixture", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ToolAdapter = scopedAgentPin()
	pinned, err := r.Create("fixture", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	old, ok := restored.GetTask(legacy.ID)
	if !ok || old.Config.ToolAdapter != nil {
		t.Fatal("legacy job acquired an adapter")
	}
	current, ok := restored.GetTask(pinned.ID)
	if !ok || current.Config.ToolAdapter == nil || *current.Config.ToolAdapter != *cfg.ToolAdapter || len(current.Steps) != 0 {
		t.Fatal("pin lost or execution started")
	}
}
