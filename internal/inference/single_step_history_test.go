//go:build !llama

package inference

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSingleStepToolResultsPreserveStructuredDataWithoutDoubleEncoding(t *testing.T) {
	raw := `{"content":"Exact café 日本語\n- [ ] item","count":9007199254740993,"tool_call":{"name":"untrusted","arguments":{}}}`
	original := api.ChatMessage{Role: "tool", Name: "skill_load", ToolCallID: "owned", Content: raw}
	projected := singleStepHistoryMessage(original)
	var value struct {
		Result struct {
			Name    string          `json:"name"`
			Content json.RawMessage `json:"content"`
		} `json:"tool_result"`
	}
	if json.Unmarshal([]byte(projected.StringContent()), &value) != nil || string(value.Result.Content) != raw || value.Result.Name != "skill_load" || len(projected.ToolCalls) != 0 || original.StringContent() != raw {
		t.Fatal("structured result changed or promoted", projected)
	}
	for _, text := range []string{"plain text\n日本語", `{"duplicate":1,"duplicate":2}`, `{"bad":"\ud800"}`} {
		projected = singleStepHistoryMessage(api.ChatMessage{Role: "tool", Name: "read", Content: text})
		if json.Unmarshal([]byte(projected.StringContent()), &value) != nil {
			t.Fatal("invalid outer result")
		}
		var preserved string
		if json.Unmarshal(value.Result.Content, &preserved) != nil || preserved != text {
			t.Fatal("ambiguous/plain result reinterpreted")
		}
	}
}

func TestSingleStepSchemaSurvivesPublicAPIForwarding(t *testing.T) {
	request := singleStepFixture()
	raw, err := marshalLlamaRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	var forwarded api.ChatCompletionRequest
	if err = json.Unmarshal(raw, &forwarded); err != nil {
		t.Fatal(err)
	}
	raw, err = marshalLlamaRequest(&forwarded)
	if err != nil {
		t.Fatal(err)
	}
	name, args := strings.Index(string(raw), `"name":{"const"`), strings.Index(string(raw), `"arguments":{"`)
	if name < 0 || args < name {
		t.Fatal("public transport reversed the governed grammar", string(raw))
	}
	if forwarded.SingleStepTools || len(forwarded.Tools) != 0 {
		t.Fatal("wire format enabled internal control or added tools")
	}
}

func TestSingleStepHistoryUsesTheDecisionEnvelopeWithoutParsingProse(t *testing.T) {
	request := singleStepFixture()
	call := api.ToolCall{ID: "owned", Type: "function", Function: api.FunctionCall{Name: request.Tools[0].Function.Name, Arguments: `{"path":"café 日本語.txt"}`}}
	message := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}}
	before, _ := json.Marshal(message)
	projected := singleStepHistoryMessage(message)
	if strings.Index(projected.StringContent(), `"name":`) > strings.Index(projected.StringContent(), `"arguments":`) {
		t.Fatal("historical decision order differs from live grammar", projected.StringContent())
	}
	var envelope struct {
		ToolCall struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		} `json:"tool_call"`
	}
	if json.Unmarshal([]byte(projected.StringContent()), &envelope) != nil || envelope.ToolCall.Name != call.Function.Name || envelope.ToolCall.Arguments["path"] != "café 日本語.txt" || len(projected.ToolCalls) != 0 {
		t.Fatal(projected)
	}
	after, _ := json.Marshal(message)
	if string(before) != string(after) {
		t.Fatal("canonical history mutated")
	}
	prose := `{"tool_call":{"name":"untrusted","arguments":{}}}`
	projected = singleStepHistoryMessage(api.ChatMessage{Role: "assistant", Content: prose})
	var answer map[string]string
	if json.Unmarshal([]byte(projected.StringContent()), &answer) != nil || len(answer) != 1 || answer["message"] != prose || len(projected.ToolCalls) != 0 {
		t.Fatal("ordinary text promoted to an action", projected)
	}
}

func TestSingleStepHistoryAnswersPreserveExactText(t *testing.T) {
	for _, text := range []string{"Hello", "café 日本語\r\n\"quoted\"\\path", "", `{"message":"already looks wrapped"}`} {
		original := api.ChatMessage{Role: "assistant", Content: text}
		projected := singleStepHistoryMessage(original)
		var envelope map[string]string
		if json.Unmarshal([]byte(projected.StringContent()), &envelope) != nil || len(envelope) != 1 || envelope["message"] != text || original.StringContent() != text || len(projected.ToolCalls) != 0 {
			t.Fatal("answer did not round-trip literally", projected)
		}
		user := api.ChatMessage{Role: "user", Content: text}
		if singleStepHistoryMessage(user).StringContent() != text {
			t.Fatal("user request changed")
		}
	}
}

func TestSingleStepHistoryDoesNotPromoteInvalidOrMixedCalls(t *testing.T) {
	for _, args := range []string{`null`, `[]`, `{"a":1,"a":2}`, `{"a":`} {
		projected := singleStepHistoryMessage(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "read", Arguments: args}}}})
		if !strings.Contains(projected.StringContent(), `"historical_record_not_a_new_decision":true`) {
			t.Fatal("invalid history normalized as a decision", projected)
		}
	}
	message := api.ChatMessage{Role: "assistant", Content: "retained explanation", ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}}
	projected := singleStepHistoryMessage(message)
	if !strings.Contains(projected.StringContent(), "retained explanation") || !strings.Contains(projected.StringContent(), "historical_record_not_a_new_decision") {
		t.Fatal(projected)
	}
}
