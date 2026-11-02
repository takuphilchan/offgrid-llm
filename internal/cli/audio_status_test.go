package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestAudioStatusUsesSelectedAuthenticatedService(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		var out, errOut bytes.Buffer
		env := testEnvironment(&out, &errOut, &fakeRunner{})
		env.Options.JSON = jsonMode
		env.Service.Client = func() (*serviceclient.Client, error) {
			return serviceclient.New("http://selected-service:1234", "secret", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "http://selected-service:1234/v1/audio/status" || r.Header.Get("Authorization") != "Bearer secret" {
					t.Fatalf("wrong authenticated request: %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"asr":{"available":false,"issue":"Qwen runtime missing"},"tts":{"available":false},"profiles":[{"name":"Qwen\u001b[31m","adapter":"qwen3-asr","available":false}]}`))}, nil
			})})
		}
		if err := audioStatusCommand(context.Background(), env); err != nil {
			t.Fatal(err)
		}
		if jsonMode {
			if !json.Valid(out.Bytes()) {
				t.Fatalf("invalid JSON: %s", out.String())
			}
		} else if !strings.Contains(out.String(), "Qwen runtime missing") || strings.Contains(out.String(), "setup whisper") || strings.Contains(out.String(), "\x1b") {
			t.Fatalf("misleading/unsafe status: %s", out.String())
		}
	}
}

func TestAudioStatusRejectsHTTPFailureAndHonorsCancellation(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New("http://fixture", "", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
			if err := r.Context().Err(); err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 401, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"asr":{"available":true}}`))}, nil
		})})
	}
	if err := audioStatusCommand(context.Background(), env); err == nil {
		t.Fatal("401 was presented as readiness")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := audioStatusCommand(ctx, env); err == nil {
		t.Fatal("cancel ignored")
	}
	if out.Len() != 0 {
		t.Fatalf("failure printed success: %s", out.String())
	}
}
