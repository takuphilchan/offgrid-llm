package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestFileTransitionPreservesImagesAndRecoversCreate(t *testing.T) {
	var images []api.SessionImage
	_, _, err := ChatImageCommand("/image "+cliImageFile(t), &images)
	if err != nil {
		t.Fatal(err)
	}
	messages := []ChatMessage{{Role: "user", Content: api.SessionContent("describe café", images)}, {Role: "assistant", Content: "earlier answer"}}
	for _, mode := range []string{"success", "lost-response", "mismatch", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			var saved sessions.Session
			creates := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private" {
					t.Error("lost auth")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v2/system" {
					caps := []string{"session-files-v1"}
					if mode != "unsupported" {
						caps = append(caps, "session-history-seed-v1")
					}
					json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
					return
				}
				if r.Method == "POST" && r.URL.Path == "/v1/sessions" {
					creates++
					var req struct {
						Name    string             `json:"name"`
						Model   string             `json:"model_id"`
						History []sessions.Message `json:"history"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					saved = sessions.Session{Name: req.Name, ModelID: req.Model, Messages: req.History}
					if mode == "mismatch" {
						saved.Messages = nil
					}
					if mode == "lost-response" {
						w.WriteHeader(503)
						fmt.Fprint(w, `{"error":"lost response"}`)
						return
					}
				} else if r.Method != "GET" {
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				}
				json.NewEncoder(w).Encode(saved)
			}))
			defer s.Close()
			client, _ := serviceclient.New(s.URL, "private", s.Client())
			start := newFileConversationStarter(Environment{Service: ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}})
			files, err := start(context.Background(), "selected", messages)
			if mode == "unsupported" || mode == "mismatch" {
				if err == nil {
					t.Fatal("accepted incompatible history")
				}
				if mode == "unsupported" && creates != 0 {
					t.Fatal("created on old service")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if creates != 1 || files.Saved.Messages[0].Images[0].DataURL != images[0].DataURL || files.Saved.Messages[1].Content != "earlier answer" {
				t.Fatal("history loss")
			}
		})
	}
}

func TestPlainChatAttachesMidConversationWithoutLosingBufferedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document with spaces.txt")
	if err := os.WriteFile(path, []byte("exact source"), 0600); err != nil {
		t.Fatal(err)
	}
	var saved sessions.Session
	creates, uploads, turns := 0, 0, 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/system":
			fmt.Fprint(w, `{"product":"offgrid","capabilities":["session-files-v1","session-history-seed-v1"]}`)
		case r.URL.Path == "/v1/sessions":
			creates++
			var req struct {
				Name    string             `json:"name"`
				Model   string             `json:"model_id"`
				History []sessions.Message `json:"history"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			saved = sessions.Session{Name: req.Name, ModelID: req.Model, Messages: req.History}
			json.NewEncoder(w).Encode(saved)
		case strings.HasSuffix(r.URL.Path, "/attachments"):
			uploads++
			r.ParseMultipartForm(1024)
			defer r.MultipartForm.RemoveAll()
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "exact source" {
				t.Error("source changed")
			}
			digest := sha256.Sum256(data)
			json.NewEncoder(w).Encode(map[string]any{"attachment": sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Revision: strings.Repeat("b", 64), Name: filepath.Base(path), Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), Status: "ready"}})
		case strings.HasSuffix(r.URL.Path, "/generate"):
			turns++
			var req serviceclient.ConversationRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Content != "follow up" || len(req.Attachments) != 1 || len(saved.Messages) != 2 || saved.Messages[1].Content != "prior answer" {
				t.Errorf("lost buffered input/history/files: request=%+v history=%+v", req, saved.Messages)
			}
			msg := sessions.Message{Role: "assistant", Content: "new answer"}
			saved.Messages = append(saved.Messages, sessions.Message{Role: "user", Content: req.Content}, msg)
			saved.Turn = &sessions.Turn{ID: req.RequestID, Status: "completed"}
			w.Header().Set("Content-Type", "text/event-stream")
			data, _ := json.Marshal(map[string]any{"type": "done", "session": saved, "message": msg})
			fmt.Fprintf(w, "data: %s\n\n", data)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	client, _ := serviceclient.New(s.URL, "", s.Client())
	var out bytes.Buffer
	env := Environment{Stdin: strings.NewReader("/attachments\n/attach \"" + path + "\"\nfollow up\n/exit\n"), Stdout: &out, Stderr: &out, Service: ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
	history := []ChatMessage{{Role: "user", Content: "prior question"}, {Role: "assistant", Content: "prior answer"}}
	if err := runPlainChat(context.Background(), env, &config.Config{}, s.Client(), "model", false, nil, history, nil, nil); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || uploads != 1 || turns != 1 || !strings.Contains(out.String(), "Conversation preserved") {
		t.Fatal(creates, uploads, turns, out.String())
	}
}

func TestFileConversationRetryKeepsRequestAndDraft(t *testing.T) {
	calls := 0
	id := ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/system" {
			fmt.Fprint(w, `{"product":"offgrid","capabilities":["session-files-v1"]}`)
			return
		}
		calls++
		var req serviceclient.ConversationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Content != "original question" || len(req.Attachments) != 1 {
			t.Error("lost draft")
		}
		if id == "" {
			id = req.RequestID
		} else if id != req.RequestID {
			t.Error("replayed under a new request identity")
		}
		if calls == 1 {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"interrupted response"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		saved := sessions.NewSession("files", "model")
		saved.Turn = &sessions.Turn{ID: id, Status: "completed"}
		data, _ := json.Marshal(map[string]any{"type": "done", "session": saved, "message": sessions.Message{Role: "assistant", Content: "answer"}})
		fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	defer s.Close()
	client, _ := serviceclient.New(s.URL, "", s.Client())
	files := &ServiceFileConversation{Context: context.Background(), Client: client, Name: "files", Draft: ConversationFileDraft{Files: []sessions.Attachment{{ID: "file-" + strings.Repeat("a", 32), Revision: strings.Repeat("b", 64), Status: "ready"}}}}
	if _, err := files.Stream("model", "original question", nil, false, func(string) error { return nil }); err == nil || len(files.Draft.Files) != 1 {
		t.Fatal("lost failed draft", err)
	}
	if _, err := files.Stream("model", "original question", nil, false, func(string) error { return nil }); err != nil || len(files.Draft.Files) != 0 || calls != 2 {
		t.Fatal(err, calls)
	}
}
