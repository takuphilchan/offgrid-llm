package agents

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

func TestSkillMetadataReferencePreservesOwnedOlderRevision(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	files := []agentcontext.SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: checked\ndescription: Check the result\n---\nRetain this older guidance.\n")}}
	first, err := m.ImportSkill("workspace", "alice", "", 0, files, "")
	if err != nil {
		t.Fatal(err)
	}
	files[0].Content = append(files[0].Content, []byte("Newer guidance.\n")...)
	second, err := m.ImportSkill("workspace", "alice", first.ID, first.Revision, files, "")
	if err != nil {
		t.Fatal(err)
	}
	if !m.SkillReferenceAvailable("workspace", "alice", first.SkillReference) || !m.SkillReferenceAvailable("workspace", "alice", second.SkillReference) || m.SkillReferenceAvailable("workspace", "bob", first.SkillReference) || m.SkillReferenceAvailable("other", "alice", first.SkillReference) {
		t.Fatal("metadata ownership or immutable revision lost")
	}
	changed := first.SkillReference
	changed.Digest = strings.Repeat("0", 64)
	if m.SkillReferenceAvailable("workspace", "alice", changed) {
		t.Fatal("changed digest accepted")
	}
}

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
