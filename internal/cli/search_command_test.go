package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type searchRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn searchRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSearchUsesInjectedTransportAndStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	called := 0
	client := &http.Client{Transport: searchRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		called++
		if request.URL.Host != "huggingface.co" || request.URL.Query().Get("search") != "fixture" {
			t.Errorf("unexpected search URL: %s", request.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("[]")), Request: request}, nil
	})}
	p := Presenter{Stdout: &stdout, Stderr: &stderr, JSON: true}
	if err := RunSearch(context.Background(), []string{"fixture"}, p, client); err != nil {
		t.Fatal(err)
	}
	if called != 1 || stdout.String() != "{\n  \"count\": 0,\n  \"results\": []\n}\n" || stderr.Len() != 0 {
		t.Fatalf("called=%d stdout=%q stderr=%q", called, stdout.String(), stderr.String())
	}
}

func TestSearchHelpAndInvalidUsageNeverUseNetwork(t *testing.T) {
	var stdout, stderr bytes.Buffer
	p := Presenter{Stdout: &stdout, Stderr: &stderr, Style: PresentationStyle{Search: "search"}}
	if err := RunSearch(context.Background(), []string{"--help"}, p, nil); err != nil || !strings.Contains(stdout.String(), "Search HuggingFace") {
		t.Fatalf("help: err=%v stdout=%q", err, stdout.String())
	}
	stdout.Reset()
	if err := RunSearch(context.Background(), []string{"--ram", "bad"}, p, nil); err == nil || stdout.Len() != 0 {
		t.Fatalf("invalid usage: err=%v stdout=%q", err, stdout.String())
	}
}
