package rag

import "github.com/takuphilchan/offgrid-llm/internal/document"

func (s *SQLiteStore) SynthesisHistory(owner string, offset, limit int) (*AnalysisHistory, error) {
	if !validKnowledgeActor(owner) || offset < 0 || offset > 256 || limit < 1 || limit > 20 {
		return nil, ErrAnalysisInvalid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id,state,created_at,updated_at FROM document_analyses WHERE owner=? AND schema_version=2 ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, owner, limit+1, offset)
	if err != nil {
		return nil, err
	}
	items := []AnalysisSummary{}
	for rows.Next() {
		var item AnalysisSummary
		if err = rows.Scan(&item.ID, &item.State, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
		refs := []document.SourceRef{}
		item.Documents = []string{}
		rows, err = s.db.Query(`SELECT document_id,revision FROM document_analysis_sources WHERE analysis_id=? ORDER BY ordinal LIMIT 17`, item.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ref document.SourceRef
			if err = rows.Scan(&ref.DocumentID, &ref.Revision); err != nil {
				rows.Close()
				return nil, err
			}
			refs = append(refs, ref)
			item.Documents = append(item.Documents, ref.DocumentID)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if len(refs) < 1 || len(refs) > 16 {
			return nil, ErrAnalysisInvalid
		}
		if err = analysisAccess(s.db, owner, &document.AnalysisPlan{Sources: refs}); err == ErrAnalysisUnavailable {
			continue
		} else if err != nil {
			return nil, err
		}
		page.Analyses = append(page.Analyses, item)
	}
	return page, nil
}
