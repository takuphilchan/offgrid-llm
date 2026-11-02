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
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestOrdinarySessionFilesUseSharedDurableContract(t *testing.T) {
	source := []byte("source café 日本語 12345")
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	item := sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: "notes.txt", Bytes: int64(len(source)), SHA256: hex.EncodeToString(sum[:]), Status: "ready", Revision: strings.Repeat("c", 64)}
	sends, uploads := 0, 0
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("missing service auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/system":
			_ = json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": []string{"session-files-v1"}})
		case "/v1/sessions/chat":
			_ = json.NewEncoder(w).Encode(&sessions.Session{Name: "chat", ModelID: "model", CreatedAt: time.Now(), UpdatedAt: time.Now()})
		case "/v1/sessions/chat/attachments":
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode(map[string]any{"attachments": []sessions.Attachment{item}})
				return
			}
			uploads++
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()
			data, _ := io.ReadAll(f)
			if !bytes.Equal(data, source) {
				t.Error("file bytes changed")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"attachment": item})
		case "/v1/sessions/chat/generate":
			sends++
			var input serviceclient.ConversationRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			if !input.Stream || !input.Durable || len(input.Attachments) != 1 || input.Attachments[0].ID != item.ID || input.Attachments[0].Revision != item.Revision {
				t.Errorf("wrong turn contract: %+v", input)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"delta\",\"delta\":\"12345\"}\n\n")
			if fail {
				return
			}
			saved := &sessions.Session{Name: "chat", Turn: &sessions.Turn{ID: input.RequestID, Status: "completed"}}
			done, _ := json.Marshal(map[string]any{"type": "done", "session": saved, "message": sessions.Message{Role: "assistant", Content: "12345"}})
			fmt.Fprintf(w, "data: %s\n\n", done)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "private", nil)
	var out bytes.Buffer
	if err := RunPlainConversationTurn(context.Background(), client, []string{"send", "chat", "--file", file, "What is the code?"}, &out, true); err != nil {
		t.Fatal(err)
	}
	if uploads != 1 || sends != 1 || !strings.Contains(out.String(), `"content":"12345"`) {
		t.Fatalf("missing persisted result: %s", out.String())
	}
	draft := ConversationFileDraft{Files: []sessions.Attachment{item}}
	fail = true
	out.Reset()
	err := RunPlainConversationTurn(context.Background(), client, []string{"send", "chat", "--attachment", item.ID, "What is the code?"}, &out, false)
	if err == nil || uploads != 1 || sends != 2 {
		t.Fatal("incomplete stream accepted or source uploaded again")
	}
	handled, _, err := draft.Command(context.Background(), client, "chat", "/remove-attachment 1")
	if !handled || err != nil || len(draft.Files) != 0 || sends != 2 {
		t.Fatal("draft removal dispatched work")
	}
}

func TestSessionFileCommandsRejectInvalidLocalInput(t *testing.T) {
	directory := t.TempDir()
	if _, _, err := ReadChatFile(directory); err == nil {
		t.Fatal("directory accepted")
	}
	if _, _, err := ReadChatFile(filepath.Join(directory, "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
	var out bytes.Buffer
	if err := RunSessionService(context.Background(), nil, []string{"send"}, &out, false, nil); err == nil {
		t.Fatal("missing session accepted")
	}
	draft := ConversationFileDraft{}
	for _, command := range []string{"/attach", "/remove-attachment 1", "/attachments bad"} {
		if _, _, err := draft.Command(context.Background(), nil, "chat", command); err == nil {
			t.Fatalf("invalid command accepted: %s", command)
		}
	}
}
