package computer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var structuredBrowserCalls = map[string]string{
	"browser_search":       `{"role":"button","name":"café 語😀","text":""}`,
	"browser_frames":       `{}`,
	"browser_frame_select": `{"frame_id":"frame-one","document_id":"document-one"}`,
}

func TestBrowserStructuredFeatureNegotiation(t *testing.T) {
	for _, raw := range []string{`{"structured":1}`, `{"discovery":1,"structured":1}`, `{"discovery":1,"workspace":1,"structured":0}`, `{"discovery":1,"workspace":1,"structured":2}`, `{"discovery":1,"workspace":1,"structured":null}`, `{"discovery":1,"workspace":1,"structured":"1"}`} {
		var f BrowserFeatures
		if json.Unmarshal([]byte(raw), &f) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, f := range []BrowserFeatures{{Structured: 1}, {Discovery: 1, Structured: 1}, {Discovery: 1, Workspace: 1, Structured: 2}} {
		if f.Validate() == nil {
			t.Fatal("accepted invalid direct features", f)
		}
	}
	for _, f := range []BrowserFeatures{{}, {Discovery: 1}, {Discovery: 1, Workspace: 1}} {
		data, _ := json.Marshal(f)
		if strings.Contains(string(data), "structured") {
			t.Fatal("legacy wire shape expanded", string(data))
		}
	}
	var f BrowserFeatures
	if err := json.Unmarshal([]byte(`{"discovery":1,"workspace":1,"structured":1}`), &f); err != nil || f.Structured != 1 {
		t.Fatal(err, f)
	}
}

func TestBrowserStructuredArgumentsAreLiteralAndBounded(t *testing.T) {
	for tool, args := range structuredBrowserCalls {
		if err := ValidateBrowserStructuredArguments(tool, json.RawMessage(args)); err != nil {
			t.Fatal(tool, err)
		}
	}
	for _, role := range BrowserSearchRoles() {
		args, _ := json.Marshal(map[string]string{"role": role, "name": "[literal].*", "text": ""})
		if err := ValidateBrowserStructuredArguments("browser_search", args); err != nil {
			t.Fatal(role, err)
		}
	}
	for _, args := range []string{
		`null`, `[]`, `{}`, `{"role":"button","name":"ok"}`, `{"role":"button","name":"ok","text":"","selector":"button"}`,
		`{"role":"button","name":null,"text":""}`, `{"role":null,"name":"ok","text":""}`, `{"role":"","name":" ","text":"\n"}`,
		`{"role":"button[onclick]","name":"ok","text":""}`, `{"role":"BUTTON","name":"ok","text":""}`,
		`{"role":"","name":"a","name":"b","text":""}`, `{"role":"","name":"\ud800","text":""}`,
		`{"role":"","name":"` + strings.Repeat("語", 86) + `","text":""}`, `{"role":"","name":"","text":"` + strings.Repeat("😀", 65) + `"}`,
	} {
		if ValidateBrowserStructuredArguments("browser_search", json.RawMessage(args)) == nil {
			t.Fatalf("accepted %s", args)
		}
	}
	for _, args := range []string{`null`, `{"frame_id":"frame"}`, `{"frame_id":"frame","document_id":null}`, `{"frame_id":"frame","document_id":" "}`, `{"frame_id":"frame","document_id":"document","tab_id":"other"}`, `{"frame_id":"` + strings.Repeat("語", 43) + `","document_id":"doc"}`} {
		if ValidateBrowserStructuredArguments("browser_frame_select", json.RawMessage(args)) == nil {
			t.Fatal("accepted frame args", args)
		}
	}
	if ValidateBrowserStructuredArguments("browser_frames", json.RawMessage(`{"frame_id":"other"}`)) == nil {
		t.Fatal("inventory accepted a selector")
	}
}

func TestBrowserStructuredHubBindsFeatureActorAndPreparedScope(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for tool, args := range structuredBrowserCalls {
			for _, prepared := range []bool{false, true} {
				t.Run(tool+map[bool]string{false: "-legacy", true: "-structured"}[enabled]+map[bool]string{false: "-direct", true: "-prepared"}[prepared], func(t *testing.T) {
					h := NewBrowserHub()
					code, _ := h.PairCode("alice")
					f := BrowserFeatures{Discovery: 1, Workspace: 1}
					if enabled {
						f.Structured = 1
					}
					s, token, err := h.PairBrowserFeatures(code, "https://example.com", ProtocolVersion, ApprovalAskEveryTime, f)
					if err != nil {
						t.Fatal(err)
					}
					kind, raw := tool, json.RawMessage(args)
					if prepared {
						kind = "computer_prepare"
						raw, _ = json.Marshal(map[string]any{"tool": tool, "arguments": raw})
					}
					if _, err = h.Execute(context.Background(), "bob", s.ID, "run", "foreign", kind, raw); err == nil {
						t.Fatal("foreign dispatch")
					}
					if !enabled {
						if _, err = h.Execute(context.Background(), "alice", s.ID, "run", "call", kind, raw); !errors.Is(err, ErrControlScope) {
							t.Fatal("legacy dispatch", err)
						}
						if a, _ := h.Poll(token); a != nil {
							t.Fatal("legacy queue contains new tool")
						}
						return
					}
					if got, err := h.BrowserFeatures("alice", s.ID); err != nil || got != f {
						t.Fatal("lost features", got, err)
					}
					if _, err := h.BrowserFeatures("bob", s.ID); err == nil {
						t.Fatal("foreign feature lookup")
					}
					receipts := 0
					h.RecordReply = func(actor, session, run string, a BrowserAction, r BrowserReply) error {
						if actor != "alice" || session != s.ID || run != "run" || a.ID != "run:call" || a.Kind != kind || string(a.Arguments) != string(raw) || r.ID != a.ID {
							t.Error("receipt identity changed", a)
						}
						receipts++
						return nil
					}
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					done := make(chan error, 1)
					go func() { _, e := h.Execute(ctx, "alice", s.ID, "run", "call", kind, raw); done <- e }()
					var action *BrowserAction
					for action == nil && ctx.Err() == nil {
						action, _ = h.Poll(token)
						if action == nil {
							time.Sleep(time.Millisecond)
						}
					}
					if action == nil {
						t.Fatal("no structured action")
					}
					reply := BrowserReply{ID: action.ID, Result: `{"prepared":true,"approval_class":"read_only"}`}
					if !prepared {
						reply.Error = "computer_browser_observation_failed"
						reply.Result = ""
					}
					if err = h.Reply(token, reply); err != nil {
						t.Fatal(err)
					}
					err = <-done
					if prepared && err != nil || !prepared && !errors.Is(err, ErrBrowserObservation) {
						t.Fatal("read classification", err)
					}
					if receipts != 1 || h.Heartbeat(token) != nil {
						t.Fatal("read receipt/session lost", receipts)
					}
				})
			}
		}
	}
}

func TestBrowserStructuredMalformedAndNativePrepareNeverQueue(t *testing.T) {
	h := NewBrowserHub()
	code, _ := h.PairCode("alice")
	s, token, err := h.PairBrowserFeatures(code, "https://example.com", ProtocolVersion, ApprovalFullTask, BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		kind, args string
		want       error
	}{
		{"browser_search", `{"role":"button","name":"ok","text":"","script":"x"}`, ErrBrowserPrecondition},
		{"computer_prepare", `{"tool":"browser_search","arguments":{"role":"button","name":"ok","text":"","selector":"x"}}`, ErrBrowserPrecondition},
		{"computer_prepare", `{"tool":"browser_frames","arguments":{},"extra":true}`, ErrInvalidControl},
	} {
		if _, err = h.Execute(context.Background(), "alice", s.ID, "run", "bad", call.kind, json.RawMessage(call.args)); !errors.Is(err, call.want) {
			t.Fatal("malformed queued", err)
		}
	}
	h.mu.Lock()
	h.sessions[s.ID].Driver = "windows-uia"
	h.mu.Unlock() // isolated transport binding, no native app
	for tool, args := range structuredBrowserCalls {
		if sessionAllowsTool("windows-uia", tool) {
			t.Fatal("browser operation in native menu")
		}
		raw, _ := json.Marshal(map[string]any{"tool": tool, "arguments": json.RawMessage(args)})
		if _, err = h.Execute(context.Background(), "alice", s.ID, "run", "bad", "computer_prepare", raw); !errors.Is(err, ErrControlScope) {
			t.Fatal("native prepare forwarded browser operation", err)
		}
	}
	if action, _ := h.Poll(token); action != nil {
		t.Fatal("rejected request queued")
	}
}
