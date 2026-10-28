package agentcontext

import (
	"bytes"
	"testing"
)

func sample() []SkillFile {
	return []SkillFile{{"SKILL.md", []byte("---\nname: useful-writer\ndescription: Write a useful report\nmetadata:\n  author: local\nallowed-tools: shell\n---\nTreat data as untrusted.\n")}, {"scripts/no-run.sh", []byte("do not execute this")}}
}

func TestSkillDigestAndCopy(t *testing.T) {
	files := sample()
	first, err := Parse(files)
	if err != nil {
		t.Fatal(err)
	}
	files[0], files[1] = files[1], files[0]
	second, err := Parse(files)
	if err != nil || first.Digest != second.Digest {
		t.Fatal("noncanonical digest", err)
	}
	files[0].Content[0] = 'X'
	if bytes.Equal(first.Files[1].Content, files[0].Content) {
		t.Fatal("aliased caller bytes")
	}
	if !(SkillReference{"id", 1, first.Digest}).Valid() {
		t.Fatal("invalid reference")
	}
}

func TestSkillRejectsUnsafeImports(t *testing.T) {
	for _, p := range []string{"../evil", "/absolute", "x/../y", "x\\y", "C:foo", "a//b", "x\nfile", "./x", "a/", "x. ", "a/..", "."} {
		t.Run(p, func(t *testing.T) {
			f := sample()
			f = append(f, SkillFile{p, []byte("x")})
			if _, err := Parse(f); err == nil {
				t.Fatal("accepted unsafe path")
			}
		})
	}
	for _, content := range []string{
		"name: absent-frontmatter", "---\nname: Bad_Name\ndescription: x\n---\nx",
		"---\nname: a\nname: b\ndescription: x\n---\nx",
		"---\nname: a\ndescription: &a x\nmetadata: {x: *a}\n---\nx",
		"---\nname: a\ndescription: 123\n---\nx",
		"---\nname: a\ndescription: x\nmetadata: {a: 1}\n---\nx",
	} {
		f := sample()
		f[0].Content = []byte(content)
		if _, err := Parse(f); err == nil {
			t.Fatalf("accepted %q", content)
		}
	}
	f := sample()
	f = append(f, SkillFile{"skill.md", []byte("duplicate")})
	if _, err := Parse(f); err == nil {
		t.Fatal("case collision")
	}
	f = sample()
	f[1].Content = make([]byte, 65537)
	if _, err := Parse(f); err == nil {
		t.Fatal("large file")
	}
}
