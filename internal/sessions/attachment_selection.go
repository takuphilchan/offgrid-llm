package sessions

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/textsearch"
)

// Offsets identify exact UTF-8 bytes in the immutable extracted-text revision,
// not invented PDF page numbers. Originals and full extraction stay untouched.
type AttachmentExcerpt struct {
	Citation  string `json:"citation,omitempty"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text"`
}

type AttachmentSelection struct {
	ExtractionRevision string                     `json:"extraction_revision,omitempty"`
	LinkedEvidence     *document.SelectionReceipt `json:"linked_evidence,omitempty"`
	ID                 string                     `json:"id"`
	Revision           string                     `json:"revision"`
	Name               string                     `json:"name"`
	Mode               string                     `json:"mode"`
	TotalBytes         int                        `json:"total_bytes"`
	Ranges             [][2]int                   `json:"ranges"`
}

func AttachmentSelections(sources []AttachmentText) []AttachmentSelection {
	var result []AttachmentSelection
	for _, source := range sources {
		selection := AttachmentSelection{ID: source.Attachment.ID, Revision: source.Attachment.Revision, Name: source.Attachment.Name, Mode: "full_text", TotalBytes: len(source.Text), Ranges: [][2]int{{0, len(source.Text)}}}
		selection.LinkedEvidence = source.LinkedEvidence
		selection.ExtractionRevision = source.Attachment.ExtractionRevision
		if source.Selection == "excerpts" {
			selection.Mode = "excerpts"
			selection.TotalBytes = source.TotalBytes
			selection.Ranges = nil
			for _, excerpt := range source.Excerpts {
				selection.Ranges = append(selection.Ranges, [2]int{excerpt.StartByte, excerpt.EndByte})
			}
		}
		result = append(result, selection)
	}
	return result
}

func documentTerms(text string) map[string]int {
	return textsearch.Frequencies(text)
}

func documentChunks(text string) []AttachmentExcerpt {
	return attachmentChunks(text, 1200)
}

func attachmentChunks(text string, size int) []AttachmentExcerpt {
	var chunks []AttachmentExcerpt
	for start := 0; start < len(text); {
		end := min(start+max(4, size), len(text))
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
		if end < len(text) {
			middle := start + (end-start)/2
			if split := strings.LastIndexAny(text[middle:end], "\n.!?。！？"); split >= 0 {
				end = middle + split
				_, size := utf8.DecodeRuneInString(text[end:])
				end += size
			}
		}
		chunks = append(chunks, AttachmentExcerpt{StartByte: start, EndByte: end, Text: text[start:end]})
		start = end
	}
	return chunks
}

// SelectAttachmentText prioritizes lexical evidence, then uses spare room for
// orientation. It is an explicitly limited view, never a summary or
// a claim of full-document comprehension. Every source receives a budget.
func SelectAttachmentText(sources []AttachmentText, query string, budget int) ([]AttachmentText, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	sources = append([]AttachmentText(nil), sources...)
	for i := range sources {
		if graph := sources[i].Structure; graph != nil && len(graph.Relationships) > 0 {
			sources[i].LinkedEvidence = &document.SelectionReceipt{Revision: graph.ID, LinksComplete: true}
		}
	}
	full, _ := json.Marshal(sources)
	if len(full) <= budget {
		return sources, nil
	}
	if budget < len(sources)*600 {
		return nil, ErrAttachmentContextLimit
	}
	selected := make([]AttachmentText, len(sources))
	terms := map[string]int{}
	for _, term := range textsearch.QueryTerms(query) {
		terms[term] = 1
	}
	// Bound ranking work without changing the question sent to the model.
	keys := make([]string, 0, len(terms))
	for term := range terms {
		keys = append(keys, term)
	}
	sort.Strings(keys)
	for _, term := range keys[min(64, len(keys)):] {
		delete(terms, term)
	}
	for i, source := range sources {
		share := (budget-2)/len(sources) - 1
		encoded, _ := json.Marshal(source)
		if len(encoded) <= share {
			selected[i] = source
			continue
		}
		out := AttachmentText{Attachment: source.Attachment, Structure: source.Structure, Selection: "excerpts", TotalBytes: len(source.Text)}
		var links *document.LinkIndex
		if source.LinkedEvidence != nil {
			links = document.IndexLinks(source.Structure)
			out.LinkedEvidence = attachmentLinkReceipt(out, links)
		}
		metadata, _ := json.Marshal(out)
		// Metadata and escaped text share the same budget. Fixed-size chunks
		// must not force a relevant passage out in favour of a short tail.
		chunks := attachmentChunks(source.Text, min(1200, max(64, (share-len(metadata)-256)/2)))
		frequencies := make([]map[string]int, len(chunks))
		df := map[string]int{}
		for j, c := range chunks {
			words := documentTerms(c.Text)
			frequencies[j] = make(map[string]int, len(terms))
			for term := range terms {
				frequencies[j][term] = words[term]
				if frequencies[j][term] > 0 {
					df[term]++
				}
			}
		}
		order := make([]int, len(chunks))
		scores := make([]float64, len(chunks))
		for j := range chunks {
			order[j] = j
			for term := range terms {
				if n := frequencies[j][term]; n > 0 {
					scores[j] += math.Log(1+float64(len(chunks))/float64(1+df[term])) * float64(min(n, 3))
				}
			}
		}
		sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
		// A small model may fit only one excerpt. Never spend that budget on
		// the document opening before its matching evidence.
		var candidates []int
		for _, index := range order {
			if scores[index] > 0 {
				candidates = append(candidates, index)
				if len(candidates) == 20 {
					break
				}
			}
		}
		candidates = append(candidates, 0)
		for j := 1; j <= 4; j++ {
			candidates = append(candidates, j*(len(chunks)-1)/4)
		}
		candidates = append(candidates, order[:min(20, len(order))]...)
		used := map[int]bool{}
		for _, index := range candidates {
			if len(out.Excerpts) >= 64 {
				break
			}
			if len(chunks) == 0 || used[index] {
				continue
			}
			used[index] = true
			if len(out.Excerpts) == 0 && scores[index] == 0 && len(order) > 0 && scores[order[0]] > 0 {
				// Do not substitute an unrelated opening/tail when the matching
				// evidence and its disclosure cannot fit this model's context.
				return nil, ErrAttachmentContextLimit
			}
			candidate := chunks[index]
			probe := out
			probe.Excerpts = append(append([]AttachmentExcerpt(nil), out.Excerpts...), candidate)
			probe.LinkedEvidence = attachmentLinkReceipt(probe, links)
			encoded, _ := json.Marshal(probe)
			if len(encoded) > share {
				continue
			}
			out = probe
			out = includeAttachmentLinks(out, source.Text, candidate, links, share)
		}
		if len(out.Excerpts) == 0 {
			return nil, ErrAttachmentContextLimit
		}
		sort.Slice(out.Excerpts, func(a, b int) bool { return out.Excerpts[a].StartByte < out.Excerpts[b].StartByte })
		selected[i] = out
	}
	encoded, _ := json.Marshal(selected)
	if len(encoded) > budget {
		return nil, ErrAttachmentContextLimit
	}
	return selected, nil
}
