package server

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestToolApprovalKeyBindsExactCanonicalArguments(t *testing.T) {
	first, err := toolApprovalKey("write_file", json.RawMessage(`{"path":"notes.txt","content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := toolApprovalKey("write_file", json.RawMessage(`{"content":"hello","path":"notes.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := toolApprovalKey("write_file", json.RawMessage(`{"path":"other.txt","content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if first != reordered {
		t.Fatal("semantically identical arguments produced different approval keys")
	}
	if first == changed {
		t.Fatal("changed arguments reused an approval key")
	}
}

func TestResponsesInputAndFunctionCallContract(t *testing.T) {
	input := json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]`)
	messages, err := responseInputMessages(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].StringContent() != "hello" {
		t.Fatalf("unexpected converted input: %#v", messages)
	}

	response := responseFromChat(api.ChatCompletionResponse{
		ID: "resp-1", Object: "chat.completion", Model: "model-a",
		Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{
			Role: "assistant", ToolCalls: []api.ToolCall{{ID: "call-1", Type: "function", Function: api.FunctionCall{Name: "lookup", Arguments: `{"key":"answer"}`}}},
		}}},
	})
	output, ok := response["output"].([]map[string]interface{})
	if !ok || len(output) != 1 {
		t.Fatalf("unexpected responses output: %#v", response["output"])
	}
	if output[0]["type"] != "function_call" || output[0]["call_id"] != "call-1" || output[0]["arguments"] != `{"key":"answer"}` {
		t.Fatalf("function call contract mismatch: %#v", output[0])
	}
}
