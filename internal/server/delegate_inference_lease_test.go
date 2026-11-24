package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDelegateInferenceRevalidatesBeforeCounting(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "qualified", true: "changed_evidence"}[revoke], func(t *testing.T) {
			s, targets, store := localPolicyFixture(t)
			dir := t.TempDir()
			for _, id := range []string{"model", "fallback"} {
				if err := os.WriteFile(filepath.Join(dir, id+".gguf"), []byte("isolated fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s.registry = models.NewRegistry(dir)
			if err := s.registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			lookup := func(_ context.Context, id string, _ int) (*models.CapabilityEvidenceTarget, error) {
				return targets[id], nil
			}
			var err error
			s.agentToolPolicy, err = inference.NewAgentToolPolicy(store, lookup, lookup)
			if err != nil {
				t.Fatal(err)
			}
			process, engine := &pinnedContextProcess{}, &switchEngineFixture{}
			s.llamaProcess, s.engine, s.inferenceLifecycle = process, engine, inference.NewLifecycleGate(1)
			request := httptest.NewRequest("POST", "/api/v2/jobs", nil)
			child, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, agents.ProfileSettings{Name: "Worker", Model: "fallback", MaxIterations: 8, ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "adaptive", FallbackModels: []string{"model"}}})
			if err != nil {
				t.Fatal(err)
			}
			parent, err := s.agentManager.SaveProfile(s.workspaceID, s.agentActor(request), "", 0, agents.ProfileSettings{Name: "Parent", Model: "model", MaxIterations: 16, Delegates: []string{child.ID}})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			req := jobSubmitRequest{Prompt: "Compare facts", ProfileID: parent.ID, ProfileRevision: parent.Revision}
			cfg, ok := s.resolveJobConfig(w, request, &req)
			if !ok {
				t.Fatal(w.Code, w.Body.String())
			}
			runner := agents.NewRunner(s.agentManager, s.toolRegistry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				if task.ParentID != "" {
					return s.callAgentModel(ctx, task, messages, tools)
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"review","goal":"Compare facts","depends_on":[],"tools":[],"profile_id":"` + child.ID + `"}]}`}}}}}}}, nil
			})
			runner.ContextWindow, runner.CountContext = s.taskContextWindow, s.countAgentContext
			root, err := runner.Create(req.Prompt, req.Model, s.agentActor(request), cfg)
			if err != nil {
				t.Fatal(err)
			}
			root, err = runner.Continue(context.Background(), root.ID, root.Actor, "start", "", false)
			if err != nil || root.Status != agents.TaskChildren || len(root.ChildHistory) != 1 {
				t.Fatalf("delegate: %+v %v", root, err)
			}
			if revoke {
				targets["fallback"].Fingerprint.RuntimeBuild = "changed after admission"
			}
			worker, err := runner.Continue(context.Background(), root.ChildHistory[0].ID, root.Actor, "start", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if revoke {
				if worker.Status != agents.TaskFailed || len(engine.generated) != 0 || !reflect.DeepEqual(engine.counted, []string{"model"}) {
					t.Fatal("revoked child reached inference", worker.Status, engine.counted, engine.generated)
				}
			} else if worker.Status != agents.TaskCompleted || !reflect.DeepEqual(engine.generated, []string{"fallback"}) || !reflect.DeepEqual(engine.counted, []string{"model", "fallback"}) {
				t.Fatal("qualified child bypassed or failed shared lease", worker.Status, engine.counted, engine.generated)
			}
		})
	}
}
