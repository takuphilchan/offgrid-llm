package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type lostResultFixture struct {
	*agents.ToolRegistry
	calls int
}

func (f *lostResultFixture) Authorize(context.Context, string, json.RawMessage, agents.ToolExecution) error {
	return nil
}
func (f *lostResultFixture) ExecuteWithPolicy(context.Context, string, json.RawMessage, agents.ToolExecution) (string, error) {
	f.calls++
	return "", errors.New("fixture lost dispatched result")
}

func TestReconciliationAPIPreservesHumanReportWithoutVerifyingChecks(t *testing.T) {
	s := profileFixture(t)
	tools := &lostResultFixture{ToolRegistry: agents.NewToolRegistry()}
	const args = `{"path":"fixture-only","content":"exact requested text"}`
	s.agentRunner = agents.NewRunner(s.agentManager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "model-call", Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: args}}}}}}}, nil
	})
	// This deterministic test covers result provenance, not model context fit.
	s.agentRunner.ContextWindow = func(*agents.Task) int { return 32768 }
	config := agents.DefaultAgentConfig()
	config.TaskFirst, config.NativeWorkflowVersion = true, agents.DocumentWorkflowEvidenceVersion
	config.ComputerSession, config.ComputerScope, config.ComputerDriver = "synthetic-session", "desktop", "windows-uia"
	config.NativeExpectations = []agents.OutcomeExpectation{{ID: "open", Title: "Open editor", Kind: "application_open", ApplicationID: "fixture-editor"}}
	actor := s.agentActor(httptest.NewRequest("GET", "/", nil))
	task, err := s.agentRunner.Create("Write exact requested text", "model", actor, config)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, actor, "start", "", false)
	if err == nil || task.Status != agents.TaskUncertain || tools.calls != 1 {
		t.Fatal(task, err)
	}
	call := task.Checkpoint.Calls[0]
	w := httptest.NewRecorder()
	s.handleAgentTaskAction(w, httptest.NewRequest("POST", "/v1/agents/tasks/"+task.ID+"/reconcile", strings.NewReader(fmt.Sprintf(`{"call_id":%q,"result":"done"}`, call.ID))))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		Status       string                   `json:"status"`
		Outcome      string                   `json:"outcome"`
		Steps        []agents.Step            `json:"steps"`
		Verification *agents.WorkflowEvidence `json:"verification"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "interrupted" || response.Outcome != "" || tools.calls != 1 {
		t.Fatal("report was completion or replay", w.Body.String())
	}
	last := response.Steps[len(response.Steps)-1]
	if last.Provenance != "human_reported" || last.ToolArgs != call.Function.Arguments || last.ToolResult != "done" || last.Timestamp.IsZero() {
		t.Fatal("lost report provenance", last)
	}
	if response.Verification == nil || len(response.Verification.Checks) != 1 || response.Verification.Checks[0].State == "verified" {
		t.Fatal("human report verified unrelated outcome", response.Verification)
	}
}

func TestHeldJobSnapshotDoesNotOfferResumeOrSteering(t *testing.T) {
	task := &agents.Task{Status: agents.TaskInterrupted, AdmissionHeld: true, Origin: &agents.ConversationOrigin{Workspace: "workspace", Conversation: "chat", TurnID: "turn"}, Checkpoint: &agents.Checkpoint{}}
	response := taskResponse(task)
	if response["resumable"] != false || response["can_steer"] != false || response["admission_held"] != true {
		t.Fatal("held admission advertised executable controls", response)
	}
}
