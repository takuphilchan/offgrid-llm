package jsonwire

import "testing"

func TestValidateExactJSON(t *testing.T) {
	for _, raw := range []string{`{"text":"OffGrid — café 日本語 😀 e\u0301\r\n�"}`, `{"text":"\ud83d\ude00"}`, `{"text":"\\uD800"}`, `{"text":"\ufffd"}`, `{"a":{"a":1},"b":[{"a":2}]}`, `{"n":9007199254740993}`, `[]`, `{}`, `null`} {
		if err := Validate([]byte(raw)); err != nil {
			t.Fatalf("valid JSON rejected: %q: %v", raw, err)
		}
	}
	for _, raw := range []string{`{"text":"\ud800"}`, `{"text":"\udc00"}`, `{"text":"\ud800x"}`, `{"text":"\ud800\ud800"}`, "{\"text\":\"\xff\"}", `{"text":"one","text":"two"}`, `{"text":"one","\u0074ext":"two"}`, `{"x":[{"a":1,"a":2}]}`, `{} {}`} {
		if Validate([]byte(raw)) == nil {
			t.Fatalf("ambiguous JSON accepted: %q", raw)
		}
	}
}

func FuzzValidateDoesNotPanic(f *testing.F) {
	for _, raw := range []string{`{"text":"\ud83d\ude00"}`, `{"text":"\\uD800"}`, `{"x":[{"a":1}]}`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, raw []byte) { _ = Validate(raw) })
}
