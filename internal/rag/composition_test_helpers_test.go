package rag

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func completedCompositionParent(t *testing.T, s *SQLiteStore, owner string) *SynthesisRecord {
	t.Helper()
	refs := synthesisTestDocument(t, s, strings.Repeat("Submit the signed form by 31 July. Exception: approved extensions remain valid.\n", 60))
	r, err := s.CreateSynthesis(owner, "parent", "summary", refs, synthesisTestIdentity(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	r, lease, err := s.beginSynthesis(owner, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := s.synthesisTraversal(owner, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	for r.State == "running" {
		batch, err := tr.Batch(context.Background(), r.UnitCursor, r.NextByte, r.PrimaryBytes)
		if err != nil {
			t.Fatal(err)
		}
		ex, err := tr.ExecuteSynthesisBatch(context.Background(), batch, r.Model, &synthesisFixtureProvider{}, func() error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if err = s.checkpointSynthesis(lease, r.CompletedBatches, tr, batch, ex); err != nil {
			t.Fatal(err)
		}
		r, err = s.Synthesis(owner, r.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	return r
}

type compositionFixtureProvider struct {
	calls  atomic.Int32
	failAt atomic.Int32
	hook   func(context.Context) error
}

func (p *compositionFixtureProvider) WithModel(ctx context.Context, i document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	return run(&compositionFixtureSession{p: p, i: i})
}

type compositionFixtureSession struct {
	p *compositionFixtureProvider
	i document.SynthesisModelIdentity
}

func (s *compositionFixtureSession) Identity() document.SynthesisModelIdentity { return s.i }
func (s *compositionFixtureSession) Count(context.Context, document.SynthesisPrompt) (int, error) {
	return 1700, nil
}
func (s *compositionFixtureSession) Generate(ctx context.Context, prompt document.SynthesisPrompt, _ int) ([]byte, string, error) {
	if s.p.calls.Add(1) == s.p.failAt.Load() {
		return nil, "", errors.New("PRIVATE fixture failure")
	}
	if s.p.hook != nil {
		if err := s.p.hook(ctx); err != nil {
			return nil, "", err
		}
	}
	var input struct {
		TaskID string `json:"task_id"`
		Inputs []struct {
			ID string `json:"id"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(prompt.User), &input); err != nil {
		return nil, "", err
	}
	refs := []string{}
	for _, i := range input.Inputs {
		refs = append(refs, i.ID)
	}
	raw, err := json.Marshal(map[string]any{"task_id": input.TaskID, "sections": []document.CompositionSection{{Text: "Submit the signed form by 31 July; approved extensions remain valid.", Inputs: refs}}, "omitted": []string{}})
	return raw, "stop", err
}

func waitComposition(t *testing.T, s *SQLiteStore, owner, id, state string) *CompositionRecord {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.Composition(context.Background(), owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.State == state {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, _ := s.Composition(context.Background(), owner, id)
	t.Fatalf("composition did not reach %s: %+v", state, r)
	return nil
}
