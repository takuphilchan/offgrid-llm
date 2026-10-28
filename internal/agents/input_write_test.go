package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestInputWritingIsRootScopedAndOnlyUsesAppliedUserInputs(t *testing.T) {
	task := &Task{Prompt: "<content>original café 日本語</content>", Config: AgentConfig{InputWriteVersion: 1}, Instructions: []Instruction{{RequestID: "applied-123", Text: "<content>new\r\ncontent</content>"}}, PendingInstructions: []Instruction{{RequestID: "pending-123", Text: "<content>not applied</content>"}}}
	before, _ := json.Marshal(task)
	for input, wanted := range map[string]string{"request": "original café 日本語", "correction:applied-123": "new\r\ncontent"} {
		args, _ := json.Marshal(execution.InputWriteArguments{Path: "result.txt", Input: input, After: "<content>", Before: "</content>"})
		payload, proof, err := ResolveInputWrite(task, args)
		if err != nil || string(payload.Data) != wanted || proof.Input != input {
			t.Fatal(payload, proof, err)
		}
		encoded, _ := json.Marshal(payload)
		if !inputWriteIntentMatches(task, args, execution.Request{Arguments: encoded}) {
			t.Fatal("matching input intent rejected")
		}
		payload.Data = []byte("model substitution")
		encoded, _ = json.Marshal(payload)
		if inputWriteIntentMatches(task, args, execution.Request{Arguments: encoded}) {
			t.Fatal("model substitution accepted")
		}
	}
	for _, input := range []string{"correction:pending-123", "foreign-root", "correction:foreign-123", "assistant"} {
		args, _ := json.Marshal(execution.InputWriteArguments{Path: "x", Input: input, After: "<content>", Before: "</content>"})
		if _, _, err := ResolveInputWrite(task, args); err == nil {
			t.Fatal("foreign input accepted", input)
		}
	}
	after, _ := json.Marshal(task)
	if string(before) != string(after) {
		t.Fatal("source state mutated")
	}
	task.Config.InputWriteVersion = 0
	if len(InputWriteSources(task)) != 0 {
		t.Fatal("old job gained a source tool")
	}
	task.Config.InputWriteVersion = 2
	if len(InputWriteSources(task)) != 0 {
		t.Fatal("unknown feature version gained a source tool")
	}
	task.Config.InputWriteVersion = 1
	task.ParentID = "parent"
	if len(InputWriteSources(task)) != 0 {
		t.Fatal("child inherited root input access")
	}
}

func TestAmbiguousCorrectionIdentityNeverSelectsOneSilently(t *testing.T) {
	task := &Task{Prompt: "root", Config: AgentConfig{InputWriteVersion: 1}, Instructions: []Instruction{{RequestID: "duplicate-id", Text: "a"}, {RequestID: "duplicate-id", Text: "b"}}}
	if strings.Join(InputWriteSources(task), ",") != "request" {
		t.Fatal(InputWriteSources(task))
	}
	// A duplicate beyond the bounded menu cannot make the first occurrence
	// appear unique. The menu is bounded, identity validation covers all records.
	for len(task.Instructions) < 33 {
		task.Instructions = append(task.Instructions, Instruction{RequestID: ""})
	}
	task.Instructions[1].RequestID = ""
	task.Instructions[32] = Instruction{RequestID: "duplicate-id", Text: "late duplicate"}
	if strings.Join(InputWriteSources(task), ",") != "request" {
		t.Fatal("late duplicate offered", InputWriteSources(task))
	}
}
