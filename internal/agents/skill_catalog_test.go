package agents

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

func TestSkillCatalogMetadataIsScopedAndContentLoadsByPinnedDigest(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: reviewed\ndescription: Review the output\n---\nKeep references.\n")}, {Path: "references/checks.md", Content: []byte("Private supporting content")}}
	item, err := m.ImportSkill("workspace", "alice", "", 0, files, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.ImportSkill("workspace", "bob", "", 0, files, ""); err != nil {
		t.Fatal(err)
	}
	items, err := m.SkillCatalog("workspace", "alice")
	if err != nil || len(items) != 1 || items[0].SkillReference != item.SkillReference || len(items[0].ResourcePaths) != 2 {
		t.Fatal(items, err)
	}
	_, bundle, err := m.GetSkill("workspace", "alice", items[0].SkillReference)
	if err != nil || len(bundle.Files) != 2 {
		t.Fatal(bundle, err)
	}
	if _, _, err = m.GetSkill("workspace", "bob", items[0].SkillReference); err == nil {
		t.Fatal("catalog identity granted foreign access")
	}
}
