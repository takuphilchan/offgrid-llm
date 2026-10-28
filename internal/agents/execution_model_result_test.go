package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestNativeWorkspaceTextProjectionPreservesLegacyAndAuthority(t *testing.T) {
	ref := execution.Reference{Host: "fixture-host", Session: "fixture-session"}
	const text = "Exact\ttext — café 日本語\r\n"
	raw, _ := json.Marshal(execution.ProcessResult{Handle: "owned", State: "running", Output: []byte(text)})
	original, err := execution.PresentReceipt(ref, "process_inspect", execution.Receipt{State: "complete", Result: raw})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, _ := execution.Descriptor("workspace_process_output")
	task := &Task{Config: AgentConfig{TaskFirst: true, Execution: &ref}}
	if executionModelResult(task, "workspace_process_output", descriptor, original, 1) != execution.ModelReceipt(ref, "process_inspect", original, 1) {
		t.Fatal("legacy model view changed")
	}
	task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter}
	projected := executionModelResult(task, "workspace_process_output", descriptor, original, 1)
	if projected == original || !strings.HasSuffix(projected, text) {
		t.Fatal("native text lost")
	}
	descriptor.Source = "untrusted-provider"
	if executionModelResult(task, "workspace_process_output", descriptor, original, 1) != original {
		t.Fatal("foreign capability projected")
	}
}

func TestVerifiedInputProjectionNeverResolvesForeignOrQueuedInputs(t *testing.T) {
	ref := execution.Reference{Host: "host", Session: "session"}
	const input = "<c>exact café 日本語\r\n</c>"
	d, _ := execution.Descriptor(execution.InputWriteTool)
	for _, id := range []string{"request", "correction:applied-123", "correction:pending-123", "foreign-root"} {
		a := execution.InputWriteArguments{Path: "source.txt", Input: id, After: "<c>", Before: "</c>"}
		payload, proof, err := execution.SelectInput(input, a)
		if err != nil {
			t.Fatal(err)
		}
		args, _ := json.Marshal(payload)
		raw, _ := json.Marshal(execution.WriteResult{Location: execution.Location{Host: ref.Host, Path: "/fixture/source.txt"}, Revision: proof.ContentDigest, Bytes: len(payload.Data), Verified: true})
		original, err := execution.PresentInputWrite(ref, execution.Request{Operation: "file_write", Arguments: args, Binding: execution.Binding{Host: ref.Host}}, execution.Receipt{State: "complete", Result: raw}, proof)
		if err != nil {
			t.Fatal(err)
		}
		task := &Task{Prompt: input, Config: AgentConfig{TaskFirst: true, InputWriteVersion: 1, Execution: &ref}, Instructions: []Instruction{{RequestID: "applied-123", Text: input}}, PendingInstructions: []Instruction{{RequestID: "pending-123", Text: input}}}
		if executionModelResult(task, execution.InputWriteTool, d, original, 1) != original {
			t.Fatal("legacy projection changed")
		}
		task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter}
		shown := executionModelResult(task, execution.InputWriteTool, d, original, 1)
		if strings.Contains(shown, `"verified_text"`) != (id == "request" || id == "correction:applied-123") {
			t.Fatal("incorrect source projection", id)
		}
		task.ParentID = "parent"
		if executionModelResult(task, execution.InputWriteTool, d, original, 1) != original {
			t.Fatal("child inherited source")
		}
	}
}
