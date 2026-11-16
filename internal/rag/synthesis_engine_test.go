package rag

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSynthesisEngineResumesCommittedBatches(t *testing.T) {
	e := NewEngine(nil, t.TempDir())
	defer e.Close()
	s, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateSynthesis("owner", "resume", "summary", synthesisTestDocument(t, s, strings.Repeat("Policy 17.25. 例外：七月。\n", 90)), synthesisTestIdentity(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	p := &synthesisFixtureProvider{}
	p.failAt.Store(2)
	if err = e.StartSynthesis("owner", r.ID, p); err != nil {
		t.Fatal(err)
	}
	r = waitSynthesis(t, s, r.ID, "interrupted")
	if r.CompletedBatches != 1 || r.NextByte == 0 || r.ErrorCode != "analysis_failed" || p.calls.Load() != 2 {
		t.Fatal(r, p.calls.Load())
	}
	// Failure returns control; there is no unattended replay of the model call.
	time.Sleep(30 * time.Millisecond)
	if p.calls.Load() != 2 {
		t.Fatal("automatic replay")
	}
	p.failAt.Store(0)
	if err = e.StartSynthesis("owner", r.ID, p); err != nil {
		t.Fatal(err)
	}
	r = waitSynthesis(t, s, r.ID, "awaiting_review")
	if int(p.calls.Load()) != r.CompletedBatches+1 || r.Attempt != 2 || r.CompletedUnits != len(r.Plan.Units) {
		t.Fatal(r, p.calls.Load())
	}
	page, err := s.SynthesisResults("owner", r.ID, 0, 8)
	if err != nil || len(page.Receipts) == 0 {
		t.Fatal(page, err)
	}
}

func TestSynthesisEngineCancellationAndSourceRevocation(t *testing.T) {
	for _, mode := range []string{"cancel", "revoke", "close"} {
		t.Run(mode, func(t *testing.T) {
			e := NewEngine(nil, t.TempDir())
			defer e.Close()
			s, err := e.ImportStore()
			if err != nil {
				t.Fatal(err)
			}
			refs := synthesisTestDocument(t, s, "Policy 17.25.")
			policy, err := s.CreateCollection("owner", "Private", []string{"reader"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.AssignDocumentCollection("owner", "source", policy.ID, 1); err != nil {
				t.Fatal(err)
			}
			r, err := s.CreateSynthesis("reader", "stop", "requirements", refs, synthesisTestIdentity(), 1024)
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			p := &synthesisFixtureProvider{hook: func(ctx context.Context) error {
				close(entered)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}
			if err = e.StartSynthesis("reader", r.ID, p); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("worker did not start")
			}
			switch mode {
			case "cancel":
				if err = e.CancelAnalysis("reader", r.ID); err != nil {
					t.Fatal(err)
				}
			case "revoke":
				if _, err = s.UpdateCollection("owner", policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
					t.Fatal(err)
				}
				close(release)
			case "close":
				e.closeAnalyses()
			}
			e.analyses.wait.Wait()
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM document_analysis_steps WHERE analysis_id=?`, r.ID).Scan(&count); err != nil || count != 0 {
				t.Fatal("late result published", count, err)
			}
			if mode == "revoke" {
				if _, err = s.SynthesisResults("reader", r.ID, 0, 1); !errors.Is(err, ErrAnalysisUnavailable) {
					t.Fatal("revoked read", err)
				}
				if err = e.CancelAnalysis("reader", r.ID); err != nil {
					t.Fatal("cannot cancel after revocation", err)
				}
			}
		})
	}
}
