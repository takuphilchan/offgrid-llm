package rag

import (
	"fmt"
	"testing"
)

func TestAnalysisHistoryPaginationAndRevocation(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	refs := analysisDocuments(t, s)
	for i := 0; i < 23; i++ {
		if _, err = s.CreateComparison("reader", fmt.Sprint(i), refs); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.AnalysisHistory("reader", 0, 20)
	if err != nil || len(page.Analyses) != 20 || page.NextOffset == nil || *page.NextOffset != 20 {
		t.Fatal(page, err)
	}
	next, err := s.AnalysisHistory("reader", 20, 20)
	if err != nil || len(next.Analyses) != 3 || next.NextOffset != nil {
		t.Fatal(next, err)
	}
	seen := map[string]bool{}
	for _, p := range []*AnalysisHistory{page, next} {
		for _, r := range p.Analyses {
			if seen[r.ID] || len(r.Documents) != 2 || r.Documents[0] != "before" || r.Documents[1] != "after" {
				t.Fatal(r)
			}
			seen[r.ID] = true
		}
	}
	other, err := s.AnalysisHistory("admin", 0, 20)
	if err != nil || len(other.Analyses) != 0 {
		t.Fatal("owner boundary", other, err)
	}
	policy, err := s.CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AssignDocumentCollection("owner", "before", policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	revoked, err := s.AnalysisHistory("reader", 0, 20)
	if err != nil || len(revoked.Analyses) != 0 || revoked.NextOffset == nil {
		t.Fatal("revoked identities leaked", revoked, err)
	}
	for _, args := range [][2]int{{-1, 20}, {257, 20}, {0, 0}, {0, 21}} {
		if _, err = s.AnalysisHistory("reader", args[0], args[1]); err != ErrAnalysisInvalid {
			t.Fatal(args, err)
		}
	}
}
