package main

import (
	"bytes"
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

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type imageRoundTrip func(*http.Request) (*http.Response, error)

func (f imageRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTUIImageCommandsRetainFailedInputAndSaveHistory(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test image.png")
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	rt := &ChatRuntime{ModelName: "vision", SessionMgr: sessions.NewSessionManager(directory), CurrentSession: sessions.NewSession("tui-images", "vision")}
	m := newChatTUIModel(rt)
	m.handleCommand("image " + path)
	if len(rt.Images) != 1 {
		t.Fatal("command did not attach image")
	}
	original := rt.Images[0]
	calls := 0
	rt.Client = &http.Client{Transport: imageRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		var body api.ChatCompletionRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if has, err := api.ValidateChatMessages(body.Messages); err != nil || !has || len(body.Messages) != 1 {
			t.Fatal("lost image or committed failed exchange", err)
		}
		status := 200
		response := "data: {\"choices\":[{\"delta\":{\"content\":\"A square.\"}}]}\n\n"
		if calls == 1 {
			status = 422
			response = "projector unavailable"
		} else if calls == 3 {
			response += "data: {\"error\":{\"message\":\"inference failed\"}}\n\ndata: [DONE]\n\n"
		} else if calls == 4 {
			response += "data: {broken\n\ndata: [DONE]\n\n"
		} else if calls == 5 {
			response += "data: [DONE]\n\n"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
	})}
	for i := 0; i < 5; i++ {
		err := rt.stream("Describe", make(chan chatStreamMsg, 4))
		if i < 4 {
			if err == nil || len(rt.Messages) != 0 || !reflect.DeepEqual(rt.Images, []api.SessionImage{original}) {
				t.Fatal("failure consumed input", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if len(rt.Images) != 0 || len(rt.Messages) != 2 {
		t.Fatal("successful turn not committed")
	}
	loaded, err := sessions.NewSessionManager(directory).Load("tui-images")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Messages[0].Images, []api.SessionImage{original}) {
		t.Fatal("image not saved")
	}
	restored := cli.SavedChatMessages(loaded)
	if restored[0].ImageCount() != 1 || restored[0].StringContent() != "Describe" {
		t.Fatal("restore lost text/image")
	}
	m.handleCommand("image " + path)
	m.handleCommand("remove-image all")
	if len(rt.Images) != 0 {
		t.Fatal("remove command failed")
	}
}
