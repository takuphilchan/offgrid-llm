package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestExactUserMarkersNarrowOnlyTheOfferedSelection(t *testing.T) {
	ref := execution.Reference{Host: "host", Session: "session"}
	task := &Task{Prompt: "Replace source.txt exactly: <body>café 日本語\n</body>", Config: AgentConfig{Execution: &ref, InputWriteVersion: execution.InputWriteVersion}}
	tools := execution.ModelTools()
	before, _ := json.Marshal(tools)
	projected := projectInputWriteMarkers(task, tools)
	request := &api.ChatCompletionRequest{Tools: projected}
	for _, tc := range []struct {
		input, after, before string
		valid                bool
	}{
		{"request", "<body>", "</body>", true},
		{"request", "<", ">", false},
		{"request", "<body>", ">", false},
		{"foreign", "<body>", "</body>", false},
	} {
		args, _ := json.Marshal(map[string]string{"path": "source.txt", "revision": strings.Repeat("a", 64), "input": tc.input, "after": tc.after, "before": tc.before})
		message := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", ID: "call", Function: api.FunctionCall{Name: execution.InputWriteTool, Arguments: string(args)}}}}
		response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: message}}}
		if err := inference.ValidateToolDecision(request, response); (err == nil) != tc.valid {
			t.Fatal("marker schema admission", tc, err)
		}
	}
	payload, _, err := ResolveInputWrite(task, json.RawMessage(`{"path":"source.txt","revision":"","input":"request","after":"<body>","before":"</body>"}`))
	if err != nil || string(payload.Data) != "café 日本語\n" {
		t.Fatal("source bytes changed", payload, err)
	}
	after, _ := json.Marshal(tools)
	if string(before) != string(after) {
		t.Fatal("shared catalog mutated")
	}
}

func TestMarkerHintRequiresOneCompleteUniquePair(t *testing.T) {
	for _, input := range []string{"ordinary writing", "<a>one</a> <b>two</b>", "<a>one<a>two</a>", "<a>one", "</a>one<a>"} {
		if _, _, ok := uniqueInputMarkers(input); ok {
			t.Fatal("ambiguous or incomplete markers offered", input)
		}
	}
}
