package rag

import (
	"context"
	"strings"
	"testing"
	"time"
)

func linkedFixture(t *testing.T) (*Engine, *Document) {
	t.Helper()
	header := `<w:tr><w:trPr><w:tblHeader/></w:trPr><w:tc><w:p><w:r><w:t>Category</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Annual cap (USD)</w:t></w:r></w:p></w:tc></w:tr>`
	rows := ""
	for i := 0; i < 30; i++ {
		rows += `<w:tr><w:tc><w:p><w:r><w:t>Other category</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>100</w:t></w:r></w:p></w:tc></w:tr>`
	}
	parts := map[string]string{
		"word/document.xml":  docxTestStart + `<w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>Funding policy</w:t></w:r></w:p><w:p><w:r><w:t>Applications close June 30.</w:t><w:footnoteReference w:id="1"/></w:r></w:p><w:tbl>` + header + rows + `<w:tr><w:tc><w:p><w:r><w:t>Rural clinics</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>17.25</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` + docxTestEnd,
		"word/footnotes.xml": `<w:footnotes xmlns:w="` + wordNamespace + `"><w:footnote w:id="1"><w:p><w:r><w:t>` + strings.Repeat("Administrative background. ", 70) + `Exception: rural applications close July 31.</w:t></w:r></w:p></w:footnote></w:footnotes>`,
	}
	r, err := NewDocumentParser().Parse(parserZip(t, parts), "policy.docx", ".docx")
	if err != nil {
		t.Fatal(err)
	}
	text, ranges, err := parsedSourceRanges(r)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine(nil, t.TempDir())
	t.Cleanup(func() { e.Close() })
	e.enabled = true
	e.embeddingModel = "fixture"
	e.embeddingIdentity = "fixture-sha"
	e.reranking = false
	d := &Document{ID: "policy", Name: "Funding policy", RawContent: text, SourceRanges: ranges, Metadata: r.Metadata, Extraction: r.Structure, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	chunks, err := e.chunker.chunkDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	vectors := make([][]float32, len(chunks))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	identity := IndexMetadata{SchemaVersion: IndexSchemaVersion, EmbeddingModel: e.embeddingModel, EmbeddingIdentity: e.embeddingIdentity, EmbeddingDim: 2, ChunkerVersion: chunkerVersion, ParserVersion: parserVersion, UpdatedAt: time.Now()}
	if err = e.store.(*SQLiteStore).PublishDocument(d, chunks, vectors, identity, false); err != nil {
		t.Fatal(err)
	}
	return e, d
}

func selectedEvidence(t *testing.T, e *Engine, d *Document, needle string) SearchResult {
	t.Helper()
	start := strings.Index(d.RawContent, needle)
	if start < 0 {
		t.Fatal("fixture lacks needle")
	}
	c := &Chunk{ID: "selected-" + GenerateContentHash(needle), DocumentID: d.ID, Content: needle, StartChar: start, EndChar: start + len(needle), ContentHash: GenerateContentHash(needle)}
	bindChunkSource(c, d.SourceRanges)
	return SearchResult{DocumentID: d.ID, DocName: d.Name, Chunk: c, ExtractionRevision: d.ExtractionRevision, Score: 1}
}

func TestLinkedEvidenceRetainsDistantHeaderAndClauseException(t *testing.T) {
	e, d := linkedFixture(t)
	for _, tc := range []struct{ needle, related string }{{"17.25", "Annual cap (USD)"}, {"Applications close June 30.", "Exception: rural applications close July 31."}, {"Exception: rural applications close July 31.", "Applications close June 30."}} {
		t.Run(tc.needle, func(t *testing.T) {
			results, err := e.expandEvidence(context.Background(), []SearchResult{selectedEvidence(t, e, d, tc.needle)})
			if err != nil {
				t.Fatal(err)
			}
			ctx := &RAGContext{Results: results}
			ctx.FormatContext()
			if !strings.Contains(ctx.Context, tc.related) || !strings.Contains(ctx.Context, "Funding policy") {
				t.Fatal("linked evidence missing", ctx.Context)
			}
			for _, r := range ctx.Results {
				if r.EvidenceReceipt == nil || !r.EvidenceReceipt.LinksComplete {
					t.Fatal("incomplete receipt", r.EvidenceReceipt)
				}
				if strings.HasPrefix(r.Chunk.ID, "linked-") {
					if len(r.EvidenceReceipt.BlockIDs) != 1 {
						t.Fatal("linked excerpt has wrong block references")
					}
					found := false
					for _, b := range d.Extraction.Blocks {
						if b.ID == r.EvidenceReceipt.BlockIDs[0] {
							found = true
							if d.RawContent[b.Span.Start:b.Span.End] != r.Chunk.Content {
								t.Fatal("linked block reference does not match text")
							}
						}
					}
					if !found {
						t.Fatal("linked block reference unavailable")
					}
				}
				c := r.Chunk
				_, text, err := e.ForActor("local").ReadSourcePassage(d.ID, c.StartChar, c.EndChar, c.Source.SHA256, c.Source.Revision, c.ContentHash)
				if err != nil || text != c.Content {
					t.Fatal("linked citation not exact", err)
				}
			}
		})
	}
}

func TestLinkedEvidenceSearchPathAndTruncationReceipt(t *testing.T) {
	e, _ := linkedFixture(t)
	result, err := e.search(context.Background(), "17.25", SearchOptions{TopK: 1, IncludeContent: true}, func(context.Context, []string) ([][]float32, error) { return [][]float32{{0, 1}}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Context, "Annual cap (USD)") || len(result.Results) < 2 {
		t.Fatal("search skipped linked expansion", result.Context)
	}
	trimmed := *result
	trimmed.Results = append([]SearchResult(nil), result.Results...)
	trimmed.TruncateContext(300)
	if len(trimmed.Results) == 0 || trimmed.Results[0].EvidenceReceipt.LinksComplete || !strings.Contains(trimmed.Context, "Linked evidence is missing") {
		t.Fatal("model-budget reduction hid missing links")
	}
	result.Results = result.Results[:1]
	result.FormatContext()
	if result.Results[0].EvidenceReceipt.LinksComplete || !strings.Contains(result.Context, "Linked evidence is missing") {
		t.Fatal("removed header still labelled complete")
	}
}

func TestLinkedEvidenceAdmissionDoesNotWaitUnderEngineLock(t *testing.T) {
	e, d := linkedFixture(t)
	for i := 0; i < cap(structureReaders); i++ {
		structureReaders <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(structureReaders); i++ {
			<-structureReaders
		}
	}()
	started := time.Now()
	if _, err := e.expandEvidence(context.Background(), []SearchResult{selectedEvidence(t, e, d, "17.25")}); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatal("missing bounded reader admission", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("retrieval waited while holding engine lock")
	}
}

func TestLinkedEvidenceBoundsAndRevocation(t *testing.T) {
	e, d := linkedFixture(t)
	primary := selectedEvidence(t, e, d, "Applications close June 30.")
	index := indexEvidence(d.Extraction, d.SourceRanges)
	results, err := index.expand(primary, 16)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &RAGContext{Results: results}
	ctx.FormatContext()
	used := 0
	for _, r := range ctx.Results[1:] {
		used += len(r.Chunk.Content)
	}
	if used > 16 || ctx.Results[0].EvidenceReceipt.LinksComplete || !ctx.Results[0].EvidenceReceipt.LimitReached || len(ctx.Results[0].EvidenceReceipt.OmittedBlocks) == 0 {
		t.Fatal("bounded expansion silently dropped exception")
	}
	policy, err := e.store.(*SQLiteStore).CreateCollection("owner", "Private", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.(*SQLiteStore).AssignDocumentCollection("owner", d.ID, policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = e.expandEvidence(e.ForActor("reader").context(context.Background()), []SearchResult{primary}); err == nil {
		t.Fatal("expansion bypassed revoked access")
	}
}

func TestDOCXHeadersAreExplicitAndNestedTablesStaySeparate(t *testing.T) {
	for _, flag := range []string{`<w:tblHeader/>`, `<w:tblHeader w:val="false"/>`, ``} {
		body := docxTestStart + `<w:tbl><w:tr><w:trPr>` + flag + `</w:trPr><w:tc><w:p><w:r><w:t>Header</w:t></w:r></w:p></w:tc></w:tr><w:tr><w:tc><w:p><w:r><w:t>Outer value</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>Nested value</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:tc></w:tr></w:tbl>` + docxTestEnd
		r, err := NewDocumentParser().Parse(parserZip(t, map[string]string{"word/document.xml": body}), "table.docx", ".docx")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, rel := range r.Structure.Relationships {
			if rel.Kind == "header_for" {
				count++
			}
		}
		want := 0
		if flag == `<w:tblHeader/>` {
			want = 1
		}
		if count != want {
			t.Fatal("invented or cross-table header association", flag, count)
		}
	}
}
