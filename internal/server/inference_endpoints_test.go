package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func endpointEnrollmentFixture() inferenceEndpointEnrollment {
	return inferenceEndpointEnrollment{ID: "lab", Revision: "r1", Provider: inference.ProviderOllama, URL: "https://inference.example.test:8443", AllowedIPs: []string{"192.0.2.9"}, Actors: []string{"local-admin"}, CredentialEnv: "OFFGRID_TEST_ENDPOINT_TOKEN", Model: "fixture-model", ModelDigest: "sha256:" + strings.Repeat("a", 64), Format: inference.FormatGGUF, RuntimeVersion: "fixture-1", ContextLength: 8192, DeploymentAttested: true}
}

func writeEndpointEnrollment(t *testing.T, path string, entries ...inferenceEndpointEnrollment) {
	t.Helper()
	raw, err := json.Marshal(inferenceEndpointFile{Version: 1, Endpoints: entries})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredHTTPEndpointEnrollmentRevalidatesAuthorityAndPins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "endpoints.json")
	entry := endpointEnrollmentFixture()
	writeEndpointEnrollment(t, path, entry)
	t.Setenv("OFFGRID_TEST_ENDPOINT_TOKEN", "isolated-credential")
	s := &Server{config: &config.Config{InferenceEndpointsFile: path, DataDir: t.TempDir()}, providerRegistry: inference.NewProviderRegistry()}
	if err := s.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	reviewEndpointFixture(t, s, entry)
	descriptor, selected, err := s.selectNativeChat(context.Background(), "endpoint:lab@r1")
	if err != nil || !selected || descriptor.Provider != inference.ProviderOllama {
		t.Fatalf("selection=%+v %v %v", descriptor, selected, err)
	}
	encoded, _ := json.Marshal(descriptor)
	if strings.Contains(string(encoded), "credential") || strings.Contains(string(encoded), entry.URL) {
		t.Fatal("descriptor exposed credential or endpoint")
	}
	listed := s.listedEndpointModels(context.Background())
	if len(listed) != 1 || listed[0].RuntimeReady || listed[0].Qualified || listed[0].SmokeTested || listed[0].ID != "endpoint:lab@r1" {
		t.Fatalf("endpoint menu claims false readiness: %+v", listed)
	}
	for _, change := range []func(*inferenceEndpointEnrollment){func(e *inferenceEndpointEnrollment) { e.Actors = []string{"different-actor"} }, func(e *inferenceEndpointEnrollment) { e.ModelDigest = "sha256:" + strings.Repeat("b", 64) }, func(e *inferenceEndpointEnrollment) { e.URL = "https://different.example.test" }, func(e *inferenceEndpointEnrollment) { e.RuntimeVersion = "changed" }} {
		mutated := entry
		change(&mutated)
		writeEndpointEnrollment(t, path, mutated)
		if _, _, err = s.selectNativeChat(context.Background(), "endpoint:lab@r1"); err == nil {
			t.Fatal("changed enrollment reused stale route")
		}
		if len(s.listedEndpointModels(context.Background())) != 0 {
			t.Fatal("revoked endpoint remained in menu")
		}
	}
	writeEndpointEnrollment(t, path, entry)
	if err = os.Unsetenv(entry.CredentialEnv); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.selectNativeChat(context.Background(), "endpoint:lab@r1"); err == nil {
		t.Fatal("missing credential accepted")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.selectNativeChat(context.Background(), "endpoint:lab@r1"); err == nil {
		t.Fatal("removed configuration retained authority")
	}
}

func TestConfiguredHTTPEndpointRejectsUnsafeAndAmbiguousEnrollment(t *testing.T) {
	for name, change := range map[string]func(*inferenceEndpointEnrollment){"http": func(e *inferenceEndpointEnrollment) { e.URL = "http://inference.example.test" }, "loopback": func(e *inferenceEndpointEnrollment) {
		e.URL = "https://127.0.0.1:1234"
		e.AllowedIPs = []string{"127.0.0.1"}
	}, "metadata": func(e *inferenceEndpointEnrollment) { e.AllowedIPs = []string{"169.254.169.254"} }, "no attestation": func(e *inferenceEndpointEnrollment) { e.DeploymentAttested = false }, "wildcard": func(e *inferenceEndpointEnrollment) { e.Actors = []string{"*"} }, "url credentials": func(e *inferenceEndpointEnrollment) { e.URL = "https://user:secret@inference.example.test" }} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "endpoints.json")
			entry := endpointEnrollmentFixture()
			change(&entry)
			writeEndpointEnrollment(t, path, entry)
			s := &Server{config: &config.Config{InferenceEndpointsFile: path, DataDir: t.TempDir()}, providerRegistry: inference.NewProviderRegistry()}
			if err := s.InstallConfiguredHTTPEndpoints(); err == nil {
				t.Fatal("unsafe enrollment accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "endpoints.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"version":1,"endpoints":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readInferenceEndpoints(path); err == nil {
		t.Fatal("ambiguous configuration accepted")
	}
}

func TestEndpointChatIsReachableThroughExistingHTTPServiceWithoutFallback(t *testing.T) {
	descriptor := endpointDescriptor(endpointEnrollmentFixture())
	registry := inference.NewProviderRegistry()
	runtime := &nativeChatRuntimeFixture{descriptor: descriptor}
	if err := registry.Register(inference.ProviderOllama, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
		return runtime, nil
	}); err != nil {
		t.Fatal(err)
	}
	scheduler, err := inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir()), providerScheduler: scheduler, endpointChatSelector: nativeChatSelectorFixture{descriptor: descriptor}}
	installEndpointCollision(t, s)
	for _, stream := range []string{"false", "true"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"endpoint:lab@r1","messages":[{"role":"user","content":"hello"}],"stream":`+stream+`}`))
		s.handleChatCompletions(response, request)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "fixture answer") {
			t.Fatalf("endpoint response status=%d body=%s", response.Code, response.Body.String())
		}
	}
	request := &api.ChatCompletionRequest{Model: "endpoint:lab@r1", Messages: []api.ChatMessage{{Role: "user", Content: "hello"}}}
	result, err := s.streamSessionChat(context.Background(), request, "", func(ChatStreamEvent) error { return nil })
	if err != nil || result.Answer != "fixture answer" {
		t.Fatalf("durable endpoint stream: %+v %v", result, err)
	}
	for _, unavailable := range []bool{false, true} {
		s.endpointChatSelector = nativeChatSelectorFixture{err: errors.New("enrollment revoked")}
		expectedStatus := http.StatusForbidden
		if unavailable {
			s.endpointChatSelector = nil
			expectedStatus = http.StatusServiceUnavailable
		}
		for _, stream := range []string{"false", "true"} {
			response := httptest.NewRecorder()
			s.handleChatCompletions(response, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"endpoint:lab@r1","messages":[{"role":"user","content":"hello"}],"stream":`+stream+`}`)))
			if response.Code != expectedStatus || !strings.Contains(response.Body.String(), "endpoint_model_unavailable") {
				t.Fatalf("unavailable endpoint collided: status=%d body=%s", response.Code, response.Body.String())
			}
		}
		if _, err := s.completeChat(context.Background(), request); err == nil {
			t.Fatal("durable completion accepted unavailable endpoint")
		}
		if _, err := s.streamSessionChat(context.Background(), request, "", func(ChatStreamEvent) error { return nil }); err == nil {
			t.Fatal("durable stream accepted unavailable endpoint")
		}
	}
	if runtime.called != 3 {
		t.Fatalf("unavailable endpoint invoked provider: calls=%d", runtime.called)
	}
}

func TestHTTPEndpointTextProfileRunsThroughCanonicalAgent(t *testing.T) {
	s := profileFixture(t)
	installEndpointCollision(t, s)
	descriptor := endpointDescriptor(endpointEnrollmentFixture())
	registry := inference.NewProviderRegistry()
	runtime := &nativeChatRuntimeFixture{descriptor: descriptor}
	if err := registry.Register(inference.ProviderOllama, func(context.Context, inference.ProviderDescriptor) (inference.ProviderRuntime, error) {
		return runtime, nil
	}); err != nil {
		t.Fatal(err)
	}
	var err error
	s.providerScheduler, err = inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.providerScheduler.Close()
	s.endpointChatSelector = nativeChatSelectorFixture{descriptor: descriptor}
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, s.callAgentModel)
	s.agentRunner.StreamCaller = s.streamAgentModel
	created := profileHTTP(s, http.MethodPost, "/api/v2/agent-profiles", map[string]any{"settings": agents.ProfileSettings{Name: "Enrolled endpoint text", Model: "endpoint:lab@r1", MaxIterations: 3}})
	if created.Code != http.StatusCreated {
		t.Fatal(created.Code, created.Body.String())
	}
	var profile agents.AgentProfile
	if err = json.Unmarshal(created.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	accepted := httptest.NewRecorder()
	s.handleJobSubmit(accepted, httptest.NewRequest(http.MethodPost, "/api/v2/jobs", strings.NewReader(fmt.Sprintf(`{"prompt":"hello","request_id":"endpoint-text-task-001","profile_id":%q,"profile_revision":1}`, profile.ID))))
	if accepted.Code != http.StatusAccepted {
		t.Fatal(accepted.Code, accepted.Body.String())
	}
	var submitted struct {
		ID string `json:"task_id"`
	}
	if err = json.Unmarshal(accepted.Body.Bytes(), &submitted); err != nil {
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
	if task == nil || task.Status != agents.TaskCompleted || task.Result != "fixture answer" || !task.Config.TextOnlyInference || len(task.Steps) != 0 {
		t.Fatalf("endpoint job did not finish safely: %+v", task)
	}
	s.endpointChatSelector = nativeChatSelectorFixture{err: errors.New("enrollment revoked")}
	request := httptest.NewRequest(http.MethodPost, "/api/v2/jobs", strings.NewReader(fmt.Sprintf(`{"prompt":"hello","request_id":"endpoint-text-task-002","profile_id":%q,"profile_revision":1}`, profile.ID)))
	denied := httptest.NewRecorder()
	s.handleJobSubmit(denied, request)
	if denied.Code != http.StatusUnprocessableEntity || runtime.called != 1 {
		t.Fatalf("revoked endpoint admitted through local collision: %d %s", denied.Code, denied.Body.String())
	}
	count := 0
	if effective := s.effectiveProfile(request, &profile, map[string]bool{}, 0, &count, nil); effective.Ready {
		t.Fatal("revoked endpoint profile became ready from local collider")
	}
}
