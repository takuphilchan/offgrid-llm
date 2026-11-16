package serviceclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalysisHistoryClientRejectsInvalidReceipts(t *testing.T) {
	for _, body := range []string{`{"analyses":[],"next_offset":null}`, `{"analyses":[],"next_offset":0}`, `{"analyses":[{"id":"job","state":"unknown","documents":["a","b"],"created_at":"2026-11-13T00:00:00Z"}],"next_offset":null}`} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Query().Get("offset") != "0" || r.URL.Query().Get("limit") != "20" || r.Header.Get("Authorization") != "Bearer key" {
				t.Error("history request lost bounds or auth")
			}
			io.WriteString(w, body)
		}))
		c, e := New(server.URL, "key", server.Client())
		if e != nil {
			t.Fatal(e)
		}
		_, e = c.DocumentAnalysisHistory(context.Background(), 0, 20)
		server.Close()
		if (e == nil) != (body == `{"analyses":[],"next_offset":null}`) || calls != 1 {
			t.Fatal(body, e, calls)
		}
	}
}
