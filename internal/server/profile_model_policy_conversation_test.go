package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestProfileModelPolicyConversationPinsFinalResponseContext(t *testing.T) {
	for _, tc := range []struct {
		name, response                          string
		defaultEvidence, chatEvidence, admitted bool
	}{
		{"default interactive only", "", false, true, true},
		{"interactive only", "interactive", false, true, true},
		{"extended", "extended", true, false, true},
		{"interactive unqualified", "interactive", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, targets, _ := localPolicyFixture(t)
			s.config.MaxContextSize, s.config.ChatContextSize, s.config.AdaptiveContext = 16384, 4096, false
			defaultWindow := s.modelContextWindow("fallback")
			chatWindow := s.chatContextWindowForModel("interactive", "fallback")
			if defaultWindow == chatWindow {
				t.Fatal("fixture requires differing agent and conversation windows")
			}
			store := models.NewCapabilityEvidenceStore(t.TempDir())
			qualified := map[int]bool{defaultWindow: tc.defaultEvidence, chatWindow: tc.chatEvidence}
			lookup := func(_ context.Context, id string, window int) (*models.CapabilityEvidenceTarget, error) {
				if !qualified[window] || targets[id] == nil {
					return nil, nil
				}
				target := *targets[id]
				target.Fingerprint.ContextWindow = window
				return &target, nil
			}
			for _, id := range []string{"model", "fallback"} {
				for window, enabled := range qualified {
					if !enabled {
						continue
					}
					target, _ := lookup(context.Background(), id, window)
					q := &models.WorkflowQualification{SuiteSHA256: target.SuiteSHA256, Platform: target.Platform, ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 90}
					if _, err := store.Record(context.Background(), models.CapabilityEvidence{Version: 1, ModelID: id, CheckID: inference.NativeAgentToolAdapter, Fingerprint: target.Fingerprint, Outcome: "passed", Code: "fixture_passed", RecordedAt: time.Now(), Qualification: q}); err != nil {
						t.Fatal(err)
					}
				}
			}
			policy, err := inference.NewAgentToolPolicy(store, lookup, lookup)
			if err != nil {
				t.Fatal(err)
			}
			s.agentToolPolicy = policy
			s.sessionHandlers = NewSessionHandlers(t.TempDir())
			s.sessionHandlers.agentServer = s
			if _, err := s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).Create("chat", "model"); err != nil {
				t.Fatal(err)
			}
			profile, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Measured conversation", Model: "model", MaxIterations: 8, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided", FallbackModels: []string{"fallback"}}})
			if err != nil {
				t.Fatal(err)
			}
			// Stop at the real durable admission boundary: no model or native
			// action is invoked, and the persisted config is inspected directly.
			s.sessionHandlers.admissionFault = func(stage string) error {
				if stage == "intent_saved" {
					return errors.New("isolated admission boundary")
				}
				return nil
			}
			f := &conversationFixture{s: s}
			req := sessionGenerateRequest{Content: "Answer without acting", RequestID: "measured-conversation-1", ModelID: "fallback", Profile: tc.response, Agent: &sessions.AgentSelection{ID: profile.ID, Revision: profile.Revision}}
			response := f.submit(req)
			intent, err := s.agentManager.ConversationIntent(s.workspaceID, "local-admin", conversationRequestID(s.workspaceID, req.RequestID))
			if err != nil {
				t.Fatal(err)
			}
			if !tc.admitted {
				if response.Code != 422 || intent != nil {
					t.Fatal("unqualified response context admitted", response.Code, response.Body.String(), intent)
				}
				return
			}
			if intent == nil {
				t.Fatal("qualified response context rejected", response.Code, response.Body.String())
			}
			config := intent.Config
			window := s.chatContextWindowForModel(tc.response, "fallback")
			if config.ContextWindow != window || config.ModelSelection == nil || config.ToolAdapter == nil {
				t.Fatal("lost final context selection", config)
			}
			for _, candidate := range config.ModelSelection.Candidates {
				if candidate.ContextWindow != window || candidate.ToolAdapter == nil {
					t.Fatal("candidate pin predates response context", candidate, window)
				}
				if err := policy.Validate(context.Background(), candidate.Model, window, candidate.ToolAdapter); err != nil {
					t.Fatal("candidate pin does not match final context", err)
				}
				if candidate.Model == "fallback" && *candidate.ToolAdapter != *config.ToolAdapter {
					t.Fatal("selected adapter diverges from candidate")
				}
			}
		})
	}
}
