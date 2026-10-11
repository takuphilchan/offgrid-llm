package agents

import (
	"encoding/json"
	"testing"
)

func TestCanonicalArgumentsRejectLossyOrAmbiguousInput(t *testing.T) {
	for _, raw := range []string{
		`{"text":"\ud800"}`, `{"text":"\udc00"}`,
		"{\"text\":\"\xff\"}",
		`{"text":"first","text":"second"}`,
		`{"text":"first","\u0074ext":"second"}`,
		`{"nested":{"text":"first","text":"second"}}`,
	} {
		if _, err := CanonicalArguments(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid exact-text arguments accepted: %q", raw)
		}
	}
}

func TestCanonicalArgumentsPreserveValidUnicode(t *testing.T) {
	for _, text := range []string{"OffGrid — café 日本語 😀", "e\u0301\r\nمرحبا", "\ufffd", `\uD800`, "", " \t\n "} {
		raw, _ := json.Marshal(map[string]string{"text": text})
		canonical, err := CanonicalArguments(raw)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]string
		if err = json.Unmarshal(canonical, &got); err != nil || got["text"] != text {
			t.Fatal("text changed during canonicalization", err)
		}
	}
	canonical, err := CanonicalArguments(json.RawMessage(`{"text":"\ud83d\ude00"}`))
	var got map[string]string
	if err != nil || json.Unmarshal(canonical, &got) != nil || got["text"] != "😀" {
		t.Fatal("valid escaped surrogate pair rejected", err)
	}
}
