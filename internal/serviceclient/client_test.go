package serviceclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientAuthenticationAndSafeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-key" {
			t.Error("missing authentication")
		}
		w.Header().Set("X-Request-ID", "request-123")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "secret=private-key\x1b[2J")
	}))
	defer server.Close()
	client, err := New(server.URL, "private-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.JSON(context.Background(), "GET", "/v1/documents", nil, &map[string]any{})
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "forbidden" || failure.RequestID != "request-123" || strings.Contains(err.Error(), "private-key") {
		t.Fatalf("unsafe error: %#v", err)
	}
}

func TestClientNeverFollowsRedirects(t *testing.T) {
	visited := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, _ := New(server.URL, "secret", nil)
	_, err := client.Do(context.Background(), "POST", "/v1/documents", "application/json", strings.NewReader(`{"private":"body"}`))
	if err == nil || visited {
		t.Fatal("redirect followed or silently succeeded")
	}
}

func TestClientRejectsForeignOriginsAndMalformedJSON(t *testing.T) {
	for _, address := range []string{"https://user:password@localhost", "http://localhost/api", "http://localhost?key=secret", "file:///tmp", "http://localhost#x"} {
		if _, err := New(address, "", nil); err == nil {
			t.Errorf("accepted %s", address)
		}
	}
	client, _ := New("http://127.0.0.1:1", "", nil)
	for _, path := range []string{"https://example.org", "//example.org", "/path#fragment"} {
		if _, err := client.Do(context.Background(), "GET", path, "", nil); err == nil {
			t.Errorf("accepted %s", path)
		}
	}
	for _, body := range []string{"<html>secret</html>", `{}`, `{"ok":true} {}`, "null"} {
		var result map[string]any
		err := Decode(strings.NewReader(body), &result)
		if body == "{}" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err == nil {
			t.Errorf("accepted %q", body)
		}
	}
}

func TestClientCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	client, _ := New(server.URL, "", &http.Client{Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-started; cancel() }()
	_, err := client.Do(ctx, "GET", "/status", "", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestCancellationWhileDecodingRemainsCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"pending":`)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client, _ := New(server.URL, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	err := client.JSON(ctx, "GET", "/status", nil, &map[string]any{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("body cancellation lost: %v", err)
	}
}
