package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func TestAgentDefaultFilesScopeAtomicReplacementAndClear(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preferences")
	store := agentDefaultFiles{directory: func() (string, error) { return root, nil }}
	scope := cli.AgentDefaultScope{Origin: "http://localhost:11611", Workspace: "workspace", Actor: "user:alice"}
	value, err := store.Load(scope)
	if err != nil || value != nil {
		t.Fatal(value, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("read created state", err)
	}
	record := cli.AgentDefault{Version: 1, Scope: scope, ProfileID: "profile-a", Revision: 2}
	for _, rev := range []int{2, 5} {
		record.Revision = rev
		if err := store.Save(scope, &record); err != nil {
			t.Fatal(err)
		}
		got, err := store.Load(scope)
		if err != nil || got.Revision != rev {
			t.Fatal(got, err)
		}
	}
	for _, changed := range []cli.AgentDefaultScope{{Origin: "http://other:11611", Workspace: scope.Workspace, Actor: scope.Actor}, {Origin: scope.Origin, Workspace: "other", Actor: scope.Actor}, {Origin: scope.Origin, Workspace: scope.Workspace, Actor: "user:bob"}} {
		got, err := store.Load(changed)
		if err != nil || got != nil {
			t.Fatal("scope leak", got, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
	path, _ := store.path(scope)
	data, _ := os.ReadFile(path)
	for _, word := range []string{"token", "credential", "grant", "password", "settings"} {
		if strings.Contains(string(data), word) {
			t.Fatal("unexpected authority data", string(data))
		}
	}
	if err := store.Save(scope, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(scope, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestAgentDefaultFilesRejectCorruptForeignOrOversizedRecords(t *testing.T) {
	root := t.TempDir()
	store := agentDefaultFiles{directory: func() (string, error) { return root, nil }}
	scope := cli.AgentDefaultScope{Origin: "http://localhost:11611", Workspace: "w", Actor: "user:a"}
	path, _ := store.path(scope)
	for _, data := range []string{"{", strings.Repeat(" ", 5000), `{"version":2}`, `{"version":1,"scope":{"origin":"http://other:11611","workspace":"w","actor":"user:a"},"profile_id":"p","revision":1}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if value, err := store.Load(scope); err == nil || value != nil {
			t.Fatal("bad preference accepted", value, err)
		}
	}
	if err := store.Save(scope, &cli.AgentDefault{Version: 1, Scope: scope, ProfileID: "p", Revision: 0}); err == nil {
		t.Fatal("zero revision accepted")
	}
}
