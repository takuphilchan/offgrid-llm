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

func TestKnowledgeComparisonCLIReviewAndNoReplay(t *testing.T) {
	file := filepath.Join(t.TempDir(), "comparison.json")
	if err := os.WriteFile(file, []byte(`{"request_key":"reviewed","sources":[{"document_id":"before","revision":"old"},{"document_id":"after","revision":"new"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{202, 409, 503} {
		calls := 0
		c, err := serviceclient.New("http://fixture", "key", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Path != "/v1/document-analyses" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer key" {
				t.Fatal("wrong operation or scope")
			}
			var request map[string]any
			if json.NewDecoder(r.Body).Decode(&request) != nil || request["request_key"] != "reviewed" || request["workflow"] != "comparison" {
				t.Fatal("missing request identity")
			}
			body := `{"id":"job","state":"queued","completed_steps":0,"completed_units":0,"pairs":[{"before_unit":"u","after_unit":"v","alignment":"position"}],"plan":{"id":"plan","version":"document-analysis-v1","workflow":"comparison","sources":[{"document_id":"before","revision":"old"},{"document_id":"after","revision":"new"}],"units":[{"source":0,"unit_id":"u"},{"source":1,"unit_id":"v"}],"gaps":[]}}`
			return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})})
		if err != nil {
			t.Fatal(err)
		}
		_, err = RunKnowledgeService(context.Background(), c, []string{"compare", file}, strings.NewReader(""), io.Discard, true)
		if (err == nil) != (code == 202) || calls != 1 {
			t.Fatal(code, calls, err)
		}
	}
}

func TestKnowledgeAnalysisHistoryCLI(t *testing.T) {
	calls := 0
	c, err := serviceclient.New("http://fixture", "key", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/document-analyses" || r.URL.Query().Get("offset") != "20" || r.URL.Query().Get("limit") != "10" {
			t.Fatal("wrong history operation")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"analyses":[],"next_offset":null}`))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RunKnowledgeService(context.Background(), c, []string{"analyses", "20", "10"}, strings.NewReader(""), io.Discard, true); err != nil || calls != 1 {
		t.Fatal(calls, err)
	}
	if _, err = RunKnowledgeService(context.Background(), c, []string{"analyses", "-1"}, strings.NewReader(""), io.Discard, true); err == nil || calls != 1 {
		t.Fatal("invalid history was sent", calls, err)
	}
}
