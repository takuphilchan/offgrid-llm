package serviceclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDocumentAnalysisExportClientReceipt(t *testing.T) {
	for _, synthesis := range []bool{false, true} {
		name := "comparison"
		if synthesis {
			name = "synthesis"
		}
		t.Run(name, func(t *testing.T) { testDocumentExportReceipt(t, synthesis) })
	}
}

func testDocumentExportReceipt(t *testing.T, synthesis bool) {
	text := "政策 17.25"
	hash := sha256.Sum256([]byte(text))
	calls := 0
	body := DocumentAnalysisExport{AnalysisID: "job", Format: "json", Content: text, SHA256: hex.EncodeToString(hash[:])}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		path := "/v1/document-analyses/export"
		if synthesis {
			path = "/v1/document-analyses/synthesis/export"
		}
		if r.URL.Path != path || r.URL.Query().Get("id") != "job" || r.Header.Get("Authorization") != "Bearer key" {
			t.Error("identity/auth")
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()
	client, err := New(server.URL, "key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	export := client.ExportDocumentAnalysis
	if synthesis {
		export = client.ExportDocumentSynthesis
	}
	if got, err := export(context.Background(), "job", "json"); err != nil || got.Content != text {
		t.Fatal(got, err)
	}
	for _, mutate := range []func(){func() { body.Content = "forged" }, func() { body.Content = text; body.AnalysisID = "other" }, func() { body.AnalysisID = "job"; body.Format = "csv" }} {
		mutate()
		if _, err = export(context.Background(), "job", "json"); err == nil {
			t.Fatal("forged receipt accepted")
		}
	}
	if calls != 4 {
		t.Fatal("unexpected replay", calls)
	}
	if _, err = export(context.Background(), "job", "html"); err == nil || calls != 4 {
		t.Fatal("invalid format dispatched")
	}
}
