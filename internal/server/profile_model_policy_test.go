package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Synthetic evidence stays inside this disposable fixture. These records are
// not hardware/model qualification and are never installed in user state.
func localPolicyFixture(t *testing.T) (*Server, map[string]*models.CapabilityEvidenceTarget, *models.CapabilityEvidenceStore) {
	t.Helper()
	s := profileFixture(t)
	metadata := map[string]*api.ModelMetadata{}
	for _, id := range []string{"model", "fallback", "vision"} {
		metadata[id] = &api.ModelMetadata{ID: id, Type: "llm", ContextSize: 8192}
	}
	metadata["vision"].ProjectorPath = "isolated-fixture-projector"
	data, _ := json.Marshal(metadata)
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.LoadRegistry(path); err != nil {
		t.Fatal(err)
	}
	store := models.NewCapabilityEvidenceStore(t.TempDir())
	targets := map[string]*models.CapabilityEvidenceTarget{}
	for _, id := range []string{"model", "fallback", "vision"} {
		f := models.CapabilityFingerprint{ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), RuntimeSHA256: strings.Repeat("e", 64), RuntimeConfigSHA256: strings.Repeat("f", 64), Quantization: "fixture", RuntimeBuild: "fixture", Adapter: inference.NativeAgentToolAdapter, ContextWindow: s.modelContextWindow(id), Backend: "cpu", HardwareID: "fixture"}
		if id == "vision" {
			f.ProjectorSHA256, f.PreprocessSHA256 = strings.Repeat("1", 64), strings.Repeat("2", 64)
		}
		q := &models.WorkflowQualification{SuiteSHA256: strings.Repeat("c", 64), Platform: "fixture", ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 90}
		targets[id] = &models.CapabilityEvidenceTarget{Fingerprint: f, SuiteSHA256: q.SuiteSHA256, Platform: q.Platform}
		if _, err := store.Record(context.Background(), models.CapabilityEvidence{Version: 1, ModelID: id, CheckID: inference.NativeAgentToolAdapter, Fingerprint: f, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
			t.Fatal(err)
		}
	}
	lookup := func(_ context.Context, id string, _ int) (*models.CapabilityEvidenceTarget, error) {
		return targets[id], nil
	}
	policy, err := inference.NewAgentToolPolicy(store, lookup, func(context.Context, string, int) (*models.CapabilityEvidenceTarget, error) {
		t.Error("admission loaded or inspected a live runtime")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s.agentToolPolicy = policy
	return s, targets, store
}

func TestProfileModelPolicyAdmissionKeepsPrimaryAndPinsExplicitFallback(t *testing.T) {
	s, _, _ := localPolicyFixture(t)
	settings := agents.ProfileSettings{Name: "Measured pool", Model: "model", MaxIterations: 30, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"fallback"}, PerceptionModel: "vision"}}
	legacy := settings
	legacy.ModelPolicy = nil
	request := httptest.NewRequest("POST", "/api/v2/jobs", nil)
	legacyProfile, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, legacy)
	if err != nil {
		t.Fatal(err)
	}
	baseline, ok := s.resolveJobConfig(httptest.NewRecorder(), request, &jobSubmitRequest{Prompt: "Answer the question", ProfileID: legacyProfile.ID, ProfileRevision: 1})
	if !ok {
		t.Fatal("baseline profile did not admit")
	}
	w := profileHTTP(s, "POST", "/api/v2/agent-profiles", map[string]any{"settings": settings})
	var profile agents.AgentProfile
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &profile) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, selected := range []string{"", "fallback"} {
		req := jobSubmitRequest{Prompt: "Answer the question", ProfileID: profile.ID, ProfileRevision: 1, Model: selected}
		w := httptest.NewRecorder()
		config, ok := s.resolveJobConfig(w, httptest.NewRequest("POST", "/api/v2/jobs", nil), &req)
		if !ok {
			t.Fatal(w.Code, w.Body.String())
		}
		want := selected
		if want == "" {
			want = "model"
		}
		if req.Model != want || config.ModelSelection == nil || config.ModelSelection.Selected != want || config.ModelSelection.Primary != "model" || config.ModelSelection.Perception == nil || config.ToolAdapter == nil || !reflect.DeepEqual(config.AllowedCapabilities, baseline.AllowedCapabilities) || config.MaxIterations != 30 {
			t.Fatal("selection changed authority or ignored evidence", req, config)
		}
		if selected == "fallback" && config.ModelSelection.Reason != "explicit_qualified_fallback" {
			t.Fatal("selection reason missing")
		}
		task, _, err := s.agentRunner.CreateRequest(req.Prompt, req.Model, s.agentActor(httptest.NewRequest("GET", "/", nil)), config, "model-policy-"+want, "digest-"+want)
		if err != nil {
			t.Fatal(err)
		}
		config.ModelSelection.Candidates[0].Model = "mutated"
		saved, found := s.agentManager.GetTask(task.ID)
		if !found || saved.Config.ModelSelection.Candidates[0].Model != "model" {
			t.Fatal("durable selection aliases caller")
		}
	}
	changed := profile.Settings
	changed.ModelPolicy = nil
	if _, err := s.agentManager.SaveProfile(s.workspaceID, profile.Actor, profile.ID, 1, changed); err != nil {
		t.Fatal(err)
	}
	old, err := s.agentManager.GetProfile(s.workspaceID, profile.Actor, profile.ID, 1)
	if err != nil || old.Settings.ModelPolicy == nil {
		t.Fatal("edited old policy", err)
	}
}

func TestProfileModelPolicyRejectsUnavailableStaleAndTextOnlyPerception(t *testing.T) {
	s, targets, _ := localPolicyFixture(t)
	settings := agents.ProfileSettings{Name: "Pool", Model: "model", MaxIterations: 30, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided", FallbackModels: []string{"fallback"}}}
	check := func(want string) {
		t.Helper()
		_, issue := s.resolveProfileModelPolicy(context.Background(), settings, "model", s.modelContextWindow)
		if issue == nil || issue.Code != want {
			t.Fatal("wrong readiness", issue, want)
		}
	}
	targets["fallback"].Fingerprint.ModelSHA256 = strings.Repeat("3", 64)
	check("local_model_unqualified")
	settings.ModelPolicy.FallbackModels = []string{"missing"}
	check("local_model_unavailable")
	settings.ModelPolicy.FallbackModels = nil
	settings.ModelPolicy.PerceptionModel = "model"
	check("perception_model_unavailable")
	settings.ModelPolicy.PerceptionModel = "vision"
	targets["vision"].Fingerprint.ProjectorSHA256 = ""
	check("perception_model_unqualified")
	settings.ModelPolicy.PerceptionModel = ""
	s.agentToolPolicy = nil
	if selected, issue := s.resolveProfileModelPolicy(context.Background(), settings, "model", s.modelContextWindow); issue != nil || selected.Candidates[0].ToolAdapter != nil {
		t.Fatal("guided primary lost legacy path", selected, issue)
	}
	settings.ModelPolicy.Mode = "adaptive"
	check("local_model_unqualified")
}

func TestProfileModelPolicyLegacyUnknownFieldsAndExplicitOverride(t *testing.T) {
	s := profileFixture(t)
	w := profileHTTP(s, "POST", "/api/v2/agent-profiles", map[string]any{"settings": map[string]any{"name": "Policy", "model": "model", "max_iterations": 30, "model_policy": map[string]any{"version": 1, "mode": "guided", "grant": true}}})
	if w.Code != 400 {
		t.Fatal("unknown authority field accepted", w.Code)
	}
	config := agents.DefaultAgentConfig()
	config.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Model: "model", ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided"}}}
	if !s.pinJobModelPolicy(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), &jobSubmitRequest{Model: "explicit-other", ConversationModelOverride: true}, &config) || config.ModelSelection != nil {
		t.Fatal("manual override inherited another model's pool")
	}
}

func TestProfileModelPolicyExplicitChoiceCannotInventFallback(t *testing.T) {
	s, _, _ := localPolicyFixture(t)
	r := httptest.NewRequest("POST", "/api/v2/jobs", nil)
	settings := agents.ProfileSettings{Name: "Local", Model: "model", MaxIterations: 30, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided", FallbackModels: []string{"fallback"}}}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(r), "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		req := jobSubmitRequest{Prompt: "Answer", Model: "vision", ProfileID: profile.ID, ProfileRevision: 1, ConversationModelOverride: override}
		w := httptest.NewRecorder()
		config, ok := s.resolveJobConfig(w, r, &req)
		if override {
			if !ok || config.ModelSelection != nil || req.Model != "vision" {
				t.Fatal("explicit conversation choice changed", w.Code, w.Body.String())
			}
		} else if ok || w.Code != 409 {
			t.Fatal("unconfigured fallback admitted", w.Code, w.Body.String())
		}
	}
}

func TestProfileModelPolicySelectedFallbackRevalidatesBeforeInference(t *testing.T) {
	s, targets, _ := localPolicyFixture(t)
	r := httptest.NewRequest("POST", "/api/v2/jobs", nil)
	settings := agents.ProfileSettings{Name: "Local", Model: "model", MaxIterations: 30, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided", FallbackModels: []string{"fallback"}}}
	profile, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(r), "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	config, ok := s.resolveJobConfig(w, r, &jobSubmitRequest{Prompt: "Answer", Model: "fallback", ProfileID: profile.ID, ProfileRevision: 1})
	if !ok {
		t.Fatal(w.Code, w.Body.String())
	}
	targets["fallback"].Fingerprint.RuntimeConfigSHA256 = strings.Repeat("3", 64)
	request := &api.ChatCompletionRequest{Model: "fallback"}
	if err := s.configureAgentRequest(context.Background(), request, &agents.Task{Model: "fallback", Config: config}); err == nil {
		t.Fatal("stale selected candidate could infer")
	}
}
