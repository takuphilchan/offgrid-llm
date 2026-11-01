package api

import "time"

// WorkSearchResult is a navigation projection, never an execution request.
type WorkSearchResult struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkSearchResponse struct {
	Results []WorkSearchResult `json:"results"`
	Scopes  []string           `json:"scopes"`
	Partial bool               `json:"partial"`
}
