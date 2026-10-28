package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// Real loopback HTTP clients, isolated application state and explicit disposable
// input files. No installed service, model, host grant or desktop is involved.
func TestContextCLIToOwnedHTTPService(t *testing.T) {
	s := profileFixture(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/system", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"product": "offgrid", "capabilities": []string{"agent-context-resources-v1"}})
	})
	mux.HandleFunc("/api/v2/agent-skills", s.handleAgentSkills)
	mux.HandleFunc("/api/v2/agent-skills/", s.handleAgentSkills)
	mux.HandleFunc("/api/v2/agent-memory/", s.handleAgentMemory)
	mux.HandleFunc("/api/v2/agent-profiles/", s.handleAgentProfiles)
	service := httptest.NewServer(mux)
	defer service.Close()
	client, err := serviceclient.New(service.URL, "", service.Client())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	env := cli.Environment{Stdout: &out, Stderr: io.Discard, Stdin: strings.NewReader(""), Service: cli.ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }, ReadSkillDirectory: cli.ReadSkillDirectory}}
	run := func(args ...string) {
		t.Helper()
		out.Reset()
		if err := cli.RunAgentCommand(context.Background(), args, env); err != nil {
			t.Fatal(args, err)
		}
	}
	folder := t.TempDir()
	if err = os.WriteFile(filepath.Join(folder, "SKILL.md"), []byte("---\nname: note-writer\ndescription: Write notes\n---\nPreserve exact Unicode — café 日本語.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("skills", "import", folder)
	var skill agents.AgentSkill
	if json.Unmarshal(out.Bytes(), &skill) != nil || skill.ID == "" {
		t.Fatal(out.String())
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, "local-admin", "", 0, agents.ProfileSettings{Name: "Writer", Model: "model", MaxIterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	run("skills", "show", skill.ID, "1", skill.Digest)
	if !strings.Contains(out.String(), "SKILL.md") || !strings.Contains(out.String(), `"executable":false`) {
		t.Fatal(out.String())
	}
	run("skills", "attach", p.ID, "1", skill.ID, "1", skill.Digest)
	var updated agents.AgentProfile
	if json.Unmarshal(out.Bytes(), &updated) != nil || updated.Revision != 2 || len(updated.Settings.Skills) != 1 {
		t.Fatal(out.String())
	}
	run("memory", "show", p.ID)
	if !strings.Contains(out.String(), `"enabled":false`) {
		t.Fatal("implicit consent", out.String())
	}
	run("memory", "consent", p.ID, "0", "on")
	env.Stdin = strings.NewReader(`{"operation":"remember-format","expected_revision":0,"label":"Format","content":"Concise — café 日本語"}`)
	run("memory", "put", p.ID, "-")
	var entry agents.MemoryEntry
	if json.Unmarshal(out.Bytes(), &entry) != nil || entry.ID == "" {
		t.Fatal(out.String())
	}
	run("memory", "show", p.ID)
	if !strings.Contains(out.String(), entry.Content) {
		t.Fatal(out.String())
	}
	run("memory", "consent", p.ID, "1", "off")
	run("memory", "delete", p.ID, entry.ID, strconv.Itoa(entry.Revision))
	run("memory", "show", p.ID)
	if strings.Contains(out.String(), entry.Content) {
		t.Fatal("deleted data retrieved", out.String())
	}
	if len(s.agentManager.ListTasks()) != 0 {
		t.Fatal("resource administration submitted work")
	}
}
