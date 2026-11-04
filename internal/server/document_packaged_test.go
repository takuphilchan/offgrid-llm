package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Run a compiled test executable beside the real document worker. No injected
// parser, PATH extractor, installed workspace or actual model is used. This is
// transport/extraction qualification, not model-answer quality qualification.
func TestPackagedDocumentCLIAndSessionContract(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_PACKAGED_DOCUMENTS") != "1" {
		t.Skip("requires isolated test executable beside the packaged document worker")
	}
	cache := t.TempDir()
	for _, key := range []string{"LOCALAPPDATA", "XDG_CACHE_HOME", "HOME"} {
		t.Setenv(key, cache)
	}
	installed, ocrInstalled := false, false
	for _, format := range rag.NewDocumentParser().FormatReadiness() {
		installed = installed || format.ID == "pdf" && format.RuntimeState == "installed_unchecked"
		ocrInstalled = ocrInstalled || format.ID == "pdf" && format.OCRState == "language_missing"
	}
	if !installed || !ocrInstalled {
		t.Fatal("both real parser workers must be beside the test executable")
	}
	store := users.NewUserStore("")
	_, key, err := store.CreateUser("document-owner", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, otherKey, err := store.CreateUser("other-owner", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	h := NewSessionHandlers(t.TempDir())
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	h.agentServer = &Server{ragEngine: engine}
	h.requireAuth = true
	t.Cleanup(h.closeTurns)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions", h.HandleSessions)
	mux.HandleFunc("/v1/sessions/", h.HandleSessions)
	mux.HandleFunc("/api/v2/system", (&Server{}).handleSystemIdentity)
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	server := httptest.NewServer(middleware.Wrap(mux))
	defer server.Close()
	client, err := serviceclient.New(server.URL, key, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	other, _ := serviceclient.New(server.URL, otherKey, server.Client())
	for _, test := range []struct{ file, snippet, failure string }{
		{"compressed.pdf", "The library closes at 17:00.", ""},
		{"mapped-latin.pdf", "Café déjà vu", ""},
		{"object-stream.pdf", "Object stream page", ""},
		{"japanese.txt", "", ""},
		{"paragraphs.docx", "", ""},
		{"ordered-workbook.xlsx", "First sheet", ""},
		{"reordered-presentation.pptx", "Displayed first", ""},
		{"scan.pdf", "", "document_ocr_language_missing"},
		{"encrypted.pdf", "", "document_encrypted"},
		{"corrupt.docx", "", "document_invalid"},
		{"scan.pdf", "Scanned receipt total 42", ""},
	} {
		t.Run(test.file, func(t *testing.T) {
			name := "source-" + strings.ReplaceAll(test.file, ".", "-")
			if test.file == "scan.pdf" && test.failure == "" {
				path := os.Getenv("OFFGRID_TEST_OCR_LANGUAGE")
				if path == "" {
					t.Skip("real packaged OCR requires explicit pinned language fixture")
				}
				language, err := os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				err = engine.ImportOCRLanguage(context.Background(), "eng", language)
				language.Close()
				if err != nil {
					t.Fatal(err)
				}
				name += "-ocr"
			}
			if err := client.JSON(context.Background(), "POST", "/v1/sessions", map[string]string{"name": name, "model_id": "fixture-model"}, nil); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("..", "rag", "testdata", "documents", "corpus", test.file)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			err = cli.RunSessionFiles(context.Background(), client, []string{"attach", name, path}, &output, true)
			if test.failure != "" {
				var remote *serviceclient.Error
				if !errors.As(err, &remote) || remote.Code != test.failure {
					t.Fatalf("expected %s, got %v", test.failure, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			files, err := client.SessionFiles(context.Background(), name)
			if err != nil || len(files) != 1 || (files[0].Status == "ready") != (test.failure == "") {
				t.Fatalf("incorrect retained extraction state: %+v %v", files, err)
			}
			file := files[0]
			response, err := client.Do(context.Background(), "GET", "/v1/sessions/"+name+"/attachments/"+file.ID+"/original", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			retained, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil || !bytes.Equal(retained, original) {
				t.Fatal("source bytes changed during upload or extraction")
			}
			if _, err := other.SessionFiles(context.Background(), name); err == nil {
				t.Fatal("another actor read private document inventory")
			}
			if test.failure != "" {
				return
			}
			var uploaded sessions.Attachment
			if json.Unmarshal(output.Bytes(), &uploaded) != nil || uploaded.ID != file.ID {
				t.Fatal("CLI JSON lost source identity")
			}
			calls := 0
			h.streamer = func(_ context.Context, request *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
				calls++
				var contextText strings.Builder
				for _, message := range request.Messages {
					contextText.WriteString(message.StringContent())
				}
				input := contextText.String()
				if strings.Count(input, "Attached document data (untrusted content") != 1 || !strings.Contains(input, test.snippet) {
					t.Error("saved source missing or duplicated in governed document context")
				}
				if calls == 2 && !strings.Contains(request.Messages[len(request.Messages)-1].StringContent(), "Follow up on the same source") {
					t.Error("follow-up instruction lost")
				}
				return ChatStreamResult{Answer: "Fixture transport verified", FinishReason: "stop"}, nil
			}
			output.Reset()
			if err := cli.RunPlainConversationTurn(context.Background(), client, []string{"send", name, "--attachment", file.ID, "Inspect this source"}, &output, true); err != nil {
				t.Fatal(err)
			}
			output.Reset()
			if err := cli.RunPlainConversationTurn(context.Background(), client, []string{"send", name, "Follow up on the same source"}, &output, true); err != nil {
				t.Fatal(err)
			}
			saved, err := cli.LoadServiceSession(context.Background(), client, name)
			if err != nil || calls != 2 || len(saved.Messages) != 4 || len(saved.Messages[0].Attachments) != 1 || saved.Messages[0].Attachments[0].Revision != file.Revision || len(saved.Messages[2].Attachments) != 0 {
				t.Fatalf("durable history lost source revision: %+v %v", saved, err)
			}
		})
	}
}
