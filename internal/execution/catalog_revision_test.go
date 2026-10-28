package execution_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkspacePageRevisionContractRejectsModelPlaceholdersBeforeDispatch(t *testing.T) {
	for _, name := range []string{"workspace_list", "workspace_read", "workspace_write"} {
		var tool api.Tool
		for _, candidate := range execution.Tools() {
			if candidate.Function.Name == name {
				tool = candidate
			}
		}
		for _, revision := range []string{"", ".", "current", "abc", strings.Repeat("a", 64), strings.Repeat("A", 64)} {
			args := `{"path":".","revision":"` + revision + `"}`
			if name == "workspace_write" {
				args = `{"path":"new.txt","content":"café 日本語","revision":"` + revision + `"}`
			} else if revision != "" {
				args = `{"path":"reports","offset":1,"revision":"` + revision + `"}`
			}
			response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}}, FinishReason: "tool_calls"}}}
			err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{tool}}, response)
			valid := revision == "" || revision == strings.Repeat("a", 64)
			if valid && err != nil {
				t.Fatal(name, revision, err)
			}
			var repair *inference.DecisionValidationError
			if !valid && !errors.As(err, &repair) {
				t.Fatal("invalid revision was not a pre-dispatch model correction", name, revision, err)
			}
		}
	}
}

func TestWorkspacePageContractSeparatesFreshTargetsFromContinuations(t *testing.T) {
	for _, tool := range execution.Tools() {
		if tool.Function.Name != "workspace_list" && tool.Function.Name != "workspace_read" {
			continue
		}
		for _, tc := range []struct {
			offset   int
			revision string
			valid    bool
		}{
			{-1, "", true}, {0, "", true}, {-1, strings.Repeat("a", 64), false}, {0, strings.Repeat("a", 64), false},
			{1, "", false}, {1, strings.Repeat("a", 64), true},
		} {
			fields := map[string]any{"path": "reports", "revision": tc.revision}
			if tc.offset >= 0 {
				fields["offset"] = tc.offset
			}
			args, _ := json.Marshal(fields)
			response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: tool.Function.Name, Arguments: string(args)}}}}, FinishReason: "tool_calls"}}}
			err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{tool}}, response)
			var repair *inference.DecisionValidationError
			if tc.valid && err != nil || !tc.valid && !errors.As(err, &repair) {
				t.Fatalf("%s %s: %v", tool.Function.Name, args, err)
			}
		}
	}
}

func TestWorkspacePageGrammarBranchesCarryCompleteObjectContracts(t *testing.T) {
	for _, tool := range execution.Tools() {
		if tool.Function.Name != "workspace_list" && tool.Function.Name != "workspace_read" {
			continue
		}
		if _, sibling := tool.Function.Parameters["properties"]; sibling {
			t.Fatal("grammar union must not rely on sibling properties")
		}
		for _, raw := range tool.Function.Parameters["oneOf"].([]any) {
			branch := raw.(map[string]any)
			properties, ok := branch["properties"].(map[string]any)
			if !ok || branch["type"] != "object" || branch["additionalProperties"] != false {
				t.Fatal("incomplete grammar object", branch)
			}
			for _, name := range []string{"path", "offset", "limit", "revision"} {
				if properties[name] == nil {
					t.Fatal("field lost in grammar branch", name)
				}
			}
			if tool.Function.Name == "workspace_read" {
				found := false
				for _, name := range branch["required"].([]string) {
					found = found || name == "path"
				}
				if !found {
					t.Fatal("grammar could emit a read with no path")
				}
			}
		}
	}
}

func TestWorkspacePathContractRejectsForeignPathsBeforeDispatch(t *testing.T) {
	for _, tool := range execution.Tools() {
		name := tool.Function.Name
		if name != "workspace_list" && name != "workspace_read" && name != "workspace_write" && name != "workspace_search" && name != "workspace_change_directory" {
			continue
		}
		for _, path := range []string{".", "reports", "reports/café 日本語.md", "/tmp/reports", `C:\reports`, `\\server\share`, `reports\file.txt`, "https://example.com", "bad\x00file"} {
			fields := map[string]any{"path": path}
			if name == "workspace_write" {
				fields["content"] = "exact"
			}
			if name == "workspace_search" {
				fields["query"] = "text"
			}
			args, _ := json.Marshal(fields)
			response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: string(args)}}}}, FinishReason: "tool_calls"}}}
			err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{tool}}, response)
			valid := path == "." || path == "reports" || path == "reports/café 日本語.md"
			var repair *inference.DecisionValidationError
			if valid && err != nil || !valid && !errors.As(err, &repair) {
				t.Fatalf("%s path %q: %v", name, path, err)
			}
		}
	}
}
