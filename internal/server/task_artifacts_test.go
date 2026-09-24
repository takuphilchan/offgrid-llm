package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestTaskArtifactsVerifiedDurablyAndOwnerScoped(t *testing.T) {
	root := t.TempDir()
	store, err := artifacts.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{artifactStore: store, agentManager: agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())}
	args := `{"name":"comparison.csv","format":"csv","content":"model,score\nA,82\nB,91\n"}`
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	s.agentRunner = agents.NewRunner(s.agentManager, tools, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		message := api.ChatMessage{Role: "assistant", Content: "Artifact saved; content accuracy not assessed."}
		reason := "stop"
		if len(task.Steps) == 0 {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: taskArtifactToolName, Arguments: args}}}
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}, nil
	})
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 16384
	task, _ := s.agentRunner.Create("Create the comparison table", "model", "local-admin", cfg)
	result, err := s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || result.Status != agents.TaskCompleted {
		t.Fatalf("artifact run %+v %v", result, err)
	}
	found := taskArtifacts(result)
	if len(found) != 1 || !found[0].Verified || found[0].Rows != 3 || found[0].Columns != 2 {
		t.Fatalf("invalid evidence %+v", found)
	}
	digest := found[0].SHA256
	route := "/api/v2/jobs/" + task.ID + "/artifact?digest=" + digest
	w := httptest.NewRecorder()
	s.handleJobRead(w, httptest.NewRequest("GET", route, nil))
	if w.Code != 200 || w.Body.String() != "model,score\nA,82\nB,91\n" || !strings.Contains(w.Header().Get("Content-Disposition"), "comparison.csv") {
		t.Fatal(w.Code, w.Body.String())
	}
	foreign, _ := s.agentRunner.Create("same bytes", "model", "bob", cfg)
	foreign, err = s.agentRunner.Continue(context.Background(), foreign.ID, "bob", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.handleJobRead(w, httptest.NewRequest("GET", strings.Replace(route, task.ID, foreign.ID, 1), nil))
	if w.Code != 404 {
		t.Fatal("cross-actor download")
	}
	if err := s.agentRunner.Delete(task.ID, "local-admin"); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.handleArtifacts(w, httptest.NewRequest("GET", "/v1/artifacts/"+digest, nil))
	if w.Code != 404 {
		t.Fatal("legacy route bypassed deleted owner reference")
	}
	// Another owner with identical bytes does not inherit our task's metadata/access.
	if taskArtifacts(foreign)[0].Name != "comparison.csv" {
		t.Fatal("dedup changed filename")
	}
	if err = os.WriteFile(filepath.Join(root, "sha256", digest), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.saveTaskArtifact(context.Background(), json.RawMessage(args)); err == nil {
		t.Fatal("corrupt existing object reported verified")
	}
}

func TestArtifactValidationRejectsExecutablePathsAndMalformedFormats(t *testing.T) {
	for _, req := range []taskArtifactRequest{
		{"../escape.txt", "text", "x"}, {"run.exe", "text", "x"}, {"bad.json", "json", "{"}, {"bad.csv", "csv", "a,b\n1\n"}, {"formula.csv", "csv", "name\n=HYPERLINK(1)\n"}, {"big.txt", "text", strings.Repeat("x", 128<<10+1)},
	} {
		raw, _ := json.Marshal(req)
		if _, _, err := parseTaskArtifact(raw); err == nil {
			t.Fatalf("invalid artifact accepted: %s", req.Name)
		}
	}
	for _, req := range []taskArtifactRequest{{"report.txt", "text", "Unicode — مرحبا"}, {"report.md", "markdown", "# Findings"}, {"report.json", "json", `{"value":42}`}} {
		raw, _ := json.Marshal(req)
		if _, _, err := parseTaskArtifact(raw); err != nil {
			t.Fatal(err)
		}
	}
}
