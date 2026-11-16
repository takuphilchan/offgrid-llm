package rag

import (
	"context"
	"testing"
)

func TestCompositionAllExcludedRetainsOmissionsWithoutGeneration(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	parent := completedCompositionParent(t, s, "owner")
	for batch := 0; batch < parent.CompletedBatches; batch++ {
		r, err := s.SynthesisReview(ctx, "owner", parent.ID, batch)
		if err != nil {
			t.Fatal(err)
		}
		for i := range r.Decisions {
			r.Decisions[i].Decision = "exclude"
		}
		if _, err = s.SaveSynthesisReview(ctx, "owner", SynthesisReviewRequest{ID: parent.ID, Batch: batch, BatchID: r.BatchID, ResponseSHA256: r.ResponseSHA256, Decisions: r.Decisions, SectionChecked: true}); err != nil {
			t.Fatal(err)
		}
	}
	job, err := s.CreateComposition(ctx, "owner", "empty", parent.ID, synthesisTestIdentity())
	if err != nil || job.State != "awaiting_review" || len(job.Plan.Tasks) != 0 {
		t.Fatal(job, err)
	}
	report, err := s.CompositionReportView(ctx, "owner", job.ID)
	if err != nil || len(report.Sections) != 0 || report.UnrepresentedBatches != parent.CompletedBatches || len(report.Gaps) == 0 || !report.SemanticReviewRequired {
		t.Fatal("omissions hidden", report, err)
	}
	artifact, err := s.ExportComposition(ctx, "owner", job.ID, "json")
	if err != nil || artifact.Content == "" {
		t.Fatal("excluded source export", err)
	}
}
