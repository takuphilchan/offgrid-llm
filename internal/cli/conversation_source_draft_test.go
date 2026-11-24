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
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type canonicalDraftFixture struct {
	mu           sync.Mutex
	client       *serviceclient.Client
	capabilities []string
	files        []sessions.Attachment
	commands     []api.ConversationInputCommand
	entries      []api.ConversationInboxEntry
	statuses     []int
	retainLost   bool
	withdraw     bool
	readFailure  int
	question     *api.ConversationQuestion
}

func newCanonicalDraftFixture(t *testing.T) *canonicalDraftFixture {
	t.Helper()
	f := &canonicalDraftFixture{capabilities: []string{"conversation-source-inputs-v1", "conversation-input-options-v1", "session-files-v1"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api/v2/system":
			json.NewEncoder(w).Encode(serviceclient.Identity{Product: "offgrid", Capabilities: f.capabilities})
		case "/v1/sessions/Notes":
			json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: "conversation"})
		case "/v1/sessions/Notes/attachments":
			if r.Method != "POST" {
				json.NewEncoder(w).Encode(map[string]any{"attachments": f.files})
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				t.Error(err)
			}
			sum := sha256.Sum256(data)
			item := sessions.Attachment{ID: fmt.Sprintf("file-%032x", len(f.files)+1), Name: header.Filename, Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Revision: strings.Repeat("a", 64), Status: "ready"}
			f.files = append(f.files, item)
			json.NewEncoder(w).Encode(map[string]any{"attachment": item})
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 3, PendingQuestion: f.question})
		case "/api/v2/conversations/conversation/inputs":
			if r.Method == "GET" {
				if len(f.commands) > 0 && f.readFailure != 0 {
					w.WriteHeader(f.readFailure)
					return
				}
				json.NewEncoder(w).Encode(api.ConversationInboxPage{Items: f.entries})
				return
			}
			var command api.ConversationInputCommand
			if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
				t.Error(err)
			}
			f.commands = append(f.commands, command)
			status := 202
			if len(f.statuses) > 0 {
				status = f.statuses[0]
				f.statuses = f.statuses[1:]
			}
			entry := api.ConversationInboxEntry{ID: fmt.Sprintf("input-%d", len(f.commands)), RequestID: command.RequestID, Input: command.Payload, State: "accepted"}
			if status == 202 || status == 503 && f.retainLost {
				f.entries = append(f.entries, entry)
			}
			if f.withdraw {
				f.capabilities = nil
			}
			w.WriteHeader(status)
			if status == 202 {
				json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Input: &entry})
			}
		default:
			t.Errorf("unexpected operation %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	var err error
	f.client, err = serviceclient.New(server.URL, "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *canonicalDraftFixture) chat(t *testing.T, input string) string {
	t.Helper()
	var output bytes.Buffer
	if err := RunCanonicalConversationChat(context.Background(), f.client, &sessions.Session{Name: "Notes", CanonicalID: "conversation"}, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func canonicalDraftFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("Retained source "+name), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func canonicalDraftImage(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(cliImageFile(t))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCanonicalSourceDraftSourceOnlyAndNoResubmission(t *testing.T) {
	f := newCanonicalDraftFixture(t)
	image := cliImageFile(t)
	selected, err := ReadChatImage(image)
	if err != nil {
		t.Fatal(err)
	}
	output := f.chat(t, "/attach "+canonicalDraftFile(t, "first.txt")+"\n/image "+image+"\n/attachments\n/images\n\nFollow up\n/exit\n")
	if len(f.files) != 1 || len(f.commands) != 2 {
		t.Fatal(f.files, f.commands, output)
	}
	first, next := f.commands[0].Payload, f.commands[1].Payload
	if first.Content != "" || first.Intent != "" || first.SourceSession != "Notes" || len(first.Attachments) != 1 || first.Attachments[0].Revision != f.files[0].Revision || !reflect.DeepEqual(first.Images, []api.SessionImage{selected}) {
		t.Fatal(first)
	}
	if next.Content != "Follow up" || conversationInputHasSources(next) || next.SourceSession != "" {
		t.Fatal("accepted sources were resubmitted", next)
	}
}

func TestCanonicalSourceDraftUnknownRetryIsDetachedFromEdits(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(fmt.Sprint(retained), func(t *testing.T) {
			f := newCanonicalDraftFixture(t)
			f.statuses = []int{503}
			f.retainLost = retained
			oldImage, retainedImage, newImage := canonicalDraftImage(t, "old.png"), canonicalDraftImage(t, "retained.png"), canonicalDraftImage(t, "new.png")
			oldFile, newFile := canonicalDraftFile(t, "old.txt"), canonicalDraftFile(t, "new.txt")
			output := f.chat(t, "/attach "+oldFile+"\n/image "+oldImage+"\n/image "+retainedImage+"\nOriginal question\n/remove-attachment all\n/remove-image 1\n/attach "+newFile+"\n/image "+newImage+"\nMust not replace unknown input\n/retry\nLater question\n/exit\n")
			want := 3
			if retained {
				want = 2
			}
			if len(f.files) != 2 || len(f.commands) != want || !strings.Contains(output, "previous source input has an unknown outcome") {
				t.Fatal(f.files, f.commands, output)
			}
			if !retained && (f.commands[0].RequestID != f.commands[1].RequestID || !reflect.DeepEqual(f.commands[0].Payload, f.commands[1].Payload)) {
				t.Fatal("retry changed input", f.commands)
			}
			first, last := f.commands[0].Payload, f.commands[len(f.commands)-1].Payload
			if first.Attachments[0].ID == last.Attachments[0].ID || last.Content != "Later question" || len(last.Images) != 2 || last.Images[1].Name != filepath.Base(newImage) || len(first.Images) != 2 || first.Images[0].Name != "old.png" || first.Images[1].Name != "retained.png" {
				t.Fatal("receipt erased later draft", f.commands)
			}
			if strings.Count(output, "Accepted input") != 2 {
				t.Fatal(output)
			}
		})
	}
}

func TestCanonicalSourceDraftReceiptRecoverySurvivesCapabilityWithdrawal(t *testing.T) {
	f := newCanonicalDraftFixture(t)
	f.statuses = []int{503}
	f.retainLost = true
	f.withdraw = true
	output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nQuestion\n/attachments\n/retry\n/exit\n")
	if len(f.commands) != 1 || !strings.Contains(output, "Accepted input input-1") {
		t.Fatal(f.commands, output)
	}
}

func TestCanonicalSourceDraftMissingReceiptCannotBypassWithdrawnCapability(t *testing.T) {
	f := newCanonicalDraftFixture(t)
	f.statuses = []int{503}
	f.withdraw = true
	output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nQuestion\n/retry\nMust not replace\n/exit\n")
	if len(f.commands) != 1 || strings.Contains(output, "Accepted input") || !strings.Contains(output, "Update the service") || !strings.Contains(output, "previous source input has an unknown outcome") {
		t.Fatal(f.commands, output)
	}
}

func TestCanonicalSourceDraftCapabilitiesAndClarification(t *testing.T) {
	t.Run("file capability missing", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.capabilities = []string{"conversation-source-inputs-v1"}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\n/retry-attachment 1\n/exit\n")
		if len(f.files) != 0 || len(f.commands) != 0 || strings.Count(output, "Update the service") != 2 {
			t.Fatal(output)
		}
	})
	t.Run("old service", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.capabilities = nil
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\n/image nonexistent.png\n/remove-attachment all\n/remove-image all\n/exit\n")
		if len(f.files) != 0 || len(f.commands) != 0 || strings.Count(output, "Update the service") != 2 || !strings.Contains(output, "Files removed") {
			t.Fatal(output)
		}
	})
	t.Run("source-only options missing", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.capabilities = []string{"conversation-source-inputs-v1", "session-files-v1"}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\n\nExplicit caption\n/exit\n")
		if len(f.files) != 1 || len(f.commands) != 1 || f.commands[0].Payload.Content != "Explicit caption" || !strings.Contains(output, "Update the service") {
			t.Fatal(output)
		}
	})
	t.Run("question", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.question = &api.ConversationQuestion{ID: "question-one", WorkID: "work-one", Content: "Include this?"}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nYes\n/message Discuss this source\nYes\n/exit\n")
		if len(f.commands) != 2 || !strings.Contains(output, "answers and corrections do not accept source changes") || f.commands[0].Payload.Intent != "message" || len(f.commands[0].Payload.Attachments) != 1 || f.commands[1].Payload.Intent != "answer" || conversationInputHasSources(f.commands[1].Payload) {
			t.Fatal(f.commands, output)
		}
	})
	t.Run("source-only discussion during a question", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.question = &api.ConversationQuestion{ID: "question-one", WorkID: "work-one", Content: "Include this?"}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\n/message\n/exit\n")
		if len(f.commands) != 1 || f.commands[0].Payload.Content != "" || f.commands[0].Payload.Intent != "message" || len(f.commands[0].Payload.Attachments) != 1 {
			t.Fatal(f.commands, output)
		}
	})
}

func TestCanonicalSourceDraftFailedUploadsAndSelectionsStayExplicit(t *testing.T) {
	t.Run("failed upload blocks input until removal", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		missing := filepath.Join(t.TempDir(), "missing.txt")
		output := f.chat(t, "/attach "+missing+"\nMust not omit missing source\n/remove-attachment all\nNow without a source\n/exit\n")
		if len(f.commands) != 1 || f.commands[0].Payload.Content != "Now without a source" || !strings.Contains(output, "an upload was not confirmed") {
			t.Fatal(f.commands, output)
		}
	})
	t.Run("saved selection needs preparation", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.files = []sessions.Attachment{{ID: "file-" + strings.Repeat("a", 32), Name: "saved.txt", Bytes: 20, Status: "failed"}}
		output := f.chat(t, "/use-attachment "+f.files[0].ID+"\nDo not omit unprepared evidence\n/remove-attachment all\nWithout the source\n/exit\n")
		if len(f.commands) != 1 || f.commands[0].Payload.Content != "Without the source" || !strings.Contains(output, "files need preparation") {
			t.Fatal(f.commands, output)
		}
	})
	t.Run("knowledge toggle remains a preference", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		output := f.chat(t, "/rag\n\nFirst question\nSecond question\n/rag\nThird question\n/exit\n")
		if len(f.commands) != 3 || !f.commands[0].Payload.UseKnowledge || !f.commands[1].Payload.UseKnowledge || f.commands[2].Payload.UseKnowledge {
			t.Fatal(f.commands, output)
		}
	})
}

func TestCanonicalSourceDraftRejectedAndUnknownOutcomesDiffer(t *testing.T) {
	for _, status := range []int{400, 413, 422} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newCanonicalDraftFixture(t)
			f.statuses = []int{status}
			output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nRejected question\nCorrected question\n/exit\n")
			if len(f.commands) != 2 || f.commands[0].RequestID == f.commands[1].RequestID || !strings.Contains(output, "rejected before admission") || len(f.commands[1].Payload.Attachments) != 1 {
				t.Fatal(f.commands, output)
			}
		})
	}
	t.Run("read rejection cannot resolve earlier uncertainty", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.statuses = []int{503}
		f.readFailure = 400
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nUnknown question\n/retry\nMust not replace\n/discard-retry\n/exit\n")
		if len(f.commands) != 1 || !strings.Contains(output, "previous source input has an unknown outcome") || !strings.Contains(output, "does not cancel service work") {
			t.Fatal(f.commands, output)
		}
	})
	t.Run("later post rejection cannot resolve earlier uncertainty", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.statuses = []int{503, 422}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nUnknown question\n/retry\nMust not replace\n/exit\n")
		if len(f.commands) != 2 || f.commands[0].RequestID != f.commands[1].RequestID || !strings.Contains(output, "previous source input has an unknown outcome") || strings.Contains(output, "rejected before admission") {
			t.Fatal(f.commands, output)
		}
	})
	t.Run("explicit discard permits new input without upload", func(t *testing.T) {
		f := newCanonicalDraftFixture(t)
		f.statuses = []int{503}
		output := f.chat(t, "/attach "+canonicalDraftFile(t, "source.txt")+"\nUnknown question\n/discard-retry\nNew explicit input\n/exit\n")
		if len(f.files) != 1 || len(f.commands) != 2 || f.commands[0].RequestID == f.commands[1].RequestID || !strings.Contains(output, "does not cancel service work") {
			t.Fatal(f.commands, output)
		}
	})
}
