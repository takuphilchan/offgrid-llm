package serviceclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventStreamOutlivesOrdinaryRequestTimeout(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer private-key" {
			t.Error("stream lost authentication or became a mutation")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-time.After(100 * time.Millisecond):
			io.WriteString(w, "data: completed\n\n")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	httpClient := &http.Client{Timeout: 20 * time.Millisecond}
	client, _ := New(server.URL, "private-key", httpClient)
	response, err := client.Events(context.Background(), "/api/v2/jobs/run-fixture/events")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || !strings.Contains(string(data), "completed") || requests.Load() != 1 {
		t.Fatal("healthy stream terminated or retried", string(data), err, requests.Load())
	}
	if client.http.Timeout != 20*time.Millisecond || httpClient.Timeout != 20*time.Millisecond {
		t.Fatal("ordinary requests lost their timeout")
	}
}

func TestEventStreamCancellationAndStartupRemainBounded(t *testing.T) {
	for _, header := range []bool{false, true} {
		t.Run(map[bool]string{false: "startup", true: "body"}[header], func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if header {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			client, _ := New(server.URL, "", nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if !header {
				_, err := client.events(ctx, "/events", 20*time.Millisecond)
				var e *Error
				if !errors.As(err, &e) || e.Code != "timeout" {
					t.Fatal(err)
				}
				return
			}
			response, err := client.Events(ctx, "/events")
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			<-started
			cancel()
			done := make(chan error, 1)
			go func() { _, err := io.ReadAll(response.Body); done <- err }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled body completed")
				}
			case <-time.After(time.Second):
				t.Fatal("body ignored cancellation")
			}
		})
	}
}

func TestEventStreamKeepsOriginAndRedirectBoundaries(t *testing.T) {
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Store(true) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	client, _ := New(server.URL, "private-key", nil)
	if _, err := client.Events(context.Background(), "/events"); err == nil || followed.Load() {
		t.Fatal("stream followed a redirect", err)
	}
	if _, err := client.Events(context.Background(), target.URL); err == nil || followed.Load() {
		t.Fatal("stream accepted another origin", err)
	}
}
