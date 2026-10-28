package agents

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPGetRejectsLocalAndInvalidTargetsBeforeEffects(t *testing.T) {
	for _, target := range []string{"", "file:///tmp/private", "file:///<current_directory>", "/tmp/private", `C:\private`, "https:///missing-host", "ftp://example.test/a", "http://bad\nheader"} {
		t.Run(target, func(t *testing.T) {
			_, err := executeHTTPGet(context.Background(), target)
			var failure *ToolFailure
			if !errors.As(err, &failure) || !failure.BeforeEffects || failure.Code != "tool_http_url_rejected" || failure.Recovery == "" {
				t.Fatalf("expected a proven pre-dispatch failure, got %v", err)
			}
		})
	}
}

func TestHTTPGetPreservesRemoteRequestAndTransportUncertainty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("observed remote content")) }))
	result, err := executeHTTPGet(context.Background(), server.URL)
	if err != nil || !strings.Contains(result, "observed remote content") {
		t.Fatal(result, err)
	}
	server.Close()
	_, err = executeHTTPGet(context.Background(), server.URL)
	var failure *ToolFailure
	if err == nil || errors.As(err, &failure) && failure.BeforeEffects {
		t.Fatalf("transport failure is not pre-dispatch evidence: %v", err)
	}
}
