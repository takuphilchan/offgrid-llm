package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestModelPackageUsesAuthenticatedServiceAndInjectedRequestID(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("missing actor credential")
		}
		switch r.URL.Path {
		case "/api/v2/models":
			if r.Method != http.MethodGet || r.URL.Query().Get("category") != "language" {
				t.Errorf("unexpected model inventory request %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, `{"models":[{"id":"fixture","engine":"mlx"}]}`)
		case "/api/v2/models/resolve":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected resolve method %s", r.Method)
			}
			_, _ = io.WriteString(w, `{"id":"preview","resolution":{"manifest":{"name":"Fixture"}}}`)
		case "/api/v2/models/operations":
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["request_id"] != "fixed-request" || input["resolution_id"] != "preview" {
				t.Errorf("wrong operation request %#v err=%v", input, err)
			}
			_, _ = io.WriteString(w, `{"id":"operation","state":"queued"}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL)
		}
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "fixture-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	env.ModelPackage.NewRequestID = func() string { return "fixed-request" }
	if code := Execute(context.Background(), []string{"model", "list", "--category", "language"}, env); code != 0 || !strings.Contains(out.String(), `"engine":"mlx"`) {
		t.Fatalf("model list failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"model", "install", "fixture", "--yes", "--detach"}, env); code != 0 || !strings.Contains(out.String(), `"id":"operation"`) {
		t.Fatalf("model install handoff failed: code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if requests != 3 {
		t.Fatalf("unexpected service request count %d", requests)
	}
}
