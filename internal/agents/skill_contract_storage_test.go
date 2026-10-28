package agents

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

func TestReviewedSkillContractPersistsWithoutUpgradingLegacy(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: reviewed-contract\ndescription: Fixture\nresponse_contract: {version: 1, pattern: 'done', max_bytes: 4}\n---\nReturn done.")}}
	first, err := m.ImportSkill("work", "alice", "", 0, files, "")
	if err != nil || first.ResponseContract == nil {
		t.Fatal(first, err)
	}
	got, bundle, err := m.GetSkill("work", "alice", first.SkillReference)
	if err != nil || got.ResponseContract == nil || *got.ResponseContract != *first.ResponseContract {
		t.Fatal(got, err)
	}
	// Simulate an old persisted bundle: same original file bytes/digest, no
	// reviewed contract metadata. Newly recognized frontmatter stays inert.
	bundle.ResponseContract = nil
	raw, _ := json.Marshal(bundle)
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE agent_context_skill_revisions SET bundle=? WHERE skill_id=? AND revision=1`, raw, first.ID)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	got, bundle, err = m.GetSkill("work", "alice", first.SkillReference)
	if err != nil || got.ResponseContract != nil || bundle.ResponseContract != nil {
		t.Fatal("legacy behavior changed", got, err)
	}
	second, err := m.ImportSkill("work", "alice", first.ID, 1, files, "")
	if err != nil || second.ResponseContract == nil || second.Revision != 2 {
		t.Fatal(second, err)
	}
	old, _, err := m.GetSkill("work", "alice", first.SkillReference)
	if err != nil || old.ResponseContract != nil {
		t.Fatal("old revision upgraded", err)
	}
}
