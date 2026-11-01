package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func cliImageFile(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "café image.png")
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChatImageCommandsSnapshotAndReject(t *testing.T) {
	path := cliImageFile(t)
	var images []api.SessionImage
	if handled, _, err := ChatImageCommand(`/image "`+path+`"`, &images); !handled || err != nil {
		t.Fatal(handled, err)
	}
	original := images[0]
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ChatImageCommand("/image "+path, &images); err == nil {
		t.Fatal("missing file accepted")
	}
	if !reflect.DeepEqual(images, []api.SessionImage{original}) {
		t.Fatal("snapshot changed")
	}
	for _, value := range []string{"/remove-image 0", "/remove-image 9", "/images bad", "/image"} {
		if _, _, err := ChatImageCommand(value, &images); err == nil {
			t.Fatal(value)
		}
	}
	for _, data := range [][]byte{[]byte("<svg/>"), bytes.Repeat([]byte("x"), api.MaxSessionImageBytes+1)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ChatImageCommand("/image "+path, &images); err == nil {
			t.Fatal("invalid file accepted")
		}
	}
	if _, err := ReadChatImage(filepath.Dir(path)); err == nil {
		t.Fatal("directory accepted")
	}
	path = cliImageFile(t)
	for len(images) < 4 {
		if _, _, err := ChatImageCommand("/image "+path, &images); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := ChatImageCommand("/image "+path, &images); err == nil {
		t.Fatal("fifth image accepted")
	}
	if _, _, err := ChatImageCommand("/remove-image 2", &images); err != nil || len(images) != 3 {
		t.Fatal(err)
	}
	if _, _, err := ChatImageCommand("/remove-image all", &images); err != nil || len(images) != 0 {
		t.Fatal(err)
	}
}

func TestCLIImageValidationPrecedesModelAndNetworkSetup(t *testing.T) {
	for _, args := range [][]string{{"model", "--image"}, {"model", "--image", filepath.Join(t.TempDir(), "missing.png")}} {
		var out, stderr bytes.Buffer
		env := testEnvironment(&out, &stderr, &fakeRunner{})
		if err := runInteractive(context.Background(), args, env); err == nil {
			t.Fatal("invalid flag accepted")
		}
	}
	images := []api.SessionImage{{Name: "large", DataURL: strings.Repeat("x", api.MaxSessionImageRequestBytes+1)}}
	if err := ValidateChatImageBudget(nil, images); err == nil {
		t.Fatal("unbounded history")
	}
	var out bytes.Buffer
	if !renderAgentSessionCommand(&out, "/image private.png", "test", agentSessionPresentation{}) || !strings.Contains(out.String(), "No image was attached or sent") {
		t.Fatal("agent image gate missing")
	}
}

func TestPlainChatImagesSurviveFailureAndSavedReload(t *testing.T) {
	path := cliImageFile(t)
	item, err := ReadChatImage(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	env := testEnvironment(&out, &stderr, &fakeRunner{})
	env.Stdin = strings.NewReader("/image " + path + "\nfirst\n/images\nsecond\n\nfollow-up\n/exit\n")
	directory := t.TempDir()
	manager := sessions.NewSessionManager(directory)
	session := sessions.NewSession("images", "vision")
	calls := 0
	client := &http.Client{Transport: localRoundTrip(func(request *http.Request) (*http.Response, error) {
		calls++
		var req api.ChatCompletionRequest
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		present, err := api.ValidateChatMessages(req.Messages)
		if err != nil || !present {
			t.Fatal("image lost", err)
		}
		if calls <= 3 && len(req.Messages) != 1 {
			t.Fatal("failed exchange committed")
		}
		if calls == 4 && len(req.Messages) != 3 {
			t.Fatal("completed history lost")
		}
		body := "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"
		status := 200
		if calls == 1 {
			status = 422
			body = "vision unavailable"
		} else if calls > 2 {
			body += "data: [DONE]\n\n"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if err := runPlainChat(context.Background(), env, &config.Config{ServerPort: 1234}, client, "vision", false, nil, nil, session, manager); err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatal(calls)
	}
	loaded, err := sessions.NewSessionManager(directory).Load("images")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 4 || !reflect.DeepEqual(loaded.Messages[0].Images, []api.SessionImage{item}) || len(loaded.Messages[2].Images) != 0 {
		t.Fatal("saved image mismatch")
	}
	history := SavedChatMessages(loaded)
	if history[0].ImageCount() != 1 || history[0].StringContent() != "" {
		t.Fatal("restored history lost content")
	}
	if !strings.Contains(out.String(), "Pending images (1/4)") || !strings.Contains(stderr.String(), "before completion") {
		t.Fatal(out.String(), stderr.String())
	}
}
