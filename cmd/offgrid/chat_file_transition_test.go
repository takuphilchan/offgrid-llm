package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestTUIFileTransitionKeepsContextAfterFailedUpload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/api/v2/system" {
			fmt.Fprint(w, `{"product":"offgrid","capabilities":["session-files-v1"]}`)
			return
		}
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"offline"}}`)
	}))
	defer s.Close()
	client, _ := serviceclient.New(s.URL, "", s.Client())
	starts := 0
	original := sessions.NewSession("local", "model")
	original.AddMessage("user", "earlier")
	rt := &ChatRuntime{Context: context.Background(), ModelName: "model", CurrentSession: original, Messages: []ChatMessage{{Role: "user", Content: "earlier"}}, Images: []api.SessionImage{{Name: "pending"}}}
	rt.StartFiles = func(ctx context.Context, model string, history []cli.ChatMessage) (*cli.ServiceFileConversation, error) {
		starts++
		if model != "model" || len(history) != 1 || history[0].StringContent() != "earlier" {
			t.Fatal("history not supplied")
		}
		return &cli.ServiceFileConversation{Context: ctx, Client: client, Name: "copied", Saved: sessions.NewSession("copied", "model")}, nil
	}
	m := newChatTUIModel(rt)
	msg := rt.fileCommand("attachments")()
	updated, _ := m.Update(msg)
	m = updated.(chatTUIModel)
	if starts != 0 || rt.Files != nil || requests != 0 {
		t.Fatal("listing created state")
	}
	msg = rt.fileCommand("attach " + path)()
	updated, _ = m.Update(msg)
	m = updated.(chatTUIModel)
	if starts != 1 || rt.Files == nil || rt.CurrentSession.Name != "copied" || len(rt.Images) != 1 || original.Name != "local" || original.Messages[0].Content != "earlier" {
		t.Fatal("lost state on failure")
	}
	msg = rt.fileCommand("attach " + path)()
	m.Update(msg)
	if starts != 1 {
		t.Fatal("repeated creation on retry")
	}
	before := requests
	if _, err := rt.Files.Stream("model", "question", nil, false, nil); err == nil || requests != before {
		t.Fatal("failed upload became a text-only request")
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), "offgrid session chat copied") {
		t.Fatal("missing recovery identity")
	}
	rt.fileWork.Wait()
}

func TestTUIFileTransitionCancellationReapsRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt := &ChatRuntime{Context: ctx, StartFiles: func(ctx context.Context, _ string, _ []cli.ChatMessage) (*cli.ServiceFileConversation, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	command := rt.fileCommand("attach " + path)
	cancel()
	msg := command().(chatFileCommandMsg)
	if msg.err != context.Canceled || msg.files != nil {
		t.Fatal(msg.err)
	}
	rt.fileWork.Wait()
}

func TestTUIFailedSendRestoresTextDraft(t *testing.T) {
	m := newChatTUIModel(&ChatRuntime{})
	m.pendingInput = "question about café"
	m.busy = true
	updated, _ := m.Update(chatStreamMsg{err: fmt.Errorf("service unavailable")})
	m = updated.(chatTUIModel)
	if m.busy || m.input.Value() != "question about café" {
		t.Fatal("lost text draft")
	}
	m.input.SetValue("new draft typed while waiting")
	updated, _ = m.Update(chatStreamMsg{err: fmt.Errorf("service unavailable")})
	if updated.(chatTUIModel).input.Value() != "new draft typed while waiting" {
		t.Fatal("overwrote newer input")
	}
}
