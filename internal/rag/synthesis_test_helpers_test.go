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

func synthesisTestIdentity() document.SynthesisModelIdentity {
	return document.SynthesisModelIdentity{ModelID: "installed-fixture", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "test-runtime", Revision: "1"}, Language: "zh-Hans", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}
}

func synthesisTestDocument(t *testing.T, s *SQLiteStore, text string) []document.SourceRef {
	t.Helper()
	parsed, err := NewDocumentParser().Parse([]byte(text), "policy.txt", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	content, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: "source", Name: "Policy", RawContent: content, Metadata: parsed.Metadata, SourceRanges: ranges, Extraction: parsed.Structure, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = s.AddDocumentWithChunks(d, nil, nil); err != nil {
		t.Fatal(err)
	}
	g, err := s.extraction(d.ID, d.ExtractionRevision)
	if err != nil {
		t.Fatal(err)
	}
	return []document.SourceRef{{DocumentID: d.ID, Revision: g.ID, OriginalSHA256: g.OriginalSHA256}}
}

type synthesisFixtureProvider struct {
	calls  atomic.Int32
	failAt atomic.Int32
	hook   func(context.Context) error
}

func (p *synthesisFixtureProvider) WithModel(ctx context.Context, i document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	return run(&synthesisFixtureSession{p: p, i: i})
}

type synthesisFixtureSession struct {
	p *synthesisFixtureProvider
	i document.SynthesisModelIdentity
}

func (s *synthesisFixtureSession) Identity() document.SynthesisModelIdentity { return s.i }
func (s *synthesisFixtureSession) Count(context.Context, document.SynthesisPrompt) (int, error) {
	return 2048, nil
}
func (s *synthesisFixtureSession) Generate(ctx context.Context, prompt document.SynthesisPrompt, _ int) ([]byte, string, error) {
	n := s.p.calls.Add(1)
	if n == s.p.failAt.Load() {
		return nil, "", errors.New("PRIVATE raw error text")
	}
	if s.p.hook != nil {
		if err := s.p.hook(ctx); err != nil {
			return nil, "", err
		}
	}
	var payload struct {
		Batch document.SynthesisBatch `json:"batch"`
	}
	if err := json.Unmarshal([]byte(prompt.User), &payload); err != nil {
		return nil, "", err
	}
	b := payload.Batch
	draft := document.SynthesisDraft{BatchID: b.ID, Items: []document.SynthesisItem{}, ReviewedEvidence: []string{}}
	for _, e := range b.Evidence {
		draft.ReviewedEvidence = append(draft.ReviewedEvidence, e.ID)
	}
	// Full batch quote is unique even in a repetitive fixture and has exact bytes.
	if len(b.Evidence[0].Text) <= 4096 {
		draft.Items = append(draft.Items, document.SynthesisItem{Text: b.Evidence[0].Text, Evidence: []document.SynthesisQuote{{EvidenceID: "e0", Quote: b.Evidence[0].Text}}, Conditions: []document.SynthesisQuote{}, Exceptions: []document.SynthesisQuote{}})
	}
	raw, err := json.Marshal(draft)
	return raw, "stop", err
}

func synthesisTestBatch(t *testing.T, s *SQLiteStore, r *SynthesisRecord) (*document.SynthesisTraversal, *document.SynthesisBatch, *document.SynthesisExecution) {
	t.Helper()
	tr, err := s.synthesisTraversal("owner", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := tr.Batch(context.Background(), r.UnitCursor, r.NextByte, r.PrimaryBytes)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := tr.ExecuteSynthesisBatch(context.Background(), b, r.Model, &synthesisFixtureProvider{}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return tr, b, ex
}

func waitSynthesis(t *testing.T, s *SQLiteStore, id, state string) *SynthesisRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.Synthesis("owner", id)
		if err != nil {
			t.Fatal(err)
		}
		if r.State == state {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, _ := s.Synthesis("owner", id)
	t.Fatalf("did not reach %s: %+v", state, r)
	return nil
}
