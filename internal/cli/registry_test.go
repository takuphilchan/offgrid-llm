package cli

import "testing"

func TestCommandAliasRegistry(t *testing.T) {
	if len(commandNames) != 47 {
		t.Fatalf("root command count = %d, want 47", len(commandNames))
	}
	seen := make(map[string]bool)
	for _, command := range commandNames {
		for _, name := range append([]string{command.Name}, command.Aliases...) {
			if seen[name] {
				t.Errorf("duplicate alias %q", name)
			}
			seen[name] = true
			if got, ok := CanonicalCommand(name); !ok || got != command.Name {
				t.Errorf("resolve %q = %q, %v; want %q", name, got, ok, command.Name)
			}
		}
	}
	if _, ok := CanonicalCommand("not-a-command"); ok {
		t.Fatal("unknown command resolved")
	}
}
