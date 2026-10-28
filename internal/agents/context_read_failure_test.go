package agents

import (
	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"testing"
)

func TestContextReadsHavePinnedNoMutationIntent(t *testing.T) {
	for _, name := range []string{"skill_load", "memory_read", "skill_draft", "memory_write"} {
		d, _ := agentcontext.Descriptor(name)
		cp := &Checkpoint{ExecutingCall: "call", ReadOnlyCall: "call", ReadOnlyCapability: &d, Calls: []api.ToolCall{{ID: "call", Function: api.FunctionCall{Name: name}}}}
		want := name == "skill_load" || name == "memory_read"
		if validReadOnlyIntent(cp) != want {
			t.Fatal("wrong effect classification", name)
		}
		if want {
			d.Source = "untrusted-provider"
			if validReadOnlyIntent(cp) {
				t.Fatal("unreviewed descriptor trusted", name)
			}
			d, _ = agentcontext.Descriptor(name)
			cp.ReadOnlyCapability = nil
			if validReadOnlyIntent(cp) {
				t.Fatal("old checkpoint retroactively authorized", name)
			}
		}
	}
}
