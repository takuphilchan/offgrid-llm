package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestDownloadAliasesUseAuthenticatedServiceWithoutLocalFallback(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("missing actor credential")
		}
		switch r.URL.Path {
		case "/v1/catalog":
			if r.URL.Query().Get("quant") != "" {
				t.Errorf("unexpected catalog filter %s", r.URL)
			}
			_, _ = io.WriteString(w, `{"models":[{"id":"fixture","repo":"owner/repo","file":"fixture.gguf","quant":"Q4_K_M"}]}`)
		case "/v1/models/download":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected download method %s", r.Method)
			}
			_, _ = io.WriteString(w, `{"success":true,"file_name":"fixture.gguf","exists":true}`)
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
	env.ServerURL = server.URL
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	for _, alias := range []string{"download", "download-hf"} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), []string{alias, "fixture"}, env); code != 0 || !strings.Contains(out.String(), "complete · fixture.gguf") {
			t.Fatalf("%s failed: code=%d stdout=%q stderr=%q", alias, code, out.String(), errOut.String())
		}
	}
	if requests != 4 {
		t.Fatalf("unexpected request count %d", requests)
	}
	missing := env
	missing.Service.Client = nil
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"download", "fixture"}, missing); code != 1 || !strings.Contains(errOut.String(), "authenticated service client unavailable") {
		t.Fatalf("missing service did not fail closed: code=%d stderr=%q", code, errOut.String())
	}
}
