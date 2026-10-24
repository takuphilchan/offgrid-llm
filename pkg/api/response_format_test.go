package api

import (
	"encoding/json"
	"testing"
)

func TestChatResponseFormatValidation(t *testing.T) {
	for _, raw := range []string{
		`{"type":"text"}`, `{"type":"json_object"}`,
		`{"type":"json_schema","json_schema":{"name":"title","strict":true,"schema":{"type":"object","properties":{"title":{"type":"string"}},"required":["title"],"additionalProperties":false}}}`,
	} {
		var req ChatCompletionRequest
		if err := json.Unmarshal([]byte(`{"response_format":`+raw+`}`), &req); err != nil {
			t.Fatal(err)
		}
		if req.ResponseFormat == nil {
			t.Fatal("format dropped")
		}
		var legacy CompletionRequest
		if err := json.Unmarshal([]byte(`{"response_format":`+raw+`}`), &legacy); err == nil {
			t.Fatal("legacy completion accepted unsupported format")
		}
	}
	for _, raw := range []string{
		`{"type":"json_schema"}`, `{"type":"json_schema","json_schema":{"name":"x","schema":{"$ref":"https://invalid.example/private"}}}`,
		`{"type":"json_object","extra":true}`, `{"type":"json_object","json_schema":{"name":"x","schema":{}}}`,
	} {
		var req ChatCompletionRequest
		if json.Unmarshal([]byte(`{"response_format":`+raw+`}`), &req) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
