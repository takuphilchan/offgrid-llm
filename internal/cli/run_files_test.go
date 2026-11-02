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

func TestRunFilesUsesAuthenticatedServiceInEveryPresentation(t *testing.T) {
	for _, mode := range []string{"json", "plain", "tui"} {
		t.Run(mode, func(t *testing.T) {
			content := []byte("Shipment café 日本語 12345")
			path := filepath.Join(t.TempDir(), "source with spaces.txt")
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(content)
			item := sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: filepath.Base(path), Bytes: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), Revision: strings.Repeat("b", 64), Status: "ready"}
			creates, uploads, turns := 0, 0, 0
			saved := &sessions.Session{Name: "demo-files", ModelID: "exact-model", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private" {
					t.Error("lost service credentials")
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v2/system":
					fmt.Fprint(w, `{"product":"offgrid","capabilities":["session-files-v1"]}`)
				case "/v1/sessions":
					creates++
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["name"] != saved.Name || body["model_id"] != saved.ModelID {
						t.Error("silently changed model or session")
					}
					_ = json.NewEncoder(w).Encode(saved)
				case "/v1/sessions/demo-files/attachments":
					uploads++
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						return
					}
					defer r.MultipartForm.RemoveAll()
					file, header, err := r.FormFile("file")
					if err != nil {
						t.Error(err)
						return
					}
					defer file.Close()
					data, _ := io.ReadAll(file)
					if !bytes.Equal(content, data) || header.Filename != item.Name {
						t.Error("changed source")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"attachment": item})
				case "/v1/sessions/demo-files/generate":
					turns++
					var input serviceclient.ConversationRequest
					_ = json.NewDecoder(r.Body).Decode(&input)
					if input.Content != "What number?" || input.Model != "exact-model" || !input.Stream || !input.Durable || len(input.Attachments) != 1 || input.Attachments[0].Revision != item.Revision {
						t.Errorf("wrong request %+v", input)
					}
					saved.Messages = []sessions.Message{{Role: "user", Content: input.Content, Attachments: input.Attachments}, {Role: "assistant", Content: "12345"}}
					saved.Turn = &sessions.Turn{ID: input.RequestID, Status: "completed"}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"type\":\"delta\",\"delta\":\"12345\"}\n\n")
					done, _ := json.Marshal(map[string]any{"type": "done", "session": saved, "message": saved.Messages[1]})
					fmt.Fprintf(w, "data: %s\n\n", done)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "private", nil)
			var output, diagnostics bytes.Buffer
			env := Environment{Stdin: strings.NewReader("What number?\n/exit\n"), Stdout: &output, Stderr: &diagnostics, Service: ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
			args := []string{"exact-model", "--file", path, "--save", saved.Name}
			if mode == "json" {
				env.Options.JSON = true
				args = append(args, "--prompt", "What number?")
			}
			if mode == "tui" {
				env.Interactive.ShouldUseTUI = func() bool { return true }
				env.Interactive.RunTUI = func(ctx context.Context, state *InteractiveTUIState) error {
					if state.Files == nil || len(state.Files.Draft.Files) != 1 {
						t.Fatal("TUI lost prepared source")
					}
					result, err := state.Files.Stream(state.ModelName, "What number?", nil, false, func(delta string) error { _, e := output.WriteString(delta); return e })
					if err == nil && (len(state.Files.Draft.Files) != 0 || len(result.Messages[0].Attachments) != 1) {
						t.Error("TUI did not preserve completed source")
					}
					return err
				}
			}
			if err := RunInteractiveCommand(context.Background(), args, env); err != nil {
				t.Fatal(err)
			}
			if creates != 1 || uploads != 1 || turns != 1 || !strings.Contains(output.String(), "12345") {
				t.Fatalf("counts %d/%d/%d output %s", creates, uploads, turns, output.String())
			}
			if mode == "json" && !json.Valid(output.Bytes()) {
				t.Fatal("diagnostics corrupted JSON output")
			}
			if !strings.Contains(diagnostics.String(), "offgrid session chat demo-files") {
				t.Fatal("missing recovery identity")
			}
		})
	}
}

func TestRunFilesRejectsBadInputBeforeServiceMutation(t *testing.T) {
	env := Environment{Stdout: io.Discard, Stderr: io.Discard, Stdin: strings.NewReader("")}
	for _, args := range [][]string{{"model", "--file"}, {"model", "--file", "missing"}, {"model", "--file", "x", "--save", "one", "--load", "two"}} {
		if err := RunInteractiveCommand(context.Background(), args, env); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
