package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

type repositorySearcher interface {
	SearchModelsContext(context.Context, models.SearchFilter) ([]models.SearchResult, error)
}

func (s *Server) handleModelSearch(w http.ResponseWriter, r *http.Request) {
	searchRepositories(w, r, models.NewHuggingFaceClient())
}

func searchRepositories(w http.ResponseWriter, r *http.Request, hf repositorySearcher) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	filter := models.SearchFilter{Query: r.URL.Query().Get("query"), Author: r.URL.Query().Get("author"), SortBy: r.URL.Query().Get("sort"), Limit: 20}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&filter); err != nil {
			writeError(w, "Invalid search body", http.StatusBadRequest)
			return
		}
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 200 || filter.Limit < 1 || filter.Limit > 50 {
		writeError(w, "Query must be at most 200 characters; limit must be 1–50", http.StatusBadRequest)
		return
	}
	switch filter.SortBy {
	case "", "downloads", "likes", "created", "modified", "relevance":
	default:
		writeError(w, "Unsupported search sort", http.StatusBadRequest)
		return
	}
	filter.MetadataOnly, filter.OnlyGGUF, filter.ExcludeGated, filter.ExcludePrivate = true, true, true, true
	type repository struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Author    string `json:"author"`
		Downloads int64  `json:"downloads"`
		Likes     int    `json:"likes"`
	}
	response := struct {
		Total   int          `json:"total"`
		Results []repository `json:"results"`
	}{Results: []repository{}}
	if filter.Query != "" {
		results, err := hf.SearchModelsContext(r.Context(), filter)
		if err != nil {
			writeError(w, "Hugging Face search is unavailable. Check the service's internet connection and retry.", http.StatusBadGateway)
			return
		}
		for _, result := range results {
			id := result.Model.ID
			if !models.ValidHubRepository(id) {
				continue
			}
			parts := strings.SplitN(id, "/", 2)
			response.Results = append(response.Results, repository{ID: id, Author: parts[0], Name: parts[1], Downloads: result.Model.Downloads, Likes: result.Model.Likes})
		}
		response.Total = len(response.Results)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleModelSearchFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	repo := r.URL.Query().Get("repo")
	if !models.ValidHubRepository(repo) {
		writeError(w, "Repository must be owner/name", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	files, err := models.NewHuggingFaceClient().DiscoverFiles(ctx, repo)
	if err != nil {
		writeError(w, "Could not load model files. Check internet access and whether this repository is public, then retry.", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"repo": repo, "files": files})
}
