package computer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDocumentArgumentsClosedAndUnicodeExact(t *testing.T) {
	valid := map[string]string{
		"computer_documents":         `{}`,
		"computer_create_document":   `{"observation_id":"o","command":"c"}`,
		"computer_select_document":   `{"observation_id":"o","document":"d"}`,
		"computer_read_text":         `{"observation_id":"o","document":"d","page":{"snapshot":"s","revision":"r","start":0}}`,
		"computer_select_text":       `{"observation_id":"o","document":"d","snapshot":"s","start":0,"end":1}`,
		"computer_insert_text":       `{"observation_id":"o","document":"d","snapshot":"s","text":"café 日本語 😀"}`,
		"computer_replace_selection": `{"observation_id":"o","document":"d","snapshot":"s","text":""}`,
		"computer_replace_text":      `{"observation_id":"o","document":"d","snapshot":"s","text":""}`,
	}
	for name, raw := range valid {
		if !DocumentTool(name, []byte(raw)) || ValidateDocumentArguments(name, []byte(raw)) != nil {
			t.Fatal(name)
		}
		for _, bad := range []string{"null", "[]", raw + "{}", raw[:len(raw)-1] + `,"unknown":null}`, raw[:len(raw)-1] + `,"observation_id":"forged"}`} {
			if ValidateDocumentArguments(name, []byte(bad)) == nil {
				t.Fatal("accepted ambiguous arguments", name, bad)
			}
		}
	}
	for _, bad := range []string{`null`, `"\ud800"`, `"\u0000"`, `123`, `false`} {
		raw := `{"observation_id":"o","document":"d","snapshot":"s","text":` + bad + `}`
		if ValidateDocumentArguments("computer_insert_text", []byte(raw)) == nil {
			t.Fatal(bad)
		}
	}
	for _, bad := range []string{`{"snapshot":"s","revision":"r","start":null}`, `{"snapshot":"s","revision":"r","start":0.5}`, `null`, `{"snapshot":"s","revision":"r","start":65537}`} {
		if ValidateDocumentArguments("computer_read_text", []byte(`{"observation_id":"o","document":"d","page":`+bad+`}`)) == nil {
			t.Fatal(bad)
		}
	}
	if DocumentTool("computer_replace_text", []byte(`{"observation_id":"o","element":"e","text":"old"}`)) {
		t.Fatal("legacy contract changed")
	}
}

func TestDocumentHubNegotiationPrepareReadBudgetAndErrors(t *testing.T) {
	for _, feature := range []int{0, 1} {
		h := NewBrowserHub()
		code, _ := h.PairCode("alice")
		target := TargetIdentity{ID: "w", OSSession: "o", ProcessGeneration: "p", Surface: "s", Driver: "windows-uia"}
		s, token, err := h.PairNativeFeatures(code, "Owned fixture", target, 2, ApprovalFullTask, NativeFeatures{Documents: feature})
		if err != nil {
			t.Fatal(err)
		}
		cases := []struct{ name, args string }{
			{"computer_documents", `{}`},
			{"computer_prepare", `{"tool":"computer_insert_text","arguments":{"observation_id":"o","document":"d","snapshot":"s","text":"` + strings.Repeat(`\u0061`, 16000) + `"}}`},
		}
		for _, c := range cases {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			before := s.Remaining
			done := make(chan error, 1)
			go func() {
				_, e := h.Execute(ctx, "alice", s.ID, "run", "call", c.name, json.RawMessage(c.args))
				done <- e
			}()
			if feature == 0 {
				if err := <-done; !errors.Is(err, ErrDocumentUnavailable) {
					t.Fatal(err)
				}
				if action, _ := h.Poll(token); action != nil {
					t.Fatal("unnegotiated dispatch")
				}
			} else {
				var action *BrowserAction
				for action == nil && ctx.Err() == nil {
					action, _ = h.Poll(token)
					if action == nil {
						time.Sleep(time.Millisecond)
					}
				}
				if action == nil {
					t.Fatal("missing negotiated dispatch")
				}
				if s.Remaining != before {
					t.Fatal("read/prepare consumed input budget")
				}
				if err := h.Reply(token, BrowserReply{ID: action.ID, Error: "computer_document_incomplete"}); err != nil {
					t.Fatal(err)
				}
				if err := <-done; !errors.Is(err, ErrDocumentIncomplete) {
					t.Fatal(err)
				}
				if h.Check("alice", s.ID, "run") != nil {
					t.Fatal("pre-dispatch failure revoked session")
				}
			}
			cancel()
		}
	}
}
