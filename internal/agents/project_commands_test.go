package agents

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestProjectCommandsPinPreferencesAndRejectPreparedStaleContext(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	command := api.ProjectCommand{RequestID: "project-settings", TargetID: c.ProjectID, ExpectedRevision: 1, Kind: "configure_project", Payload: api.ProjectCommandPayload{Name: "Private research", ConfigurationTOML: "version=1\n[preferences]\nmodel='selected-project-model'\ndelegation=false\n"}}
	saved, err := m.ApplyProjectCommand(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Project.Preferences.Model != "selected-project-model" || saved.Project.Revision != 2 || len(saved.Project.ConfigurationDigest) != 64 {
		t.Fatal("missing project preference pin", saved)
	}
	if old, err := m.ProjectRevision(scope, c.ProjectID, 1); err != nil || old.Preferences.Model != "" || old.Name != "Private project" {
		t.Fatal("project edit rewrote pinned revision", old, err)
	}
	if retry, err := m.ApplyProjectCommand(scope, command); err != nil || retry.Revision != 2 {
		t.Fatal("project retry changed state", retry, err)
	}
	conflicting := command
	conflicting.Payload.Name = "Other name"
	if _, err = m.ApplyProjectCommand(scope, conflicting); !errors.Is(err, ErrRunConflict) {
		t.Fatal("project request identity reused", err)
	}
	config := taskConfig()
	config.ProjectID = c.ProjectID
	config.ProjectRevision = 1
	workCommand := ConversationCommand{RequestID: "prepared-stale-work", TargetID: c.ID, ExpectedRevision: 1, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Use current project context"}}}
	if _, err = r.AdmitConversationWork(scope, workCommand, "model", config); !errors.Is(err, ErrRunConflict) {
		t.Fatal("stale project context admitted", err)
	}
	config.ProjectRevision = 2
	work, err := r.AdmitConversationWork(scope, workCommand, "model", config)
	if err != nil || work.Work.ProjectRevision != 2 {
		t.Fatal("current project pin missing", work, err)
	}
	if _, err = m.ApplyProjectCommand(ControlScope{Workspace: scope.Workspace, Actor: "foreign"}, command); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("cross-actor project edit", err)
	}
}
