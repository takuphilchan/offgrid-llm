package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStrictContractLoading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.json")
	for _, raw := range []string{`{"version":1,"kind":"no_changes","workspace":"workspace"}`, `{"version":1,"version":1,"kind":"no_changes","workspace":"workspace"}`, `{"version":1,"kind":"no_changes","workspace":"workspace","unknown":true}`, `{"workspace":"workspace"} {}`, strings.Repeat(" ", 16385)} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		c, err := LoadContract(path)
		if strings.HasPrefix(raw, `{"version":1,"kind":"no_changes","workspace":"workspace"}`) {
			if err != nil || c.Workspace != filepath.Join(dir, "workspace") {
				t.Fatalf("%+v %v", c, err)
			}
		} else if err == nil {
			t.Fatalf("accepted invalid contract %q", raw[:min(80, len(raw))])
		}
	}
	if _, err := LoadContract(dir); err == nil {
		t.Fatal("accepted directory")
	}
}

func hookRequest(t *testing.T, s *Session, path string, e hookEvent, mutate func(*http.Request)) (int, Decision) {
	t.Helper()
	c := s.Connection()
	e.Version, e.Adapter, e.Run = Version, c.Adapter, c.RunID
	data, _ := json.Marshal(e)
	r, _ := http.NewRequest(http.MethodPost, c.URL+path, bytes.NewReader(data))
	r.Header.Set("Authorization", "Bearer "+c.Token)
	r.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(r)
	}
	client := &http.Client{Timeout: time.Second * 3, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var decision Decision
	_ = json.NewDecoder(response.Body).Decode(&decision)
	return response.StatusCode, decision
}

func TestAuthenticatedHookLifecycleAndFinalFiles(t *testing.T) {
	dir := t.TempDir()
	content := []byte("1| café 日本語\n")
	if err := os.WriteFile(filepath.Join(dir, "source.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Start(context.Background(), Contract{Version: 1, Kind: ExactCopy, Workspace: dir, Source: "source.txt", Destination: "copy.txt"}, HermesAdapter)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, mutate := range []func(*http.Request){func(r *http.Request) { r.Header.Del("Authorization") }, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, func(r *http.Request) { r.Host = "evil.example" }, func(r *http.Request) { r.Method = "GET" }} {
		status, _ := hookRequest(t, s, "/bind", hookEvent{Session: "s"}, mutate)
		if status != 403 {
			t.Fatal(status)
		}
	}
	_, decision := hookRequest(t, s, "/bind", hookEvent{Session: "s"}, nil)
	if !decision.Allowed {
		t.Fatal(decision)
	}
	for i, op := range []Operation{{Kind: "read", Path: "source.txt"}, {Kind: "write", Path: "copy.txt", Content: content}, {Kind: "read", Path: "copy.txt"}} {
		call := string(rune('a' + i))
		_, d := hookRequest(t, s, "/before", hookEvent{Session: "s", Turn: "t", Call: call, Kind: op.Kind, Path: filepath.Join(dir, op.Path), Content: string(op.Content)}, nil)
		if !d.Allowed {
			t.Fatal(d)
		}
		if op.Kind == "write" {
			if err := os.WriteFile(filepath.Join(dir, "copy.txt"), content, 0600); err != nil {
				t.Fatal(err)
			}
		}
		_, d = hookRequest(t, s, "/after", hookEvent{Session: "s", Turn: "t", Call: call, Status: "completed", CompleteRead: op.Kind == "read"}, nil)
		if !d.Allowed {
			t.Fatal(d)
		}
	}
	_, d := hookRequest(t, s, "/end", hookEvent{Session: "s", Status: "completed"}, nil)
	if !d.Allowed {
		t.Fatal(d)
	}
	r := s.Finish(context.Background(), "completed")
	if r.State != "passed" || r.ObservedCalls != 3 {
		t.Fatal(r)
	}
	if r != s.Finish(context.Background(), "failed") {
		t.Fatal("receipt changed")
	}
}

func TestMissingClosureReplayAndCancellationCannotPass(t *testing.T) {
	for _, mode := range []string{"missing_bind", "missing_end", "replay", "foreign", "cancelled", "late_event", "completed"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, err := Start(ctx, Contract{Version: 1, Kind: NoChanges, Workspace: t.TempDir()}, OpenClawAdapter)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if mode != "missing_bind" {
				hookRequest(t, s, "/bind", hookEvent{Session: "s"}, nil)
			}
			if mode == "replay" {
				hookRequest(t, s, "/bind", hookEvent{Session: "s"}, nil)
			}
			if mode == "foreign" {
				hookRequest(t, s, "/before", hookEvent{Session: "other", Turn: "t", Call: "a", Kind: "read", Path: "x"}, nil)
			}
			if mode == "completed" || mode == "late_event" {
				hookRequest(t, s, "/end", hookEvent{Session: "s", Status: "completed"}, nil)
			}
			if mode == "late_event" {
				hookRequest(t, s, "/before", hookEvent{Session: "s", Turn: "t", Call: "late", Kind: "write", Path: "out"}, nil)
			}
			if mode == "cancelled" {
				cancel()
			}
			r := s.Finish(ctx, "completed")
			if (r.State == "passed") != (mode == "completed") {
				t.Fatal(r)
			}
		})
	}
}

func TestNoReadOrFinishEndpointAndStrictEvents(t *testing.T) {
	s, err := Start(context.Background(), Contract{Version: 1, Kind: NoChanges, Workspace: t.TempDir()}, HermesAdapter)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, path := range []string{"/finish", "/files", "/execute"} {
		status, _ := hookRequest(t, s, path, hookEvent{Session: "s"}, nil)
		if status != 404 {
			t.Fatal(status)
		}
	}
	for _, body := range []string{`{"version":1,"version":1}`, `{"extra":true}`, strings.Repeat(" ", (8<<20)+1)} {
		status, _ := hookRequest(t, s, "/bind", hookEvent{}, func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader(body))
			r.ContentLength = int64(len(body))
		})
		if status != 400 {
			t.Fatal(status)
		}
	}
}
