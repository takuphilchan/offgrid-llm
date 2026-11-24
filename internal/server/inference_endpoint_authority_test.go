package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func authenticatedEndpointFixture(t *testing.T) (*Server, *users.User, string, inference.ProviderDescriptor) {
	t.Helper()
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	actor, key, err := s.userStore.CreateUser("endpoint-user", "isolated-endpoint-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	entry := endpointEnrollmentFixture()
	entry.Actors = []string{actor.ID}
	t.Setenv(entry.CredentialEnv, "isolated-endpoint-token")
	s.config.InferenceEndpointsFile = filepath.Join(t.TempDir(), "endpoints.json")
	if s.config.DataDir == "" {
		s.config.DataDir = t.TempDir()
	}
	writeEndpointEnrollment(t, s.config.InferenceEndpointsFile, entry)
	s.providerRegistry = inference.NewProviderRegistry()
	if err := s.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	reviewEndpointFixture(t, s, entry)
	installEndpointCollision(t, s)
	return s, actor, key, endpointDescriptor(entry)
}

func endpointAuthenticatedRequest(t *testing.T, s *Server, key, path, body string, next http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+key)
	response := httptest.NewRecorder()
	middleware.Wrap(next).ServeHTTP(response, request)
	return response
}

func TestEndpointAuthorityRechecksLiveStoreFromStaleRequestAndDurableActor(t *testing.T) {
	s, actor, key, _ := authenticatedEndpointFixture(t)
	var requestContext context.Context
	response := endpointAuthenticatedRequest(t, s, key, "/fixture", "", func(_ http.ResponseWriter, request *http.Request) { requestContext = request.Context() })
	if response.Code != 200 || requestContext == nil {
		t.Fatal("authenticated request fixture failed")
	}
	durableContext := withEndpointActor(context.Background(), actor.ID)
	for _, ctx := range []context.Context{requestContext, durableContext} {
		if _, _, err := s.selectNativeChat(ctx, "endpoint:lab@r1"); err != nil {
			t.Fatal(err)
		}
	}
	for _, disabled := range []bool{true, false} {
		if err := s.userStore.UpdateUser(actor.ID, map[string]any{"disabled": disabled}); err != nil {
			t.Fatal(err)
		}
		for _, ctx := range []context.Context{requestContext, durableContext} {
			_, _, err := s.selectNativeChat(ctx, "endpoint:lab@r1")
			if disabled == (err == nil) {
				t.Fatalf("disabled=%v selected with error=%v", disabled, err)
			}
		}
	}
	if err := s.userStore.DeleteUser(actor.ID); err != nil {
		t.Fatal(err)
	}
	for _, ctx := range []context.Context{requestContext, durableContext} {
		if _, _, err := s.selectNativeChat(ctx, "endpoint:lab@r1"); err == nil {
			t.Fatal("deleted actor retained endpoint authority")
		}
	}
	// Turning off authentication does not promote persisted user tasks to the
	// anonymous installation owner, even though new anonymous requests can run.
	s.config.RequireAuth = false
	if _, err := s.currentEndpointActor(durableContext); err == nil {
		t.Fatal("old actor promoted to local-admin")
	}
	if _, err := s.currentEndpointActor(withEndpointActor(requestContext, "another-actor")); err == nil {
		t.Fatal("request/task actor mismatch accepted")
	}
}

func TestEndpointAuthenticatedBackgroundJobPreservesActorWithoutRequestCredentials(t *testing.T) {
	s, actor, key, descriptor := authenticatedEndpointFixture(t)
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
	s.agentRunner = agents.NewRunner(s.agentManager, &browserRunTools{RunTools: s.toolRegistry, server: s}, s.callAgentModel)
	s.agentRunner.StreamCaller = s.streamAgentModel
	created := endpointAuthenticatedRequest(t, s, key, "/api/v2/agent-profiles", `{"settings":{"name":"Authenticated endpoint","model":"endpoint:lab@r1","max_iterations":3}}`, s.handleAgentProfiles)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Code, created.Body.String())
	}
	var profile agents.AgentProfile
	if err := json.Unmarshal(created.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	accepted := endpointAuthenticatedRequest(t, s, key, "/api/v2/jobs", fmt.Sprintf(`{"prompt":"hello","request_id":"authenticated-endpoint-task","profile_id":%q,"profile_revision":1}`, profile.ID), s.handleJobSubmit)
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
	if task == nil || task.Actor != actor.ID || task.Status != agents.TaskCompleted || task.Result != "fixture answer" || runtime.called != 1 {
		t.Fatalf("authenticated background endpoint failed: %+v calls=%d", task, runtime.called)
	}
	scheduled := task.Config
	scheduledConfiguration(&scheduled)
	if err := s.validateScheduleResources(context.Background(), actor.ID, scheduled, false); err != nil {
		t.Fatalf("saved actor unavailable during schedule validation: %v", err)
	}
	if err := s.userStore.DeleteUser(actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.callNativeTextAgent(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "after deletion"}}, nil, nil); err == nil || runtime.called != 1 {
		t.Fatal("deleted durable actor dispatched provider generation")
	}
}
