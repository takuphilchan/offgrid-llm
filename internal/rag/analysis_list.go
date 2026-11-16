package rag

import (
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"time"
)

// History contains identities and progress only, never document bodies. The
// cursor advances over inaccessible jobs as well, without disclosing their IDs.
type AnalysisSummary struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Documents []string  `json:"documents"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type AnalysisHistory struct {
	Analyses   []AnalysisSummary `json:"analyses"`
	NextOffset *int              `json:"next_offset"`
}

func (s *SQLiteStore) AnalysisHistory(owner string, offset, limit int) (*AnalysisHistory, error) {
	if !validKnowledgeActor(owner) || offset < 0 || offset > 256 || limit < 1 || limit > 20 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Read only the bounded identity projection; history must not deserialize
	// every source unit in every potentially multi-megabyte analysis plan.
	rows, err := s.db.Query(`SELECT a.id,a.state,a.created_at,a.updated_at,b.document_id,c.document_id
	 FROM document_analyses a
	 JOIN document_analysis_sources b ON b.analysis_id=a.id AND b.ordinal=0
	 JOIN document_analysis_sources c ON c.analysis_id=a.id AND c.ordinal=1
	 WHERE a.owner=? AND a.schema_version=1 ORDER BY a.created_at DESC,a.id DESC LIMIT ? OFFSET ?`, owner, limit+1, offset)
	if err != nil {
		return nil, err
	}
	items := []AnalysisSummary{}
	for rows.Next() {
		item := AnalysisSummary{Documents: make([]string, 2)}
		if err = rows.Scan(&item.ID, &item.State, &item.CreatedAt, &item.UpdatedAt, &item.Documents[0], &item.Documents[1]); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	page := &AnalysisHistory{Analyses: []AnalysisSummary{}}
	if len(items) > limit {
		next := offset + limit
		page.NextOffset = &next
		items = items[:limit]
	}
	for _, item := range items {
		e := analysisAccess(s.db, owner, &document.AnalysisPlan{Sources: []document.SourceRef{{DocumentID: item.Documents[0]}, {DocumentID: item.Documents[1]}}})
		if e == ErrAnalysisUnavailable {
			continue
		}
		if e != nil {
			return nil, e
		}
		page.Analyses = append(page.Analyses, item)
	}
	return page, nil
}
