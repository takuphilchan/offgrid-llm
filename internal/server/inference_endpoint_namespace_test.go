package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// A legacy registry/import may contain a literal endpoint ID even on a host
// whose filename rules cannot create it. Keep a real harmless file to prove
// that API routing neither executes nor removes the colliding local artifact.
func installEndpointCollision(t *testing.T, s *Server) string {
	t.Helper()
	directory := t.TempDir()
	path := filepath.Join(directory, "unchanged.gguf")
	if err := os.WriteFile(path, []byte("isolated local collision fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	entries := map[string]*api.ModelMetadata{}
	for _, listed := range s.registry.ListModels() {
		entry, err := s.registry.GetModel(listed.ID)
		if err != nil {
			t.Fatal(err)
		}
		entries[listed.ID] = entry
	}
	for _, id := range []string{"endpoint:lab@r1", "endpoint:malformed"} {
		entries[id] = &api.ModelMetadata{ID: id, Path: path, Type: "llm", Format: "gguf", ContextSize: 128000, ProjectorPath: path}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(directory, "registry.json")
	if err := os.WriteFile(registryPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.LoadRegistry(registryPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.registry.GetChatModel("endpoint:lab@r1"); err != nil {
		t.Fatalf("fixture lacks actual local collision: %v", err)
	}
	return path
}

func TestEndpointNamespaceCannotReachLocalPublicOperations(t *testing.T) {
	s := profileFixture(t)
	path := installEndpointCollision(t, s)
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
		payload string
		status  int
	}{
		{"legacy completion", s.handleCompletions, `{"model":"endpoint:lab@r1","prompt":"hello"}`, 404},
		{"embeddings", s.handleEmbeddings, `{"model":"endpoint:lab@r1","input":"hello"}`, 400},
		{"prepare", s.handleInferencePrepare, `{"model":"endpoint:lab@r1"}`, 400},
		{"legacy agent", s.handleAgentRun, `{"model":"endpoint:lab@r1","prompt":"hello"}`, 404},
		{"computer qualification", s.handleComputerModelCheck, `{"model":"endpoint:lab@r1"}`, 404},
		{"delete", s.handleDeleteModel, `{"model_id":"endpoint:lab@r1"}`, 409},
		{"malformed endpoint", s.handleChatCompletions, `{"model":"endpoint:malformed","messages":[{"role":"user","content":"hello"}]}`, 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodPost, "/fixture", strings.NewReader(test.payload)))
			if response.Code != test.status {
				t.Fatalf("local operation accepted endpoint identity: %d %s", response.Code, response.Body.String())
			}
		})
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != "isolated local collision fixture" {
		t.Fatalf("colliding local file changed: %q %v", content, err)
	}
	if _, err := s.registry.GetChatModel("endpoint:lab@r1"); err != nil {
		t.Fatal("colliding local registry record was removed")
	}
}

func TestEndpointNamespaceCannotAcquireLocalLeaseOrTokenizer(t *testing.T) {
	engine := &agentPromptCounterFixture{}
	s := &Server{config: &config.Config{}, registry: models.NewRegistry(t.TempDir()), engine: engine, inferenceLifecycle: inference.NewLifecycleGate(1)}
	installEndpointCollision(t, s)
	// Even a cached lifecycle binding must not let an endpoint reuse local work.
	release, err := s.inferenceLifecycle.AcquireWithContext(context.Background(), "endpoint:lab@r1", 4096, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	loading := false
	if lease, err := s.acquireInferenceLease(context.Background(), "endpoint:lab@r1", 4096, func() error { loading = true; return nil }); err == nil || lease != nil || loading {
		t.Fatal("endpoint reused or acquired a cached local lease")
	}
	for _, textOnly := range []bool{false, true} {
		task := &agents.Task{Model: "endpoint:lab@r1", Config: agents.AgentConfig{TextOnlyInference: textOnly}}
		if count, err := s.countAgentContext(context.Background(), task, []api.ChatMessage{{Role: "user", Content: "hello"}}, nil); count != 0 || !errors.Is(err, agents.ErrContextCountUnavailable) || engine.calls != 0 {
			t.Fatalf("endpoint used local tokenizer: %d %v calls=%d", count, err, engine.calls)
		}
	}
	if err := s.sessionImageReady("endpoint:lab@r1"); err == nil {
		t.Fatal("endpoint reused local vision projector")
	}
	settings := agents.ProfileSettings{Model: "endpoint:lab@r1", ModelPolicy: &agents.ProfileModelPolicy{Version: 1, Mode: "guided"}}
	if selection, issue := s.resolveProfileModelPolicy(context.Background(), settings, settings.Model, s.modelContextWindow); selection != nil || issue == nil {
		t.Fatal("endpoint was admitted as an installed local policy candidate")
	}
}

func TestEndpointNamespaceListDoesNotRevealRevokedLocalCollider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "endpoints.json")
	entry := endpointEnrollmentFixture()
	writeEndpointEnrollment(t, path, entry)
	t.Setenv(entry.CredentialEnv, "isolated-test-token")
	s := &Server{config: &config.Config{InferenceEndpointsFile: path, DataDir: t.TempDir()}, registry: models.NewRegistry(t.TempDir()), providerRegistry: inference.NewProviderRegistry()}
	installEndpointCollision(t, s)
	if err := s.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	reviewEndpointFixture(t, s, entry)
	for _, revoked := range []bool{false, true} {
		if revoked {
			entry.Actors = []string{"another-actor"}
			writeEndpointEnrollment(t, path, entry)
		}
		response := httptest.NewRecorder()
		s.handleListModels(response, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
		var listed api.ModelListResponse
		if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
			t.Fatal(err)
		}
		if revoked && len(listed.Data) != 0 || !revoked && (len(listed.Data) != 1 || listed.Data[0].OwnedBy != "enrolled-endpoint") {
			t.Fatalf("model list exposed colliding local record: %+v", listed.Data)
		}
	}
}
