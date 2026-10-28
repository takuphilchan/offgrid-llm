package agentcontext

import (
	"fmt"
	"testing"
)

func contractSkill(definition string) []SkillFile {
	return []SkillFile{{Path: "SKILL.md", Content: []byte("---\nname: contract-test\ndescription: Test formatting\nresponse_contract: " + definition + "\n---\nReturn the requested text.")}}
}

func TestResponseContractImportAndLegacyMetadata(t *testing.T) {
	valid := `{version: 1, pattern: 'café 日本語\r?\n', max_bytes: 64}`
	bundle, err := Parse(contractSkill(valid))
	if err != nil || bundle.ResponseContract == nil {
		t.Fatal(bundle, err)
	}
	c := bundle.ResponseContract
	for _, text := range []string{"café 日本語\n", "café 日本語\r\n"} {
		if !c.Matches(text) {
			t.Fatal("exact Unicode rejected")
		}
	}
	for _, text := range []string{"preface café 日本語\n", "café 日本語\nextra", string([]byte{0xff})} {
		if c.Matches(text) {
			t.Fatal("nonmatching answer accepted")
		}
	}
	for _, bad := range []string{`null`, `{version: 2, pattern: 'x', max_bytes: 4}`, `{version: 1, pattern: '[', max_bytes: 4}`, `{version: 1, pattern: 'x', max_bytes: 32769}`, `{version: 1, pattern: 'x', max_bytes: 4, command: 'run'}`, `{version: 1, version: 1, pattern: 'x', max_bytes: 4}`} {
		if _, err := Parse(contractSkill(bad)); err == nil {
			t.Fatal("bad contract accepted", bad)
		}
	}
	// The previously inert key stays inert even if invalid under today's parser.
	for _, inert := range []string{valid, `{version: 999, arbitrary: old-data}`} {
		old, err := ParseStored(contractSkill(inert), nil)
		if err != nil || old.ResponseContract != nil {
			t.Fatal("old revision gained behavior", err)
		}
	}
	if _, err := ParseStored(contractSkill(valid), &ResponseContract{Version: 1, Pattern: "other", MaxBytes: 64}); err == nil {
		t.Fatal("changed stored contract accepted")
	}
	tooSmall := ResponseContract{Version: 1, Pattern: ".*", MaxBytes: 1}
	if tooSmall.Matches("é") {
		t.Fatal("byte limit counted characters")
	}
	if _, err := Parse(contractSkill(fmt.Sprintf(`{version: 1, pattern: '%s', max_bytes: 4}`, "(?=x)"))); err == nil {
		t.Fatal("non-RE2 accepted")
	}
}
