package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSourceRangesOfficeAndTextSurviveSQLiteAndReindex(t *testing.T) {
	for _, fixture := range readParserCorpus(t) {
		if fixture.ExpectedError != "" {
			continue
		}
		name := fixture.Path
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "documents", "corpus", name))
			if err != nil {
				t.Fatal(err)
			}
			parser := NewDocumentParser()
			if filepath.Ext(name) == ".pdf" {
				parser.pdfCacheDir = t.TempDir()
				parser.pdfWorker = os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER")
				if parser.pdfWorker == "" {
					t.Skip("requires actual PDF worker")
				}
			}
			parsed, err := parser.Parse(data, name, filepath.Ext(name))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.ExpectedText {
				if !strings.Contains(strings.Join(strings.Fields(parsed.Content), " "), strings.Join(strings.Fields(expected), " ")) {
					t.Fatal("source lost expected fixture text", expected)
				}
			}
			content, ranges, err := parsedSourceRanges(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Ext(name) == ".xlsx" && (len(ranges) == 0 || len(ranges[0].Cells) == 0) {
				t.Fatal("real workbook has no cell locators")
			}
			doc := &Document{ID: GenerateDocumentID([]byte(content)), Name: name, RawContent: content, SourceRanges: ranges, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			chunker := NewChunker(ChunkingOptions{ChunkSize: 17, ChunkOverlap: 5})
			chunks, err := chunker.chunkDocument(doc)
			if err != nil || len(chunks) == 0 {
				t.Fatal(err)
			}
			vectors := make([][]float32, len(chunks))
			for i, c := range chunks {
				if !utf8.ValidString(c.Content) || doc.RawContent[c.StartChar:c.EndChar] != c.Content || c.Source == nil || c.StartChar < c.Source.Start || c.EndChar > c.Source.End {
					t.Fatalf("invalid exact source: %+v", c)
				}
				vectors[i] = []float32{1, 0}
				c.CreatedAt = time.Now()
			}
			dir := t.TempDir()
			store, err := NewSQLiteStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.AddDocumentWithChunks(doc, chunks, vectors); err != nil {
				t.Fatal(err)
			}
			store.Close()
			store, err = NewSQLiteStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			saved, err := store.GetDocument(doc.ID)
			if err != nil || !reflect.DeepEqual(saved.SourceRanges, ranges) {
				t.Fatal("lost source units", err)
			}
			rebuilt, err := chunker.chunkDocument(saved)
			if err != nil {
				t.Fatal(err)
			}
			for i, c := range rebuilt {
				if c.Content != chunks[i].Content || c.StartChar != chunks[i].StartChar || !reflect.DeepEqual(c.Source, chunks[i].Source) {
					t.Fatal("reindex changed source locator")
				}
				c.CreatedAt = time.Now()
			}
			if err = store.ReplaceDocumentWithChunks(saved, rebuilt, vectors); err != nil {
				t.Fatal(err)
			}
			results, err := store.Search([]float32{1, 0}, len(chunks), 0)
			if err != nil || len(results) != len(chunks) {
				t.Fatal(err, len(results))
			}
			for _, r := range results {
				loc := locatorForResult(r)
				if loc.Source == nil || saved.RawContent[loc.StartChar:loc.EndChar] != r.Chunk.Content || loc.Source.SHA256 != ranges[0].SHA256 {
					t.Fatal("citation lost fidelity", loc)
				}
				passage, err := saved.ReadSourcePassage(loc.StartChar, loc.EndChar, loc.Source.SHA256, loc.Source.Revision, loc.ContentHash)
				if err != nil || passage != r.Chunk.Content || loc.Source.SHA256 != fixture.SHA256 {
					t.Fatal("persisted citation failed exact readback", err)
				}
				if loc.Source.Kind == "page" && loc.Page != loc.Source.Index {
					t.Fatal("wrong cited page", loc)
				}
			}
			assertCorpusSourceIdentity(t, fixture, ranges)
			t.Logf("%s: %d exact chunk citations survived restart and reindex", name, len(results))
		})
	}
}

func assertCorpusSourceIdentity(t *testing.T, fixture parserCorpusCase, ranges []SourceRange) {
	t.Helper()
	var got []string
	for _, source := range ranges {
		if source.Kind == "sheet" {
			for _, cell := range source.Cells {
				got = append(got, "sheet:"+source.Name+"!"+cell.Reference)
			}
		} else {
			got = append(got, fmt.Sprintf("%s:%d", source.Kind, source.Index))
		}
	}
	want := fixture.SourceUnits
	if fixture.Path == "ordered-workbook.xlsx" {
		// The original corpus lists anchor cells, not the complete inventory.
		// These additional cells are present in the frozen worksheet XML.
		want = append(append([]string(nil), want...), "sheet:Budget café!D4", "sheet:Budget café!B6", "sheet:Budget café!B7")
	}
	if filepath.Ext(fixture.Path) == ".docx" {
		// The frozen fixture describes paragraphs; DOCX deliberately provides
		// a body section, never fabricated rendered page/paragraph locators.
		want = []string{"section:1"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong source identity: got %v want %v", got, want)
	}
}

func TestSourceRangesRepeatedPagesAndInvalidReceipts(t *testing.T) {
	hash := strings.Repeat("a", 64)
	parsed := &ParseResult{State: ExtractionComplete, Content: "same 日本語\nsame 日本語", SourceUnits: []SourceUnit{{Kind: "page", Index: 1, Text: "same 日本語", State: "ready", SourceSHA256: hash, ExtractorRevision: "fixture"}, {Kind: "page", Index: 2, Text: "same 日本語", State: "ready", SourceSHA256: hash, ExtractorRevision: "fixture"}}}
	content, ranges, err := parsedSourceRanges(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if ranges[1].Start <= ranges[0].End {
		t.Fatal("repeated page incorrectly mapped to first page")
	}
	doc := &Document{ID: "pages", RawContent: content, SourceRanges: ranges}
	chunks, err := NewChunker(ChunkingOptions{ChunkSize: 1, ChunkOverlap: 0}).chunkDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chunks {
		if !utf8.ValidString(c.Content) || c.Page != c.Source.Index {
			t.Fatal(c)
		}
	}
	parsed.State = ExtractionPartial
	if _, _, err := parsedSourceRanges(parsed); err == nil {
		t.Fatal("partial extraction silently indexed")
	}
	parsed.State = ExtractionComplete
	parsed.SourceUnits[1].Text = "not present"
	if _, _, err := parsedSourceRanges(parsed); err == nil {
		t.Fatal("fabricated source accepted")
	}
	doc.SourceRanges[0].End = len(content) + 1
	if _, err := NewChunker(DefaultChunkingOptions()).chunkDocument(doc); err == nil {
		t.Fatal("corrupt retained source accepted")
	}
}

func TestSourceRangeTruncatedExcerptStillReadsBackExactly(t *testing.T) {
	text := strings.Repeat("日本語 café ", 200)
	unit := SourceRange{Kind: "page", Index: 1, Start: 0, End: len(text), SHA256: strings.Repeat("a", 64), Revision: "fixture"}
	original := &Chunk{ID: "chunk", Content: text, StartChar: 0, EndChar: len(text), Source: &unit}
	rc := &RAGContext{Results: []SearchResult{{DocumentID: "doc", DocName: "source.pdf", Chunk: original}}}
	rc.FormatContext()
	rc.TruncateContext(600)
	c := rc.Results[0].Chunk
	if !c.Truncated || text[c.StartChar:c.EndChar] != c.Content || original.Content != text || c.ContentHash != GenerateContentHash(c.Content) {
		t.Fatal("truncated citation does not match real source")
	}
}
