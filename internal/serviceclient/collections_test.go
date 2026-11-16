package serviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestCollectionUpdateRejectsUnconfirmedAudienceWithoutReplay(t *testing.T) {
	for _, mode := range []string{"confirmed", "stale", "wrong-audience", "wrong-id", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "PUT" || r.URL.Path != "/v1/collections" || r.URL.Query().Get("id") != "private&id" {
					t.Error("wrong operation/scope")
				}
				var request rag.CollectionUpdate
				if json.NewDecoder(r.Body).Decode(&request) != nil || request.ExpectedRevision != 1 || request.Readers == nil || request.Editors == nil {
					t.Error("missing reviewed membership")
				}
				if mode == "conflict" {
					w.WriteHeader(409)
					_, _ = w.Write([]byte(`{"error":{"code":"collection_conflict","message":"changed"}}`))
					return
				}
				p := rag.CollectionPolicy{ID: "private&id", Version: 1, Revision: 2, Name: "Records", Owner: "owner", Readers: []string{"reader"}, Editors: []string{}}
				switch mode {
				case "stale":
					p.Revision = 1
				case "wrong-audience":
					p.Readers = []string{"other"}
				case "wrong-id":
					p.ID = "other"
				}
				_ = json.NewEncoder(w).Encode(p)
			}))
			defer s.Close()
			c, err := New(s.URL, "test-key", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.UpdateCollection(context.Background(), "private&id", rag.CollectionUpdate{ExpectedRevision: 1, Name: "Records", Readers: []string{"reader"}, Editors: []string{}})
			if (err == nil) != (mode == "confirmed") || calls != 1 {
				t.Fatal(mode, calls, err)
			}
		})
	}
}
