package inference

import (
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func decisionFixture() (*api.ChatCompletionRequest, *api.ChatCompletionResponse) {
	parallel := false
	req := &api.ChatCompletionRequest{ToolChoice: "auto", ParallelToolCalls: &parallel, Tools: []api.Tool{{Type: "function", Function: api.FunctionDef{Name: "write_draft", Parameters: map[string]any{
		"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}, "version": map[string]any{"type": "integer"}},
		"required": []string{"text", "version"}, "additionalProperties": false,
	}}}}}
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "one", Type: "function", Function: api.FunctionCall{Name: "write_draft", Arguments: `{"text":"café 日本語","version":9007199254740993}`}}}}}}}
	return req, response
}

func TestToolDecisionCompleteNativeContract(t *testing.T) {
	req, response := decisionFixture()
	original := response.Choices[0].Message.ToolCalls[0].Function.Arguments
	if err := ValidateToolDecision(req, response); err != nil {
		t.Fatal(err)
	}
	if response.Choices[0].Message.ToolCalls[0].Function.Arguments != original {
		t.Fatal("exact text or number spelling changed")
	}
	response.Choices[0] = api.ChatCompletionChoice{FinishReason: "stop", Message: api.ChatMessage{Content: `Here is an example: {"tool_call":{"name":"write_draft"}}`}}
	if err := ValidateToolDecision(req, response); err != nil || len(response.Choices[0].Message.ToolCalls) != 0 {
		t.Fatal("ordinary conversation became an action", err)
	}
}

func TestToolDecisionRejectsInvalidNativeCalls(t *testing.T) {
	cases := map[string]func(*api.ChatCompletionRequest, *api.ChatCompletionResponse){
		"truncated": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices[0].FinishReason = "length"
		},
		"stop with call": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) { r.Choices[0].FinishReason = "stop" },
		"missing finish": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) { r.Choices[0].FinishReason = "" },
		"unknown tool": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices[0].Message.ToolCalls[0].Function.Name = "shell"
		},
		"unknown type": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices[0].Message.ToolCalls[0].Type = "shell"
		},
		"missing id": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices[0].Message.ToolCalls[0].ID = ""
		},
		"duplicate catalog": func(q *api.ChatCompletionRequest, _ *api.ChatCompletionResponse) {
			q.Tools = append(q.Tools, q.Tools[0])
		},
		"two decisions": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices = append(r.Choices, r.Choices[0])
		},
		"batch": func(_ *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			r.Choices[0].Message.ToolCalls = append(r.Choices[0].Message.ToolCalls, r.Choices[0].Message.ToolCalls[0])
		},
		"duplicate ids": func(q *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			q.ParallelToolCalls = nil
			r.Choices[0].Message.ToolCalls = append(r.Choices[0].Message.ToolCalls, r.Choices[0].Message.ToolCalls[0])
		},
		"none": func(q *api.ChatCompletionRequest, _ *api.ChatCompletionResponse) { q.ToolChoice = "none" },
		"wrong chosen tool": func(q *api.ChatCompletionRequest, _ *api.ChatCompletionResponse) {
			q.ToolChoice = map[string]any{"type": "function", "function": map[string]any{"name": "other"}}
		},
		"no required call": func(q *api.ChatCompletionRequest, r *api.ChatCompletionResponse) {
			q.ToolChoice = "required"
			r.Choices[0] = api.ChatCompletionChoice{FinishReason: "stop", Message: api.ChatMessage{Content: "Done"}}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			req, response := decisionFixture()
			change(req, response)
			if err := ValidateToolDecision(req, response); err == nil {
				t.Fatal("invalid decision accepted")
			}
		})
	}
	for _, args := range []string{`{"text":"first","text":"private-value","version":1}`, `{"text":"\ud800","version":1}`, `{"text":"private-value","version":"1"}`, `{"text":"private-value","version":1,"extra":true}`, `{"version":1}`, `[]`, `null`, `{`, `{} {}`, `{"text":"private-value","version":1.5}`} {
		t.Run(args, func(t *testing.T) {
			req, response := decisionFixture()
			response.Choices[0].Message.ToolCalls[0].Function.Arguments = args
			err := ValidateToolDecision(req, response)
			if err == nil || strings.Contains(err.Error(), "private-value") {
				t.Fatal("invalid arguments accepted or leaked", err)
			}
		})
	}
}

func TestToolDecisionSchemaReferencesStayLocal(t *testing.T) {
	req, response := decisionFixture()
	req.Tools[0].Function.Parameters = map[string]any{"$defs": map[string]any{"input": map[string]any{"type": "object", "required": []string{"text"}}}, "$ref": "#/$defs/input"}
	if err := ValidateToolDecision(req, response); err != nil {
		t.Fatal("local reference rejected", err)
	}
	for _, ref := range []string{"https://example.invalid/private-schema", "file:///private/schema"} {
		req.Tools[0].Function.Parameters = map[string]any{"$ref": ref}
		err := ValidateToolDecision(req, response)
		if err == nil || strings.Contains(err.Error(), ref) {
			t.Fatal("external schema accepted or leaked", err)
		}
		if _, operational := err.(*DecisionContractError); !operational {
			t.Fatal("provider schema failure was blamed on model", err)
		}
	}
}

func TestToolDecisionDoesNotRoundInvalidFractionToInteger(t *testing.T) {
	for _, value := range []string{"0.999999999999999999999", "1e999999", "1e-999999", "9007199254740993.0"} {
		req, response := decisionFixture()
		response.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"text":"fixture","version":` + value + `}`
		if err := ValidateToolDecision(req, response); err == nil {
			t.Fatal("rounded invalid integer", value)
		}
	}
	req, response := decisionFixture()
	req.Tools[0].Function.Parameters["properties"].(map[string]any)["version"] = map[string]any{"type": "number"}
	response.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"text":"fixture","version":0.1}`
	if err := ValidateToolDecision(req, response); err != nil {
		t.Fatal("ordinary decimal rejected", err)
	}
}
