package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestContextCLIUsesServiceAndExplicitInputReader(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: note-writer\ndescription: Local instructions\n---\nDo not execute scripts.\n")}}
	reads, calls := 0, 0
	env.Service.ReadSkillDirectory = func(folder string) ([]agentcontext.SkillFile, error) {
		reads++
		if folder != "chosen folder" {
			t.Fatal(folder)
		}
		return files, nil
	}
	env.HTTP = &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"product":"offgrid","capabilities":["agent-context-resources-v1"]}`
		if r.URL.Path != "/api/v2/system" {
			calls++
			if r.URL.Path != "/api/v2/agent-skills" || r.Method != "POST" {
				t.Fatal(r.Method, r.URL)
			}
			var payload struct {
				Files []agentcontext.SkillFile `json:"files"`
			}
			if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload.Files) != 1 || !bytes.Equal(payload.Files[0].Content, files[0].Content) {
				t.Fatal(payload)
			}
			body = `{"id":"skill-owned","revision":1}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture:11611", "", env.HTTP) }
	if err := RunAgentCommand(context.Background(), []string{"skills", "import", "chosen folder"}, env); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || calls != 1 || !strings.Contains(out.String(), "skill-owned") {
		t.Fatal(reads, calls, out.String())
	}
	if err := RunAgentCommand(context.Background(), []string{"skills", "show", "../escape", "1", strings.Repeat("a", 64)}, env); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if reads != 1 || calls != 1 {
		t.Fatal("invalid command performed effects")
	}
}

func TestContextCLINeverReadsAmbientFilesWithoutAdapter(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New("http://fixture:11611", "", &http.Client{Transport: localRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"product":"offgrid","capabilities":["agent-context-resources-v1"]}`))}, nil
		})})
	}
	if err := RunAgentCommand(context.Background(), []string{"skills", "import", "."}, env); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatal(err)
	}
	env.Stdin = strings.NewReader(`{"operation":"edit","label":"Useful","content":"text","source_job":"forged"}`)
	if err := RunAgentCommand(context.Background(), []string{"memory", "put", "profile", "-"}, env); err == nil {
		t.Fatal("accepted provenance injection")
	}
}

func TestReadSkillDirectoryIsBoundedAndInert(t *testing.T) {
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "SKILL.md"), []byte("---\nname: note-writer\ndescription: A note writer\n---\nUse references.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "scripts", "inert.sh"), []byte("exit 9"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := ReadSkillDirectory(folder)
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	if err = os.WriteFile(filepath.Join(folder, "large.bin"), make([]byte, 65537), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadSkillDirectory(folder); err == nil {
		t.Fatal("oversized import accepted")
	}
}
