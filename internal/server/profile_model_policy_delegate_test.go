package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestDelegateModelPolicyPinsRequestedContext(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		evidenceWindow int
		pool, admitted bool
	}{
		{"interactive pool", "interactive", 4096, true, true},
		{"extended pool", "extended", 8192, true, true},
		{"wrong window", "interactive", 8192, true, false},
		{"native without pool", "interactive", 4096, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, targets, _ := localPolicyFixture(t)
			s.config.MaxContextSize, s.config.ChatContextSize, s.config.AdaptiveContext = 16384, 4096, false
			store := models.NewCapabilityEvidenceStore(t.TempDir())
			lookup := func(_ context.Context, id string, window int) (*models.CapabilityEvidenceTarget, error) {
				if window != tc.evidenceWindow || targets[id] == nil {
					return nil, nil
				}
				target := *targets[id]
				target.Fingerprint.ContextWindow = window
				return &target, nil
			}
			for _, id := range []string{"model", "fallback"} {
				target, _ := lookup(context.Background(), id, tc.evidenceWindow)
				q := &models.WorkflowQualification{SuiteSHA256: target.SuiteSHA256, Platform: target.Platform, ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 90}
				if _, err := store.Record(context.Background(), models.CapabilityEvidence{Version: 1, ModelID: id, CheckID: inference.NativeAgentToolAdapter, Fingerprint: target.Fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
					t.Fatal(err)
				}
			}
			policy, err := inference.NewAgentToolPolicy(store, lookup, lookup)
			if err != nil {
				t.Fatal(err)
			}
			s.agentToolPolicy = policy
			request := httptest.NewRequest("POST", "/api/v2/jobs", nil)
			settings := agents.ProfileSettings{Name: "Child", Model: "fallback", MaxIterations: 8}
			if tc.pool {
				settings.ModelPolicy = &agents.ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"model"}}
			}
			child, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, settings)
			if err != nil {
				t.Fatal(err)
			}
			parent, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, agents.ProfileSettings{Name: "Parent", Model: "model", MaxIterations: 16, Delegates: []string{child.ID}})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			req := jobSubmitRequest{Prompt: "Compare supplied facts", ProfileID: parent.ID, ProfileRevision: parent.Revision, ResponseProfile: tc.response}
			config, ok := s.resolveJobConfig(w, request, &req)
			if !tc.admitted {
				if ok || w.Code != 422 {
					t.Fatal("unqualified child admitted", ok, w.Code, w.Body.String())
				}
				return
			}
			if !ok {
				t.Fatal(w.Code, w.Body.String())
			}
			pinned := config.DelegateProfiles[child.ID]
			window := s.chatContextWindowForModel(tc.response, "fallback")
			if pinned.ContextWindow != window || pinned.ToolAdapter == nil {
				t.Fatal("child lost window or adapter", pinned.ContextWindow, window, pinned.ToolAdapter)
			}
			if err := policy.Validate(context.Background(), "fallback", window, pinned.ToolAdapter); err != nil {
				t.Fatal(err)
			}
			if (pinned.ModelSelection != nil) != tc.pool {
				t.Fatal("child policy changed")
			}
			if tc.pool {
				for _, candidate := range pinned.ModelSelection.Candidates {
					if candidate.ContextWindow != window {
						t.Fatal("candidate uses default context", candidate)
					}
					if err := policy.Validate(context.Background(), candidate.Model, window, candidate.ToolAdapter); err != nil {
						t.Fatal(err)
					}
				}
				task, err := s.agentRunner.Create(req.Prompt, req.Model, s.agentActor(request), config)
				if err != nil {
					t.Fatal(err)
				}
				pinned.ModelSelection.Candidates[0].Model = "mutated"
				saved, _ := s.agentManager.GetTask(task.ID)
				if saved.Config.DelegateProfiles[child.ID].ModelSelection.Candidates[0].Model != "fallback" {
					t.Fatal("saved delegate aliases admission caller")
				}
			}
		})
	}
}
