package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const linkedEvidenceBytes = 64 << 10
const linkedEvidenceCount = 16

// Expand only the shortlisted documents, one verified immutable graph at a time.
// References stay exact, individually citable spans; headers are never prepended
// to a table value under the value's original byte range.
func (e *Engine) expandEvidence(ctx context.Context, results []SearchResult) ([]SearchResult, error) {
	s, ok := e.store.(*SQLiteStore)
	if !ok {
		return results, nil
	}
	groups := make([][]SearchResult, len(results))
	byRevision := map[string][]int{}
	for i, r := range results {
		groups[i] = []SearchResult{r}
		if r.Chunk != nil && r.Chunk.Source != nil && r.Chunk.Content != "" && r.ExtractionRevision != "" {
			key := r.DocumentID + "/" + r.ExtractionRevision
			byRevision[key] = append(byRevision[key], i)
		}
	}
	// Deterministic processing order also makes the total expansion budget stable.
	if len(byRevision) > 0 {
		// Search already holds the engine read lock. Never wait here for a slot
		// held by a reader waiting behind an engine writer: fail explicitly busy.
		select {
		case structureReaders <- struct{}{}:
			defer func() { <-structureReaders }()
		default:
			return nil, fmt.Errorf("linked evidence readers are busy")
		}
	}
	keys := make([]string, 0, len(byRevision))
	for key := range byRevision {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	budget := 256 << 10
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		indexes := byRevision[key]
		first := results[indexes[0]]
		if err := e.documentAccess(principalFromContext(ctx), first.DocumentID, false); err != nil {
			return nil, err
		}
		graph, err := s.extraction(first.DocumentID, first.ExtractionRevision)
		if err != nil {
			return nil, fmt.Errorf("retained linked evidence unavailable: %w", err)
		}
		var rawRanges string
		if err = s.db.QueryRowContext(ctx, `SELECT source_ranges FROM document_extractions WHERE document_id=? AND revision=? AND LENGTH(CAST(source_ranges AS BLOB))<=?`, first.DocumentID, first.ExtractionRevision, 16<<20).Scan(&rawRanges); err != nil {
			return nil, err
		}
		var ranges []SourceRange
		if err = json.Unmarshal([]byte(rawRanges), &ranges); err != nil {
			return nil, err
		}
		if err = validateSourceRanges(graph.Text, ranges); err != nil {
			return nil, err
		}
		index := indexEvidence(graph, ranges)
		for _, i := range indexes {
			groups[i], err = index.expand(results[i], min(budget, linkedEvidenceBytes))
			if err != nil {
				return nil, err
			}
			for _, r := range groups[i][1:] {
				budget -= len(r.Chunk.Content)
			}
		}
	}
	out := []SearchResult{}
	seen := map[string]bool{}
	for _, group := range groups {
		for _, r := range group {
			if r.Chunk == nil {
				out = append(out, r)
				continue
			}
			key := r.DocumentID + "/" + r.Chunk.ID
			if !seen[key] {
				out = append(out, r)
				seen[key] = true
			}
		}
	}
	// Reading a historical graph must not bypass a concurrent membership change.
	for _, r := range out {
		if err := e.documentAccess(principalFromContext(ctx), r.DocumentID, false); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type evidenceIndex struct {
	graph   *document.Extraction
	links   *document.LinkIndex
	sources map[string]SourceRange
}

func indexEvidence(graph *document.Extraction, ranges []SourceRange) evidenceIndex {
	index := evidenceIndex{graph: graph, links: document.IndexLinks(graph), sources: map[string]SourceRange{}}
	for _, unit := range graph.Units {
		for _, source := range ranges {
			if unit.Kind == source.Kind && unit.Index == source.Index && source.SHA256 == graph.OriginalSHA256 {
				index.sources[unit.ID] = source
			}
		}
	}
	return index
}

func (index evidenceIndex) expand(primary SearchResult, budget int) ([]SearchResult, error) {
	g, c := index.graph, primary.Chunk
	if c.StartChar < 0 || c.EndChar > len(g.Text) || c.EndChar <= c.StartChar || g.Text[c.StartChar:c.EndChar] != c.Content {
		return nil, fmt.Errorf("retrieved chunk does not match retained extraction")
	}
	receipt := &document.EvidenceReceipt{Revision: g.ID, GroupID: c.ID, LinksComplete: true}
	primary.EvidenceReceipt = receipt
	out := []SearchResult{primary}
	links := index.links.ForSpan(document.Span{Start: c.StartChar, End: c.EndChar})
	receipt.BlockIDs, receipt.LimitReached = links.BlockIDs, links.LimitReached
	blocks := links.Required
	covered := []document.Span{{Start: c.StartChar, End: c.EndChar}}
	omit := func(id string) {
		receipt.LimitReached = true
		if len(receipt.OmittedBlocks) < 128 {
			receipt.OmittedBlocks = append(receipt.OmittedBlocks, id)
		}
	}
	for _, b := range blocks {
		contained := false
		for _, span := range covered {
			if span.Start <= b.Span.Start && span.End >= b.Span.End {
				contained = true
				break
			}
		}
		if contained || b.Span.End == b.Span.Start {
			continue
		}
		if len(out) >= linkedEvidenceCount || b.Span.End-b.Span.Start > budget {
			omit(b.ID)
			continue
		}
		source, found := index.sources[b.UnitID]
		if !found || b.Span.Start < source.Start || b.Span.End > source.End {
			omit(b.ID)
			continue
		}
		text := g.Text[b.Span.Start:b.Span.End]
		chunk := &Chunk{ID: "linked-" + GenerateContentHash(g.ID+":"+b.ID), DocumentID: c.DocumentID, Content: text, StartChar: b.Span.Start, EndChar: b.Span.End, ContentHash: GenerateContentHash(text), Section: source.Name, Source: sourceForSpan(source, b.Span.Start, b.Span.End)}
		if source.Kind == "page" {
			chunk.Page = source.Index
		}
		linkedReceipt := *receipt
		linkedReceipt.BlockIDs = []string{b.ID}
		linked := SearchResult{Chunk: chunk, Score: primary.Score, DocumentID: primary.DocumentID, DocName: primary.DocName, Metadata: primary.Metadata, ExtractionRevision: g.ID, EvidenceReceipt: &linkedReceipt}
		linked.Locator = locatorForResult(linked)
		out = append(out, linked)
		covered = append(covered, b.Span)
		budget -= len(text)
	}
	for _, r := range out {
		receipt.RequiredChunks = append(receipt.RequiredChunks, r.Chunk.ID)
	}
	receipt.LinksComplete = !receipt.LimitReached
	for i := range out {
		out[i].EvidenceReceipt.RequiredChunks = receipt.RequiredChunks
		out[i].EvidenceReceipt.OmittedBlocks = receipt.OmittedBlocks
		out[i].EvidenceReceipt.LimitReached = receipt.LimitReached
		out[i].EvidenceReceipt.LinksComplete = receipt.LinksComplete
	}
	return out, nil
}

// Recompute after every context reduction. An earlier true receipt must not
// survive when a linked exception/header has been dropped for the model budget.
func (rc *RAGContext) refreshEvidenceReceipts() {
	present := map[string]bool{}
	for _, r := range rc.Results {
		if r.Chunk != nil && !r.Chunk.Truncated {
			present[r.DocumentID+"/"+r.Chunk.ID] = true
		}
	}
	for i, r := range rc.Results {
		if r.EvidenceReceipt == nil {
			continue
		}
		receipt := *r.EvidenceReceipt
		receipt.LinksComplete = !receipt.LimitReached
		for _, id := range receipt.RequiredChunks {
			if !present[r.DocumentID+"/"+id] {
				receipt.LinksComplete = false
			}
		}
		rc.Results[i].EvidenceReceipt = &receipt
	}
}

// Full block identities belong in the saved receipt, not repeated in the small
// model context. Connect exact passages through the citation numbers already
// emitted by FormatContext, preserving space for their actual evidence text.
func (rc *RAGContext) linkedSourceNumbers(result SearchResult) string {
	required := map[string]bool{}
	for _, id := range result.EvidenceReceipt.RequiredChunks {
		required[id] = true
	}
	numbers := []string{}
	for i, r := range rc.Results {
		if r.DocumentID == result.DocumentID && r.Chunk != nil && required[r.Chunk.ID] {
			numbers = append(numbers, "["+strconv.Itoa(i+1)+"]")
		}
	}
	return "\nLinked structural evidence: " + strings.Join(numbers, ", ") + ". This is not a complete-document review."
}
