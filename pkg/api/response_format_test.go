package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponseSchemaRoundTripPreservesDecisionPropertyOrder(t *testing.T) {
	raw := `{"type":"json_schema","json_schema":{"name":"decision","strict":true,"schema":{"type":"object","properties":{"tool_call":{"type":"object","properties":{"name":{"const":"read"},"arguments":{"type":"object","properties":{"path":{"type":"string"}}}},"required":["name","arguments"]}}}}}`
	var format ResponseFormat
	if err := json.Unmarshal([]byte(raw), &format); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(format)
	if err != nil {
		t.Fatal(err)
	}
	name, args := strings.Index(string(encoded), `"name":{"const"`), strings.Index(string(encoded), `"arguments":{"`)
	if name < 0 || args < name {
		t.Fatalf("forwarding reordered the discriminator behind its arguments: %s", encoded)
	}
	format.JSONSchema.Schema["description"] = "changed after decoding"
	encoded, err = json.Marshal(format)
	if err != nil || !strings.Contains(string(encoded), "changed after decoding") {
		t.Fatal("stale wire schema hid a caller mutation", string(encoded), err)
	}
}

func TestResponseSchemaPreservesExactNumbersAndRejectsAmbiguousWireValues(t *testing.T) {
	const raw = `{"type":"json_schema","json_schema":{"name":"integer","schema":{"type":"integer","const":9007199254740993}}}`
	var format ResponseFormat
	if err := json.Unmarshal([]byte(raw), &format); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(format)
	if err != nil || !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatal("integer rounded in transit", string(encoded), err)
	}
	for _, schema := range []string{
		`{"name":"x","schema":{"type":"string","type":"integer"}}`,
		`{"name":"x","schema":{"properties":{"a":{"const":"\ud800"}}}}`,
		`{"name":"x","schema":{"type":"string"},"unknown":true}`,
	} {
		if json.Unmarshal([]byte(`{"type":"json_schema","json_schema":`+schema+`}`), &format) == nil {
			t.Fatal("ambiguous or unknown schema accepted", schema)
		}
	}
}

func TestResponseSchemaNestedMutationDoesNotUseStaleWireValue(t *testing.T) {
	var format ResponseFormat
	if err := json.Unmarshal([]byte(`{"type":"json_schema","json_schema":{"name":"x","schema":{"properties":{"value":{"const":"original"}}}}}`), &format); err != nil {
		t.Fatal(err)
	}
	format.JSONSchema.Schema["properties"].(map[string]any)["value"].(map[string]any)["const"] = "updated"
	encoded, err := json.Marshal(format)
	if err != nil || strings.Contains(string(encoded), "original") || !strings.Contains(string(encoded), "updated") {
		t.Fatal(string(encoded), err)
	}
}

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
