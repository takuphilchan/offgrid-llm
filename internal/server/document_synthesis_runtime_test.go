package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// Opt-in integration: existing local assets only, disposable registry and data,
// real tokenizer/generation/process ownership and authenticated-source boundaries.
// Synthetic English/Chinese policy checks are not representative qualification.
func TestRealDocumentSynthesisHTTP(t *testing.T) {
	modelPath := os.Getenv("OFFGRID_TEST_SYNTHESIS_MODEL")
	if modelPath == "" || os.Getenv("OFFGRID_LLAMA_SERVER_PATH") == "" {
		t.Skip("requires explicit existing local model and runtime; no downloads")
	}
	dir := t.TempDir()
	// A read-only hard link keeps this test's registry independent of installed
	// user metadata without copying multiple gigabytes or changing model bytes.
	if err := os.Link(modelPath, filepath.Join(dir, "fixture.gguf")); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	cache := inference.NewModelCache(1, 0, t.TempDir())
	cache.SetNumThreads(6)
	defer cache.UnloadAll()
	llama := inference.NewLlamaHTTPEngine("http://127.0.0.1:1")
	process := inference.NewLlamaCPPProcess(cache, llama)
	scheduler, err := inference.NewProviderScheduler(inference.NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	store, err := engine.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{ModelsDir: dir, DataDir: t.TempDir(), MaxContextSize: 8192, NumThreads: 6, ModelLoadTimeout: 90}, registry: registry, engine: llama, modelCache: cache, llamaProcess: process, inferenceLifecycle: inference.NewLifecycleGate(1), providerScheduler: scheduler, ragEngine: engine}
	s.documentSynthesisFactory = func(actor, work string) documentSynthesisRuntime {
		return &tracedSynthesisRuntime{documentSynthesisRuntime: &localDocumentSynthesis{server: s, actor: actor, work: work}, t: t}
	}
	accounts := users.NewUserStore("")
	owner, key, err := accounts.CreateUser("reviewer", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	s.userStore, s.config.RequireAuth = accounts, true
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	doc := &rag.Document{ID: "synthetic-policy", Name: "Synthetic policy", RawContent: "Applicants must submit a signed form by 31 July. Exception: rural clinics may submit by 15 August. The fee is USD 25. No other eligibility conditions are specified.", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"en", "zh-Hans"} {
		t.Run(language, func(t *testing.T) {
			started := time.Now()
			workflow := "requirements"
			if os.Getenv("OFFGRID_TEST_COMPOSITION") == "1" {
				workflow = "summary"
			}
			body, _ := json.Marshal(synthesisCreateRequest{RequestKey: language, Workflow: workflow, Sources: []document.SourceRef{{DocumentID: doc.ID, Revision: doc.ExtractionRevision}}, Model: "fixture", Language: language})
			w := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/v1/document-analyses/synthesis", bytes.NewReader(body)).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer "+key)
			middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(s.handleDocumentSynthesis))).ServeHTTP(w, request)
			var job rag.SynthesisRecord
			if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &job) != nil {
				t.Fatal("create", w.Code, w.Body.String())
			}
			for job.State == "queued" || job.State == "running" {
				select {
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(200 * time.Millisecond):
				}
				current, err := store.Synthesis(owner.ID, job.ID)
				if err != nil {
					t.Fatal(err)
				}
				job = *current
			}
			if job.State != "awaiting_review" {
				t.Fatalf("generation did not produce validated candidates: state=%s code=%s", job.State, job.ErrorCode)
			}
			page, err := store.SynthesisResults(owner.ID, job.ID, 0, 2)
			if err != nil || len(page.Receipts) != 1 || len(page.Receipts[0].Result.Candidates) == 0 {
				t.Fatal("missing candidates", err)
			}
			for _, candidate := range page.Receipts[0].Result.Candidates {
				if candidate.Verification != "source_identity_only" {
					t.Fatal("incorrect approval")
				}
				for _, quote := range candidate.Evidence {
					if !strings.Contains(doc.RawContent, quote.Text) {
						t.Fatal("invented quotation")
					}
				}
			}
			artifact, err := store.ExportSynthesis(ctx, owner.ID, job.ID, "csv")
			if err != nil || !strings.Contains(artifact.Content, "source_identity_only") {
				t.Fatal("real result export", err)
			}
			t.Logf("model_sha256=%s runtime_sha256=%s context=%d prompt_tokens=%d candidates=%d elapsed=%s", job.Model.ModelSHA256, job.Model.Runtime.SHA256, job.Model.ContextTokens, page.Receipts[0].PromptTokens, len(page.Receipts[0].Result.Candidates), time.Since(started))
			if workflow == "summary" {
				runRealComposition(t, ctx, s, store, middleware, key, owner.ID, job)
			}
		})
	}
}

type tracedSynthesisRuntime struct {
	documentSynthesisRuntime
	t *testing.T
}

func (p *tracedSynthesisRuntime) WithModel(ctx context.Context, i document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	return p.documentSynthesisRuntime.WithModel(ctx, i, func(session document.SynthesisLocalSession) error {
		trace := &tracedSynthesisSession{SynthesisLocalSession: session}
		err := run(trace)
		if err != nil {
			p.t.Logf("synthetic fixture rejection: %v; response: %s", err, trace.raw)
		} else {
			p.t.Logf("synthetic fixture model output: %s", trace.raw)
		}
		return err
	})
}

type tracedSynthesisSession struct {
	document.SynthesisLocalSession
	raw []byte
}

func (s *tracedSynthesisSession) Generate(ctx context.Context, p document.SynthesisPrompt, tokens int) ([]byte, string, error) {
	raw, finish, err := s.SynthesisLocalSession.Generate(ctx, p, tokens)
	s.raw = append([]byte(nil), raw...)
	return raw, finish, err
}
