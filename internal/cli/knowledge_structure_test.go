package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestKnowledgeStructureCLIReceiptAndNoCorrectionReplay(t *testing.T) {
	file := filepath.Join(t.TempDir(), "review.json")
	request := `{"document_id":"private&id","expected_revision":"old","edits":[{"block_id":"clause","expected_text_sha256":"` + strings.Repeat("a", 64) + `","text":"Reviewed text"}]}`
	if err := os.WriteFile(file, []byte(request), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"confirmed", `{"success":true,"document":{"id":"private&id","extraction_revision":"new"}}`, 200, true},
		{"stale receipt", `{"success":true,"document":{"id":"private&id","extraction_revision":"old"}}`, 200, false},
		{"conflict", `{"error_code":"document_extraction_conflict"}`, 409, false},
		{"unavailable", `{}`, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client, err := serviceclient.New("http://fixture", "fixture-key", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/v1/documents/corrections" || r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatal("wrong authority/endpoint")
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || body["expected_revision"] != "old" || body["author"] != nil {
					t.Fatal("wrong revision/author")
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			_, err = RunKnowledgeService(context.Background(), client, []string{"correct", file}, nil, io.Discard, false)
			if (err == nil) != tc.valid || calls != 1 {
				t.Fatal("correction replay/receipt failure", calls, err)
			}
		})
	}
	client, err := serviceclient.New("http://fixture", "fixture-key", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Query().Get("id") != "private&id" || r.URL.Query().Get("revision") != "old" || r.URL.Query().Get("offset") != "10" || r.URL.Query().Get("navigation_offset") != "100" {
			t.Fatal("wrong structure scope", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"document_id":"private&id","revision":"old","offset":10,"relationship_offset":0,"navigation_offset":100,"navigation":[],"blocks":[]}`))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RunKnowledgeService(context.Background(), client, []string{"structure", "private&id", "--revision", "old", "--offset", "10", "--navigation-offset", "100"}, nil, io.Discard, false); err != nil {
		t.Fatal(err)
	}
}
