package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func nativeAgentFixture(t *testing.T) (*Server, *nativeChatRuntimeFixture) {
	t.Helper()
	s := profileFixture(t)
	descriptor := inference.ProviderDescriptor{ProtocolRevision: inference.ProviderProtocolRevision, Provider: inference.ProviderTransformers, RunnerRevision: "fixture", AdapterRevision: "fixture", ModelDigest: "sha256:fixture", Format: inference.FormatSafetensors, Backend: "cpu", ContextLength: 8192, Capabilities: []inference.ProviderCapability{inference.CapabilityText, inference.CapabilityStreaming}}
	registry := inference.NewProviderRegistry()
	runtime := &nativeChatRuntimeFixture{descriptor: descriptor}
	if err := registry.Register(inference.ProviderTransformers, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
		return runtime, nil
	}); err != nil {
		t.Fatal(err)
	}
	var err error
	s.providerScheduler, err = inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.providerScheduler.Close() })
	s.nativeChatSelector = nativeChatSelectorFixture{descriptor: descriptor}
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, s.callAgentModel)
	s.agentRunner.StreamCaller = s.streamAgentModel
	return s, runtime
}

func TestQualifiedNativeTextProfileRunsInCanonicalJobWithoutTools(t *testing.T) {
	s, runtime := nativeAgentFixture(t)
	settings := agents.ProfileSettings{Name: "Text only", Model: "test@revision", MaxIterations: 3}
	w := profileHTTP(s, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": settings})
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	var profile agents.AgentProfile
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	payload := fmt.Sprintf(`{"prompt":"hello","request_id":"native-text-task-001","profile_id":%q,"profile_revision":1}`, profile.ID)
	accepted := httptest.NewRecorder()
	s.handleJobSubmit(accepted, httptest.NewRequest(http.MethodPost, "/api/v2/jobs", strings.NewReader(payload)))
	if accepted.Code != http.StatusAccepted {
		t.Fatal(accepted.Code, accepted.Body.String())
	}
	var submitted struct {
		ID string `json:"task_id"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var task *agents.Task
	for time.Now().Before(deadline) {
		task, _ = s.agentManager.GetTask(submitted.ID)
		if task != nil && (task.Status == agents.TaskCompleted || task.Status == agents.TaskFailed) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if task == nil || task.Status != agents.TaskCompleted || task.Result != "fixture answer" || !task.Config.TextOnlyInference || task.Config.ContextWindow != 8192 || runtime.called != 1 {
		t.Fatalf("qualified native text job did not finish safely: task=%+v runtime calls=%d", task, runtime.called)
	}
	if len(task.Steps) != 0 {
		t.Fatalf("text-only model caused tool activity: %+v", task.Steps)
	}
}

func TestNativeTextJobRejectsToolsAndNeverFallsBackToLlama(t *testing.T) {
	s, runtime := nativeAgentFixture(t)
	d, _ := s.toolRegistry.Capability("calculator")
	settings := agents.ProfileSettings{Name: "Needs tools", Model: "test@revision", MaxIterations: 3, Tools: []agents.ProfileToolReference{{Provider: d.Source, Tool: d.Name, SchemaRevision: d.Revision, Location: d.Location}}}
	w := profileHTTP(s, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": settings})
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "native_tools_unqualified") || runtime.called != 0 {
		t.Fatal("tool-enabled native profile was admitted", w.Code, w.Body.String())
	}
	textOnly := agents.DefaultAgentConfig()
	textOnly.TaskFirst, textOnly.TextOnlyInference = true, true
	textOnly.MaxIterations = 3
	task := &agents.Task{Model: settings.Model, Config: textOnly}
	if _, err := s.callNativeTextAgent(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "hello"}}, []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "calculator"}}}, nil); err == nil || runtime.called != 0 {
		t.Fatal("tool catalog reached an unqualified native worker")
	}
	s.nativeChatSelector = nativeChatSelectorFixture{err: errors.New("stale qualification")}
	if _, err := s.callNativeTextAgent(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "hello"}}, nil, nil); err == nil || runtime.called != 0 {
		t.Fatal("stale native model silently fell back or generated")
	}
}

func TestNativeTextChildProfileIsNotAdvertisedBeforeDelegationQualification(t *testing.T) {
	s, _ := nativeAgentFixture(t)
	child := profileHTTP(s, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": agents.ProfileSettings{Name: "Native reader", Model: "test@revision", MaxIterations: 3}})
	if child.Code != http.StatusCreated {
		t.Fatal(child.Code, child.Body.String())
	}
	var profile agents.AgentProfile
	if err := json.Unmarshal(child.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	parent := profileHTTP(s, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": agents.ProfileSettings{Name: "Parent", Model: "model", MaxIterations: 6, Delegates: []string{profile.ID}}})
	if parent.Code != http.StatusUnprocessableEntity || !strings.Contains(parent.Body.String(), "delegate_unavailable") {
		t.Fatal("unqualified native child reached profile admission", parent.Code, parent.Body.String())
	}
}
