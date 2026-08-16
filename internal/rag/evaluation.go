package rag

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// EvaluationCase is a deterministic retrieval test. At least one expected
// document must appear in the returned top-k results for the case to pass.
type EvaluationCase struct {
	Name                string   `json:"name,omitempty"`
	Query               string   `json:"query"`
	ExpectedDocumentIDs []string `json:"expected_document_ids"`
	TopK                int      `json:"top_k,omitempty"`
}

type EvaluationCaseResult struct {
	Name          string   `json:"name,omitempty"`
	Query         string   `json:"query"`
	Passed        bool     `json:"passed"`
	FirstHitRank  int      `json:"first_hit_rank,omitempty"`
	ReturnedIDs   []string `json:"returned_document_ids"`
	LatencyMillis int64    `json:"latency_ms"`
	Error         string   `json:"error,omitempty"`
}

type EvaluationReport struct {
	Cases       []EvaluationCaseResult `json:"cases"`
	Total       int                    `json:"total"`
	Passed      int                    `json:"passed"`
	HitRate     float64                `json:"hit_rate"`
	MeanRR      float64                `json:"mean_reciprocal_rank"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
}

// Evaluate runs retrieval-only quality checks without asking a language model
// to judge its own output.
func (e *Engine) Evaluate(ctx context.Context, cases []EvaluationCase) (EvaluationReport, error) {
	if len(cases) == 0 {
		return EvaluationReport{}, fmt.Errorf("at least one evaluation case is required")
	}
	report := EvaluationReport{Cases: make([]EvaluationCaseResult, 0, len(cases)), Total: len(cases), EvaluatedAt: time.Now().UTC()}
	var reciprocalRank float64
	for _, testCase := range cases {
		result := EvaluationCaseResult{Name: testCase.Name, Query: testCase.Query, ReturnedIDs: []string{}}
		if strings.TrimSpace(testCase.Query) == "" || len(testCase.ExpectedDocumentIDs) == 0 {
			result.Error = "query and expected_document_ids are required"
			report.Cases = append(report.Cases, result)
			continue
		}
		topK := testCase.TopK
		if topK <= 0 {
			topK = 5
		}
		started := time.Now()
		contextResult, err := e.Search(ctx, testCase.Query, SearchOptions{TopK: topK, MinScore: 0, IncludeContent: false})
		result.LatencyMillis = time.Since(started).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			report.Cases = append(report.Cases, result)
			continue
		}
		expected := make(map[string]struct{}, len(testCase.ExpectedDocumentIDs))
		for _, id := range testCase.ExpectedDocumentIDs {
			expected[id] = struct{}{}
		}
		for i, item := range contextResult.Results {
			result.ReturnedIDs = append(result.ReturnedIDs, item.DocumentID)
			if result.FirstHitRank == 0 {
				if _, ok := expected[item.DocumentID]; ok {
					result.FirstHitRank = i + 1
					result.Passed = true
				}
			}
		}
		if result.Passed {
			report.Passed++
			reciprocalRank += 1 / float64(result.FirstHitRank)
		}
		report.Cases = append(report.Cases, result)
	}
	report.HitRate = float64(report.Passed) / float64(report.Total)
	report.MeanRR = reciprocalRank / float64(report.Total)
	return report, nil
}
