package agents

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestProfileModelPolicyImmutableHistoryAndIsolation(t *testing.T) {
	directory := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	settings := ProfileSettings{Name: "Local assistant", Model: "primary", MaxIterations: 30}
	old, err := m.SaveProfile("work", "alice", "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(old.Settings)
	if strings.Contains(string(encoded), "model_policy") {
		t.Fatal("old digest changed")
	}
	settings.ModelPolicy = &ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"fallback"}, PerceptionModel: "vision"}
	updated, err := m.SaveProfile("work", "alice", old.ID, 1, settings)
	if err != nil || updated.Revision != 2 || updated.Digest == old.Digest {
		t.Fatal(updated, err)
	}
	settings.ModelPolicy.FallbackModels[0] = "external-mutation"
	updated.Settings.ModelPolicy.Mode = "changed"
	restored := NewManagerWithPersistence(nil, nil, nil, directory)
	previous, err := restored.GetProfile("work", "alice", old.ID, 1)
	if err != nil || previous.Settings.ModelPolicy != nil || previous.Digest != old.Digest {
		t.Fatal("legacy broadened", previous, err)
	}
	current, err := restored.GetProfile("work", "alice", old.ID, 2)
	if err != nil || current.Settings.ModelPolicy.Mode != "adaptive" || current.Settings.ModelPolicy.FallbackModels[0] != "fallback" {
		t.Fatal("policy not immutable", current, err)
	}
	if _, err = restored.GetProfile("work", "bob", old.ID, 2); !errors.Is(err, ErrProfileNotFound) {
		t.Fatal("policy crossed actor boundary", err)
	}
}

func TestRunAdmissionDetachesProfilePolicyAndCapabilities(t *testing.T) {
	config := DefaultAgentConfig()
	config.Profile = &AgentProfile{Settings: ProfileSettings{Model: "model", ModelPolicy: &ProfileModelPolicy{Version: 1, Mode: "guided", FallbackModels: []string{"fallback"}}}}
	config.AllowedCapabilities = map[string]capabilities.Descriptor{"read": {Name: "read", Kind: capabilities.Read}}
	var empty []string
	var noTools []ProfileToolReference
	config.KnowledgeSelection = &empty
	config.ConversationConfiguration = &ConversationConfiguration{Restrictions: ResourceRestrictions{Tools: &noTools, Knowledge: &empty}}
	task, err := prepareConversationRun("Request", "model", "alice", config, nil)
	if err != nil {
		t.Fatal(err)
	}
	config.Profile.Settings.ModelPolicy.FallbackModels[0] = "unreviewed"
	config.AllowedCapabilities["write"] = capabilities.Descriptor{Name: "write", Kind: capabilities.Write}
	if task.Config.Profile.Settings.ModelPolicy.FallbackModels[0] != "fallback" || len(task.Config.AllowedCapabilities) != 1 {
		t.Fatal("caller mutated admitted authority/preferences")
	}
	if task.Config.KnowledgeSelection == nil || task.Config.ConversationConfiguration.Restrictions.Tools == nil || task.Config.ConversationConfiguration.Restrictions.Knowledge == nil || *task.Config.KnowledgeSelection == nil || *config.KnowledgeSelection != nil {
		t.Fatal("explicit empty ceiling broadened or caller mutated")
	}
	data, _ := json.Marshal(task)
	var restored Task
	if json.Unmarshal(data, &restored) != nil || restored.Config.KnowledgeSelection == nil || restored.Config.ConversationConfiguration.Restrictions.Tools == nil {
		t.Fatal("persisted empty ceiling lost")
	}
}

func TestProfileModelPolicyRejectsInvalidPools(t *testing.T) {
	for _, policy := range []*ProfileModelPolicy{
		{Version: 2, Mode: "guided"}, {Version: 1, Mode: "automatic"},
		{Version: 1, Mode: "guided", FallbackModels: []string{"primary"}},
		{Version: 1, Mode: "guided", FallbackModels: []string{"a", "a"}},
		{Version: 1, Mode: "guided", FallbackModels: []string{"a", "b", "c"}},
		{Version: 1, Mode: "guided", FallbackModels: []string{"endpoint:local@revision"}},
		{Version: 1, Mode: "guided", PerceptionModel: "endpoint:vision@revision"},
		{Version: 1, Mode: "guided", FallbackModels: []string{"bad\nreference"}},
	} {
		if validateProfileSettings(ProfileSettings{Name: "Test", Model: "primary", MaxIterations: 30, ModelPolicy: policy}) == nil {
			t.Fatalf("invalid pool accepted: %+v", policy)
		}
	}
	var omitted *ProfileModelPolicy
	if !omitted.Allows("primary", "primary") || omitted.Allows("primary", "other") {
		t.Fatal("nil policy broadened legacy selection")
	}
}

func TestProfileModelSelectionSurvivesRestartWithoutDispatch(t *testing.T) {
	directory := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	r := NewRunner(m, NewToolRegistry(), nil)
	config := DefaultAgentConfig()
	config.ModelSelection = &ModelPolicySelection{Version: 1, Mode: "guided", Primary: "primary", Selected: "fallback", Reason: "explicit_qualified_fallback", Candidates: []ModelCandidate{{Model: "fallback", ContextWindow: 4096, ToolAdapter: &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, EvidenceID: strings.Repeat("a", 64), FingerprintID: strings.Repeat("b", 64)}}}}
	created, _, err := r.CreateRequest("Inspect only", "fallback", "alice", config, "policy-restart", "digest")
	if err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, directory)
	saved, found := restored.GetTask(created.ID)
	if !found || saved.Status != TaskInterrupted || saved.Config.ModelSelection == nil || saved.Config.ModelSelection.Selected != "fallback" || saved.Config.ModelSelection.Candidates[0].ToolAdapter.EvidenceID != strings.Repeat("a", 64) || saved.Checkpoint.BudgetedModelCalls != 0 {
		t.Fatal("restart changed pins or dispatched inference", saved)
	}
}
