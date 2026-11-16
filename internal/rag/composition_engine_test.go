package rag

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCompositionEngineResumesCommittedMergesAndPinsReviewHistory(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	parent := completedCompositionParent(t, s, "owner")
	r, err := s.CreateComposition(ctx, "owner", "report", parent.ID, synthesisTestIdentity())
	if err != nil {
		t.Fatal(err)
	}
	p := &compositionFixtureProvider{}
	p.failAt.Store(2)
	if err = e.StartComposition("owner", r.ID, p); err != nil {
		t.Fatal(err)
	}
	r = waitComposition(t, s, "owner", r.ID, "interrupted")
	if r.CompletedSteps != 1 || p.calls.Load() != 2 || r.ErrorCode != "composition_failed" {
		t.Fatal(r, p.calls.Load())
	}
	p.failAt.Store(0)
	e.analyses.wait.Wait()
	if err = e.StartComposition("owner", r.ID, p); err != nil {
		t.Fatal(err)
	}
	r = waitComposition(t, s, "owner", r.ID, "awaiting_review")
	e.analyses.wait.Wait()
	if r.CompletedSteps != len(r.Plan.Tasks) || int(p.calls.Load()) != len(r.Plan.Tasks)+1 || r.Attempt != 2 {
		t.Fatal(r, p.calls.Load())
	}
	report, current, err := s.CompositionReport(ctx, "owner", r.ID)
	if err != nil || !current || len(report.Sections) != 1 || len(report.Sections[0].Contributors) != parent.CompletedBatches || !report.SemanticReviewRequired {
		t.Fatal(report, current, err)
	}
	if _, _, err = s.CompositionReport(ctx, "other", r.ID); !errors.Is(err, ErrAnalysisUnavailable) {
		t.Fatal("private report leaked", err)
	}
	review, err := s.SynthesisReview(ctx, "owner", parent.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	decisions := make([]SynthesisDecision, len(review.Decisions))
	for i := range decisions {
		decisions[i].Decision = "exclude"
	}
	if _, err = s.SaveSynthesisReview(ctx, "owner", SynthesisReviewRequest{ID: parent.ID, Batch: 0, BatchID: review.BatchID, ResponseSHA256: review.ResponseSHA256, Decisions: decisions, SectionChecked: true}); err != nil {
		t.Fatal(err)
	}
	old, current, err := s.CompositionReport(ctx, "owner", r.ID)
	if err != nil || current || len(old.Sections[0].Contributors) != len(report.Sections[0].Contributors) {
		t.Fatal("historical report silently changed", old, current, err)
	}
	again, err := s.CreateComposition(ctx, "owner", "report", parent.ID, synthesisTestIdentity())
	if err != nil || again.ID != r.ID || again.ReviewsCurrent {
		t.Fatal("idempotency lost snapshot", again, err)
	}
	fresh, err := s.CreateComposition(ctx, "owner", "report-new", parent.ID, synthesisTestIdentity())
	if err != nil || fresh.Plan.ID == r.Plan.ID || fresh.Plan.Leaves[0].Decision != "exclude" {
		t.Fatal(fresh, err)
	}
}

func TestCompositionEngineWithholdsCancelledRevokedAndChangedReviewResults(t *testing.T) {
	for _, mode := range []string{"cancel", "revoke", "review", "close"} {
		t.Run(mode, func(t *testing.T) {
			e := NewEngine(nil, t.TempDir())
			defer e.Close()
			s, err := e.ImportStore()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			parent := completedCompositionParent(t, s, "reader")
			policy, err := s.CreateCollection("owner", "Private", []string{"reader"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.AssignDocumentCollection("owner", "source", policy.ID, 1); err != nil {
				t.Fatal(err)
			}
			r, err := s.CreateComposition(ctx, "reader", "report", parent.ID, synthesisTestIdentity())
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			p := &compositionFixtureProvider{hook: func(ctx context.Context) error {
				close(entered)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}
			if err = e.StartComposition("reader", r.ID, p); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("composition did not start")
			}
			switch mode {
			case "cancel":
				if err = e.CancelComposition("reader", r.ID); err != nil {
					t.Fatal(err)
				}
			case "revoke":
				if _, err = s.UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
					t.Fatal(err)
				}
				close(release)
			case "review":
				review, e := s.SynthesisReview(ctx, "reader", parent.ID, 0)
				if e != nil {
					t.Fatal(e)
				}
				decisions := make([]SynthesisDecision, len(review.Decisions))
				for i := range decisions {
					decisions[i].Decision = "keep"
				}
				if _, err = s.SaveSynthesisReview(ctx, "reader", SynthesisReviewRequest{ID: parent.ID, BatchID: review.BatchID, ResponseSHA256: review.ResponseSHA256, Decisions: decisions, SectionChecked: true}); err != nil {
					t.Fatal(err)
				}
				close(release)
			case "close":
				e.closeAnalyses()
			}
			e.analyses.wait.Wait()
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_composition_steps WHERE composition_id=?`, r.ID).Scan(&count); err != nil || count != 0 {
				t.Fatal("late composition published", count, err)
			}
			if mode == "revoke" {
				if _, err = s.Composition(ctx, "reader", r.ID); !errors.Is(err, ErrAnalysisUnavailable) {
					t.Fatal("revoked report read", err)
				}
				if err = e.CancelComposition("reader", r.ID); err != nil {
					t.Fatal("revoked job cannot be cancelled", err)
				}
			}
			if mode == "review" {
				if err = e.StartComposition("reader", r.ID, &compositionFixtureProvider{}); !errors.Is(err, ErrCompositionReviewsChanged) {
					t.Fatal("stale input retry", err)
				}
			}
		})
	}
}
