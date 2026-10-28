package execution

import (
	"encoding/json"
	"testing"
)

func TestProcessArgumentHelpPreservesSavedCapabilityIdentity(t *testing.T) {
	before, _ := Descriptor("workspace_process_start")
	for _, shown := range ModelTools() {
		if shown.Function.Name != before.Name {
			continue
		}
		arguments := shown.Function.Parameters["properties"].(map[string]any)["arguments"].(map[string]any)
		if arguments["description"] == nil {
			t.Fatal("argv semantics missing")
		}
		delete(arguments, "description")
		actual, _ := json.Marshal(shown)
		for _, original := range Tools() {
			if original.Function.Name == before.Name {
				expected, _ := json.Marshal(original)
				if string(expected) != string(actual) {
					t.Fatal("presentation changed the registered invocation grammar")
				}
			}
		}
	}
	after, _ := Descriptor("workspace_process_start")
	if before != after {
		t.Fatal("help invalidated saved capability identity")
	}
}
