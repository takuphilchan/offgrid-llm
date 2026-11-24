package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalSourceDraftCLIActualServiceReceiptAndRestart(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	source := filepath.Join(t.TempDir(), "policy.txt")
	if err := os.WriteFile(source, []byte("Shipment 12345 is exempt."), 0600); err != nil {
		t.Fatal(err)
	}
	image := testSessionImage(t)
	imageBytes, err := base64.StdEncoding.DecodeString(strings.SplitN(image.DataURL, ",", 2)[1])
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "diagram.png")
	if err = os.WriteFile(imagePath, imageBytes, 0600); err != nil {
		t.Fatal(err)
	}
	projector := filepath.Join(t.TempDir(), "projector-fixture")
	if err = os.WriteFile(projector, []byte("fixture only"), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]api.ModelMetadata{"model": {ID: "model", Type: "llm", ProjectorPath: projector}})
	registry := filepath.Join(t.TempDir(), "registry.json")
	if err = os.WriteFile(registry, metadata, 0600); err != nil {
		t.Fatal(err)
	}
	if err = f.s.registry.LoadRegistry(registry); err != nil {
		t.Fatal(err)
	}
	var uploads, submissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/system":
			f.s.handleSystemIdentity(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v2/conversations"):
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/inputs") {
				if submissions.Add(1) == 1 {
					accepted := httptest.NewRecorder()
					f.s.handleConversations(accepted, r)
					if accepted.Code != 202 {
						t.Error(accepted.Code, accepted.Body.String())
					}
					w.WriteHeader(503)
					return
				}
			}
			f.s.handleConversations(w, r)
		case strings.HasPrefix(r.URL.Path, "/v1/sessions"):
			if strings.HasSuffix(r.URL.Path, "/generate") {
				t.Error("legacy executor invoked")
				w.WriteHeader(500)
				return
			}
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/attachments") {
				uploads.Add(1)
			}
			f.s.sessionHandlers.HandleSessions(w, r)
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	input := "/attach " + source + "\n/attachments\n/preview-attachment 1\n/retry-attachment 1\n/image " + imagePath + "\n\nMust not replace unknown input\n/retry\nFollow up\n/exit\n"
	if err = cli.RunPlainConversationChat(context.Background(), client, []string{"chat", "chat"}, strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	if uploads.Load() != 1 || submissions.Load() != 2 || f.calls.Load() != 0 || strings.Count(output.String(), "Accepted input") != 2 {
		t.Fatal("upload/inference replay or lost receipt", output.String(), uploads.Load(), submissions.Load(), f.calls.Load())
	}
	request := httptest.NewRequest("GET", "/", nil)
	scope := f.s.controlScope(request)
	manifest, err := f.s.agentManager.ImportedConversation(scope, "chat")
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := f.s.agentManager.ConversationInbox(scope, manifest.ConversationID, 0, 100)
	if err != nil || len(inbox.Items) != 2 {
		t.Fatal(inbox, err)
	}
	first, next := inbox.Items[0], inbox.Items[1]
	if first.Input.Content != "" || first.Input.Intent != "" || len(first.Input.Attachments) != 1 || len(first.Input.Images) != 1 || next.Input.Content != "Follow up" || len(next.Input.Attachments)+len(next.Input.Images) != 0 {
		t.Fatal(inbox)
	}
	prepared, err := f.s.prepareConversationTurn(context.Background(), scope, manifest.ConversationID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Prompt != "" || prepared.Config.Profile != nil || !strings.Contains(prepared.Config.InputEvidence, "12345") || prepared.Config.InputAttachments[0].Revision != first.Input.Attachments[0].Revision {
		t.Fatal("source-only evidence lost", prepared)
	}
	hasImage, err := api.ValidateChatMessages(prepared.Checkpoint.Messages)
	if err != nil || !hasImage {
		t.Fatal("source image missing", err)
	}
	restarted := agents.NewManagerWithPersistence(nil, nil, nil, f.agentDirectory)
	retained, found := restarted.GetTask(prepared.ID)
	if !found || retained.Prompt != "" || retained.Config.InputAttachments[0].Revision != first.Input.Attachments[0].Revision {
		t.Fatal("restart lost source", retained)
	}
	if _, err = f.s.sessionHandlers.manager.WithAccess(sessions.Access{All: true}).AppendExchange("chat", "model", "legacy", "must fail", nil); err != sessions.ErrCanonicalConversation {
		t.Fatal("legacy guard changed", err)
	}
}

func TestCanonicalSourceDraftCLIActualValidationRejectionAllowsCorrection(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	source := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(source, []byte("Owned source"), 0600); err != nil {
		t.Fatal(err)
	}
	var statuses []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/system":
			f.s.handleSystemIdentity(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v2/conversations"):
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/inputs") {
				recorder := httptest.NewRecorder()
				f.s.handleConversations(recorder, r)
				statuses = append(statuses, recorder.Code)
				for key, values := range recorder.Header() {
					w.Header()[key] = values
				}
				w.WriteHeader(recorder.Code)
				_, _ = w.Write(recorder.Body.Bytes())
				return
			}
			f.s.handleConversations(w, r)
		case strings.HasPrefix(r.URL.Path, "/v1/sessions"):
			f.s.sessionHandlers.HandleSessions(w, r)
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", nil)
	var output bytes.Buffer
	input := "/attach " + source + "\n" + strings.Repeat("x", 32001) + "\nCorrected source question\n/exit\n"
	if err := cli.RunPlainConversationChat(context.Background(), client, []string{"chat", "chat"}, strings.NewReader(input), &output, false); err != nil {
		t.Fatal(err)
	}
	scope := f.s.controlScope(httptest.NewRequest("GET", "/", nil))
	manifest, err := f.s.agentManager.ImportedConversation(scope, "chat")
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := f.s.agentManager.ConversationInbox(scope, manifest.ConversationID, 0, 100)
	if len(statuses) != 2 || statuses[0] != 400 || statuses[1] != 202 || err != nil || len(inbox.Items) != 1 || inbox.Items[0].Input.Content != "Corrected source question" || len(inbox.Items[0].Input.Attachments) != 1 || !strings.Contains(output.String(), "rejected before admission") {
		t.Fatal(statuses, inbox, err, output.String())
	}
}

func TestCanonicalSourceDraftCLIActualVisionRejectionRetainsDraft(t *testing.T) {
	f := newConversationFixture(t)
	f.s.config.EnableConversationPreview = true
	f.s.bindCanonicalConversationGuard()
	scopeRequest := httptest.NewRequest("GET", "/", nil)
	manifest, err := f.s.migrateConversation(scopeRequest, "chat")
	if err != nil {
		t.Fatal(err)
	}
	image := testSessionImage(t)
	data, err := base64.StdEncoding.DecodeString(strings.SplitN(image.DataURL, ",", 2)[1])
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "source.png")
	if err = os.WriteFile(imagePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/system":
			f.s.handleSystemIdentity(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v2/conversations"):
			f.s.handleConversations(w, r)
		case strings.HasPrefix(r.URL.Path, "/v1/sessions"):
			f.s.sessionHandlers.HandleSessions(w, r)
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "", nil)
	var output bytes.Buffer
	input := "/image " + imagePath + "\nInspect this image\n/images\n/remove-image all\nText without image\n/exit\n"
	if err = cli.RunCanonicalConversationChat(context.Background(), client, &sessions.Session{Name: "chat", CanonicalID: manifest.ConversationID}, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	inbox, err := f.s.agentManager.ConversationInbox(f.s.controlScope(scopeRequest), manifest.ConversationID, 0, 100)
	if err != nil || len(inbox.Items) != 1 || inbox.Items[0].Input.Content != "Text without image" || len(inbox.Items[0].Input.Images) != 0 || f.calls.Load() != 0 || !strings.Contains(output.String(), "rejected before admission") || strings.Count(output.String(), "Pending images (1/4)") != 2 {
		t.Fatal(inbox, err, f.calls.Load(), output.String())
	}
}
