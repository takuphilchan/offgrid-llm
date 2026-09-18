package server

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRepositorySearcher func(context.Context, models.SearchFilter) ([]models.SearchResult, error)

func (f fakeRepositorySearcher) SearchModelsContext(ctx context.Context, filter models.SearchFilter) ([]models.SearchResult, error) {
	return f(ctx, filter)
}
func TestModelSearchPostBodyAndForcedPublicFilters(t *testing.T) {
	fake := fakeRepositorySearcher(func(ctx context.Context, filter models.SearchFilter) ([]models.SearchResult, error) {
		if filter.Query != "large model" || !filter.MetadataOnly || !filter.ExcludeGated || !filter.ExcludePrivate || !filter.OnlyGGUF {
			t.Fatalf("wrong filters: %+v", filter)
		}
		return []models.SearchResult{{Model: models.HFModel{ID: "owner/model"}}}, nil
	})
	w := httptest.NewRecorder()
	searchRepositories(w, httptest.NewRequest("POST", "/v1/search", strings.NewReader(`{"query":"large model","ExcludePrivate":false}`)), fake)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"owner/model"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func TestSearchFailureAndValidation(t *testing.T) {
	calls := 0
	fake := fakeRepositorySearcher(func(context.Context, models.SearchFilter) ([]models.SearchResult, error) {
		calls++
		return nil, errors.New("private internal proxy details")
	})
	for _, item := range []struct {
		url    string
		status int
	}{{"/v1/search?query=model", 502}, {"/v1/search?query=model&sort=bad", 400}, {"/v1/search", 200}} {
		w := httptest.NewRecorder()
		searchRepositories(w, httptest.NewRequest(http.MethodGet, item.url, nil), fake)
		if w.Code != item.status || strings.Contains(w.Body.String(), "private internal") {
			t.Fatalf("%s: %d %s", item.url, w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("unnecessary network calls: %d", calls)
	}
}
