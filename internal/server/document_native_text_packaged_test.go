package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Uses only explicitly supplied local bytes and a disposable service. The model
// callback checks source admission; it is not evidence of model answer quality.
func TestPackagedNativeTextUploadAndFailedRetry(t *testing.T) {
	path := os.Getenv("OFFGRID_TEST_DOCUMENT_SAMPLE")
	if path == "" || os.Getenv("OFFGRID_TEST_PACKAGED_DOCUMENTS") != "1" {
		t.Skip("explicit sample and packaged workers required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := newStreamingSessions(t)
	defer h.closeTurns()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions", h.HandleSessions)
	mux.HandleFunc("/v1/sessions/", h.HandleSessions)
	mux.HandleFunc("/api/v2/system", (&Server{}).handleSystemIdentity)
	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := serviceclient.New(server.URL, "", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	draft := cli.ConversationFileDraft{}
	_, display, err := draft.Command(ctx, client, "chat", "/attach "+path)
	if err != nil || len(draft.Files) != 1 || !strings.Contains(display, "have not been interpreted") {
		t.Fatalf("CLI upload: %v %s", err, display)
	}
	uploaded := draft.Files[0]
	if uploaded.Status != "ready" || uploaded.Coverage != "native_text_only" {
		t.Fatal("coverage missing")
	}
	// Reproduce the persisted failed attachment from the previous worker build.
	scope := h.scoped(httptest.NewRequest("GET", "/v1/sessions/chat", nil))
	old, err := scope.CreateAttachment("chat", filepath.Base(path), "application/pdf", data)
	if err != nil {
		t.Fatal(err)
	}
	old, err = scope.PrepareAttachment("chat", old.ID, "old partial", "old-worker", "document_ocr_required")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := client.PrepareSessionFile(ctx, "chat", old)
	if err != nil || recovered.Status != "ready" || recovered.Coverage != "native_text_only" || recovered.SHA256 != old.SHA256 {
		t.Fatal("saved retry did not recover", err)
	}
	preview, err := client.PreviewSessionFile(ctx, "chat", uploaded)
	if err != nil {
		t.Fatal(err)
	}
	var units []string
	for _, u := range preview.Units {
		units = append(units, u.Text)
	}
	_, text, err := scope.ReadAttachment("chat", uploaded.ID)
	if err != nil || text != strings.TrimSpace(strings.Join(units, "\n")) {
		t.Fatal("native bytes changed", err)
	}
	seen := false
	h.streamer = func(_ context.Context, request *api.ChatCompletionRequest, _ string, _ func(ChatStreamEvent) error) (ChatStreamResult, error) {
		input := request.Messages[len(request.Messages)-1].StringContent()
		if !strings.Contains(input, "native_text_only") || !strings.Contains(input, "do not claim to have interpreted") {
			t.Error("coverage missing from model context")
		}
		seen = true
		return ChatStreamResult{Answer: "Fixture admission checked", FinishReason: "stop"}, nil
	}
	var out bytes.Buffer
	if err := cli.RunSessionFiles(ctx, client, []string{"attachments", "chat"}, &out, false); err != nil || !strings.Contains(out.String(), "have not been interpreted") {
		t.Fatal("CLI inventory coverage lost", err)
	}
	out.Reset()
	if err := cli.RunPlainConversationTurn(ctx, client, []string{"send", "chat", "--attachment", uploaded.ID, "Summarize the text"}, &out, true); err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("model path never admitted")
	}
	saved, err := scope.Load("chat")
	if err != nil || len(saved.Messages) != 2 {
		t.Fatal("history not saved", err)
	}
	refs := []sessions.AttachmentReference{{ID: uploaded.ID, Revision: uploaded.Revision}}
	texts, err := scope.AttachmentTexts("chat", refs)
	if err != nil || texts[0].Attachment.Coverage != "native_text_only" {
		t.Fatal("saved coverage lost", err)
	}
}
