package agents

import (
	"strings"
	"testing"
)

func TestProjectFileAcceptsPreferencesAndReferencesOnly(t *testing.T) {
	data := []byte("version = 1\n[preferences]\nmodel = 'installed-model'\ndelegation = false\n[[resources]]\nkind='instructions'\nid='AGENTS.md'\nrequired=true\n[[resources]]\nkind='skill'\nid='skill-123'\nrevision='4'\n")
	parsed, err := ParseProjectFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Preferences.Model != "installed-model" || parsed.Preferences.Delegation == nil || *parsed.Preferences.Delegation || len(parsed.Resources) != 2 || len(parsed.Digest) != 64 {
		t.Fatal("incorrect parsed configuration", parsed)
	}
	for _, bad := range []string{
		"version=2", "version=1\nhooks=['run this']", "version=1\n[authority]\ncomputer=true",
		"version=1\n[preferences]\ncredentials='secret'", "version=1\n[preferences]\nmodel='one'\nmodel='two'",
		"version=1\n[[resources]]\nkind='instructions'\nid='../outside'", "version=1\n[[resources]]\nkind='instructions'\nid='C:/outside'",
		"version=1\n[[resources]]\nkind='host_grant'\nid='forged'", "version=1\n[[resources]]\nkind='connection'\nid='existing'\ncommand='spawn'",
		strings.Repeat("# too large\n", 7000),
	} {
		if _, err := ParseProjectFile([]byte(bad)); err == nil {
			t.Errorf("accepted authority or malformed config: %.100s", bad)
		}
	}
}
