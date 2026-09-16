package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestSessionCommandsUseAuthenticatedService(t *testing.T) {
	session := sessions.NewSession("Notes · 日本語 #50%", "local-model")
	session.AddMessage("user", "hello")
	session.AddMessage("assistant", "saved answer")
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Error("missing authentication")
		}
		if r.URL.Path == "/v1/sessions" {
			json.NewEncoder(w).Encode(map[string]any{"sessions": []*sessions.Session{session}})
			return
		}
		if r.URL.EscapedPath() != "/v1/sessions/"+url.PathEscape(session.Name) {
			t.Errorf("wrong resource path: %s", r.URL.EscapedPath())
		}
		if r.Method == "DELETE" {
			deleted = true
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		json.NewEncoder(w).Encode(session)
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "key", nil)
	for _, args := range [][]string{{"list"}, {"show", session.Name}, {"export", session.Name}, {"delete", session.Name}} {
		var out bytes.Buffer
		if err := sessionCommand(context.Background(), client, args, &out, true); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("not JSON: %s", out.String())
		}
	}
	if !deleted {
		t.Fatal("deletion never reached service")
	}
	filename := filepath.Join(t.TempDir(), "export.md")
	var out bytes.Buffer
	if err := exportServiceSession(context.Background(), client, session.Name, "markdown", filename, &out, true); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filename)
	if err != nil || !strings.Contains(string(content), "saved answer") {
		t.Fatalf("bad export: %q %v", content, err)
	}
	if err := exportServiceSession(context.Background(), client, session.Name, "markdown", filename, &out, true); err == nil {
		t.Fatal("overwrote existing export")
	}
}

func TestSessionPermissionFailureHasNoLocalFallback(t *testing.T) {
	root := t.TempDir()
	manager := sessions.NewSessionManager(filepath.Join(root, "sessions"))
	if err := manager.Save(sessions.NewSession("private", "model")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_DATA_DIR", root)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "Forbidden private details", 403) }))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "key", nil)
	for _, args := range [][]string{{"list"}, {"show", "private"}, {"delete", "private"}, {"export", "private"}} {
		var out bytes.Buffer
		err := sessionCommand(context.Background(), client, args, &out, true)
		var typed *serviceclient.Error
		if !errors.As(err, &typed) || typed.Status != 403 || out.Len() != 0 {
			t.Fatalf("%v fell back or leaked data: %q %v", args, out.String(), err)
		}
	}
	if _, err := manager.Load("private"); err != nil {
		t.Fatal("local session was altered")
	}
}

func TestSessionResponsesFailClosed(t *testing.T) {
	for _, test := range []struct {
		args     []string
		response string
	}{
		{[]string{"list"}, `{}`},
		{[]string{"list"}, `{"sessions":[{}]}`},
		{[]string{"show", "x"}, `{"name":"wrong"}`},
		{[]string{"delete", "x"}, `{"success":false}`},
		{[]string{"delete", "x"}, `{}`},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(test.response)) }))
		client, _ := serviceclient.New(server.URL, "", nil)
		var out bytes.Buffer
		if err := sessionCommand(context.Background(), client, test.args, &out, true); err == nil {
			t.Fatalf("accepted incomplete result: %v", test)
		}
		server.Close()
	}
}
