package agents

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

func skillFiles(body string) []agentcontext.SkillFile {
	return []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: test-skill\ndescription: A local skill\n---\n" + body)}, {Path: "scripts/inert.sh", Content: []byte("never run this")}}
}

func TestSkillOwnedImmutableRevisionsAndIntegrity(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	first, err := m.ImportSkill("work", "alice", "", 0, skillFiles("first"), "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.ImportSkill("work", "alice", first.ID, 1, skillFiles("second"), "")
	if err != nil || second.Revision != 2 || first.Digest == second.Digest {
		t.Fatal(second, err)
	}
	_, bundle, err := m.GetSkill("work", "alice", first.SkillReference)
	if err != nil || string(bundle.Files[0].Content) != string(skillFiles("first")[0].Content) {
		t.Fatal(bundle, err)
	}
	for _, scope := range [][2]string{{"work", "bob"}, {"other", "alice"}} {
		if _, _, err = m.GetSkill(scope[0], scope[1], first.SkillReference); !errors.Is(err, ErrContextNotFound) {
			t.Fatal("foreign skill", err)
		}
		items, e := m.ListSkills(scope[0], scope[1])
		if e != nil || len(items) != 0 {
			t.Fatal(items, e)
		}
	}
	settings := ProfileSettings{Name: "Worker", Model: "local", MaxIterations: 10, Skills: []agentcontext.SkillReference{first.SkillReference}}
	p, err := m.SaveProfile("work", "alice", "", 0, settings)
	if err != nil || p.Settings.Skills[0] != first.SkillReference {
		t.Fatal(p, err)
	}
	if _, err = m.SaveProfile("work", "bob", "", 0, settings); !errors.Is(err, ErrInvalidProfile) {
		t.Fatal("foreign attachment", err)
	}
	if _, err = m.ImportSkill("work", "alice", first.ID, 1, skillFiles("stale"), ""); !errors.Is(err, ErrContextConflict) {
		t.Fatal(err)
	}
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	encoded, _ := json.Marshal(map[string]any{"files": skillFiles("corrupt"), "name": "test-skill", "description": "A local skill", "digest": first.Digest})
	if _, err = db.Exec(`UPDATE agent_context_skill_revisions SET bundle=? WHERE skill_id=? AND revision=1`, encoded, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.GetSkill("work", "alice", first.SkillReference); err == nil {
		t.Fatal("corrupt bytes accepted")
	}
}

func TestContextStorageFailureNeverCreatesReplacementResources(t *testing.T) {
	m := NewManager(nil, nil, nil)
	if _, err := m.ImportSkill("work", "alice", "", 0, skillFiles("data"), ""); !errors.Is(err, ErrRunStorage) {
		t.Fatal(err)
	}
	if _, err := m.GetMemoryConsent("work", "alice", "profile"); !errors.Is(err, ErrRunStorage) {
		t.Fatal(err)
	}
	if _, err := m.WriteMemory("work", "alice", "profile", 0, MemoryWrite{Operation: "test", Label: "label", Content: "content"}); !errors.Is(err, ErrRunStorage) {
		t.Fatal(err)
	}
}

func TestSkillDraftRequiresOwnedRootAndIsNotAttachable(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	if _, err := m.ImportSkill("work", "alice", "", 0, skillFiles("draft"), "missing"); !errors.Is(err, ErrContextNotFound) {
		t.Fatal(err)
	}
	m.tasks["root"] = &Task{ID: "root", Actor: "alice", Config: AgentConfig{Profile: &AgentProfile{Workspace: "work"}}}
	draft, err := m.ImportSkill("work", "alice", "", 0, skillFiles("draft"), "root")
	if err != nil || !draft.Draft {
		t.Fatal(draft, err)
	}
	_, err = m.SaveProfile("work", "alice", "", 0, ProfileSettings{Name: "Worker", Model: "local", MaxIterations: 10, Skills: []agentcontext.SkillReference{draft.SkillReference}})
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatal("draft activated", err)
	}
	m.tasks["root"].ParentID = "parent"
	if _, err = m.ImportSkill("work", "alice", "", 0, skillFiles("draft"), "root"); !errors.Is(err, ErrContextNotFound) {
		t.Fatal("child draft", err)
	}
}
