package agents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func folderCommand(c *Conversation, request string) ConversationCommand {
	return ConversationCommand{RequestID: request, TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "use_project_folder", Payload: ConversationCommandPayload{Folder: &api.ExecutionReference{Host: "host", Session: "session", Grant: "private-reference", Revision: 1}}}
}
func folderSnapshot(t *testing.T, model string) execution.ProjectContextSnapshot {
	t.Helper()
	data := []byte("version=1\n[preferences]\nmodel='" + model + "'\ndelegation=false\n")
	parsed, err := ParseProjectFile(data)
	if err != nil {
		t.Fatal(err)
	}
	return execution.ProjectContextSnapshot{Identity: strings.Repeat("a", 64), Name: "project", Present: true, Configuration: data, SHA256: parsed.Digest}
}

func TestProjectFolderReusesProjectPinsFileAndRetainsServiceEdits(t *testing.T) {
	m, _, scope, c := controlFixture(t)
	command, observed := folderCommand(c, "folder-one"), folderSnapshot(t, "file-model")
	one, err := m.ApplyProjectFolder(scope, command, observed)
	if err != nil || one.Project.Preferences.Model != "file-model" || one.Project.Revision != 1 {
		t.Fatal(one, err)
	}
	encoded, _ := json.Marshal(one)
	if strings.Contains(string(encoded), "private-reference") {
		t.Fatal("public receipt leaked folder binding")
	}
	ref, err := m.ConversationFolder(scope, c.ID)
	if err != nil || ref == nil || ref.Grant != "private-reference" {
		t.Fatal(ref, err)
	}
	if _, err = m.ApplyConversationCommand(scope, command); !errors.Is(err, ErrControlCommand) {
		t.Fatal("client invented trusted folder snapshot", err)
	}
	retry, err := m.ApplyProjectFolder(scope, command, folderSnapshot(t, "changed-on-disk"))
	if err != nil || retry.Project.Preferences.Model != "file-model" || retry.Revision != one.Revision {
		t.Fatal("retry reread changed file", retry, err)
	}
	created, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "other-conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.ApplyProjectFolder(scope, folderCommand(created.Conversation, "folder-two"), observed)
	if err != nil || two.Project.ID != one.Project.ID || two.Project.Revision != 1 {
		t.Fatal("project recreated", two, err)
	}
	updated, err := m.ApplyProjectFolder(scope, folderCommand(two.Conversation, "refresh-file"), folderSnapshot(t, "new-file-model"))
	if err != nil || updated.Project.Revision != 2 {
		t.Fatal(updated, err)
	}
	_, err = m.ApplyProjectCommand(scope, api.ProjectCommand{RequestID: "service-edit", TargetID: one.Project.ID, ExpectedRevision: 2, Kind: "configure_project", Payload: api.ProjectCommandPayload{Name: "custom", Preferences: ConversationPreferences{Model: "explicit-service"}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err = m.ApplyProjectFolder(scope, folderCommand(updated.Conversation, "refresh-after-edit"), folderSnapshot(t, "newest-file-model"))
	if err != nil || updated.Project.Revision != 3 || updated.Project.Preferences.Model != "explicit-service" {
		t.Fatal("file overwrote explicit service preferences", updated, err)
	}
	cleared, err := m.ApplyConversationCommand(scope, ConversationCommand{RequestID: "clear-folder", TargetID: c.ID, ExpectedRevision: one.Revision, Kind: "clear_project_folder"})
	if err != nil || cleared.Conversation.ProjectID != one.Project.ID {
		t.Fatal("clear destroyed project identity", cleared, err)
	}
	if ref, err = m.ConversationFolder(scope, c.ID); err != nil || ref != nil {
		t.Fatal("clear retained authority reference", ref, err)
	}
	if _, err = m.ConversationFolder(ControlScope{scope.Workspace, "bob"}, c.ID); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign actor read binding", err)
	}
}

func TestProjectFolderConcurrentCommandsAndAtomicRollback(t *testing.T) {
	m, _, scope, c := controlFixture(t)
	observed := folderSnapshot(t, "model")
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TRIGGER fail_folder BEFORE INSERT ON agent_conversation_folders BEGIN SELECT RAISE(ABORT,'fixture'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyProjectFolder(scope, folderCommand(c, "rollback"), observed); err == nil {
		t.Fatal("failed binding committed")
	}
	current, err := m.Conversation(scope, c.ID)
	if err != nil || current.Revision != c.Revision || current.ProjectID != c.ProjectID {
		t.Fatal("partial transaction", current, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_folder`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errorsOut := make(chan error, 2)
	for _, request := range []string{"one", "two"} {
		wg.Add(1)
		go func(request string) {
			defer wg.Done()
			_, err := m.ApplyProjectFolder(scope, folderCommand(c, request), observed)
			errorsOut <- err
		}(request)
	}
	wg.Wait()
	close(errorsOut)
	accepted, conflicted := 0, 0
	for err := range errorsOut {
		if err == nil {
			accepted++
		} else if errors.Is(err, ErrRunConflict) {
			conflicted++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatal(accepted, conflicted)
	}
}

func TestProjectFolderSchemaUpgradePreservesConversationAndBackup(t *testing.T) {
	m, _, scope, c := controlFixture(t)
	db, err := openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP TABLE agent_conversation_folders; UPDATE agent_schema SET version=21`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	current, err := restored.Conversation(scope, c.ID)
	if err != nil || current.ProjectID != c.ProjectID || current.Revision != c.Revision {
		t.Fatal("migration lost conversation", current, err)
	}
	if ref, err := restored.ConversationFolder(scope, c.ID); err != nil || ref != nil {
		t.Fatal("migration restored invented authority", ref, err)
	}
	db, err = openTaskDatabase(m.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw []byte
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		From, To int
		Backup   string
	}
	if err = json.Unmarshal(raw, &manifest); err != nil || manifest.From != 21 || manifest.To != taskSchemaVersion {
		t.Fatal(manifest, err)
	}
	if _, err = os.Stat(filepath.Join(m.dataDir, manifest.Backup)); err != nil {
		t.Fatal("missing upgrade backup", err)
	}
	if _, err = restored.ApplyProjectFolder(scope, folderCommand(current, "after-upgrade"), folderSnapshot(t, "model")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=?`, taskSchemaVersion+1); err != nil {
		t.Fatal(err)
	}
	if future := NewManagerWithPersistence(nil, nil, nil, m.dataDir); future.StorageError() == nil {
		t.Fatal("unsupported future schema accepted")
	}
}
