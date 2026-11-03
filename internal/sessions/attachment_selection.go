package sessions

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Offsets identify exact UTF-8 bytes in the immutable extracted-text revision,
// not invented PDF page numbers. Originals and full extraction stay untouched.
type AttachmentExcerpt struct {
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Text      string `json:"text"`
}

type AttachmentSelection struct {
	ID         string   `json:"id"`
	Revision   string   `json:"revision"`
	Name       string   `json:"name"`
	Mode       string   `json:"mode"`
	TotalBytes int      `json:"total_bytes"`
	Ranges     [][2]int `json:"ranges"`
}

func AttachmentSelections(sources []AttachmentText) []AttachmentSelection {
	var result []AttachmentSelection
	for _, source := range sources {
		selection := AttachmentSelection{ID: source.Attachment.ID, Revision: source.Attachment.Revision, Name: source.Attachment.Name, Mode: "full_text", TotalBytes: len(source.Text), Ranges: [][2]int{{0, len(source.Text)}}}
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
	terms := map[string]int{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if utf8.RuneCountInString(word) > 2 {
			terms[word]++
		}
	}
	return terms
}

func documentChunks(text string) []AttachmentExcerpt {
	var chunks []AttachmentExcerpt
	for start := 0; start < len(text); {
		end := min(start+1200, len(text))
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
		chunks = append(chunks, AttachmentExcerpt{start, end, text[start:end]})
		start = end
	}
	return chunks
}

// SelectAttachmentText uses local lexical retrieval, with opening and distributed
// passages for orientation. It is an explicitly limited view, never a summary or
// a claim of full-document comprehension. Every source receives a budget.
func SelectAttachmentText(sources []AttachmentText, query string, budget int) ([]AttachmentText, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	full, _ := json.Marshal(sources)
	if len(full) <= budget {
		return sources, nil
	}
	if budget < len(sources)*600 {
		return nil, ErrAttachmentContextLimit
	}
	selected := make([]AttachmentText, len(sources))
	terms := documentTerms(query)
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
		chunks := documentChunks(source.Text)
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
		// Opening content anchors title/abstract questions. Query matches then
		// compete with evenly spaced coverage, without changing source text.
		candidates := []int{0}
		for j, index := range order {
			candidates = append(candidates, index)
			if j < 4 {
				candidates = append(candidates, (j+1)*(len(chunks)-1)/4)
			}
		}
		out := AttachmentText{Attachment: source.Attachment, Selection: "excerpts", TotalBytes: len(source.Text)}
		used := map[int]bool{}
		for _, index := range candidates {
			if len(chunks) == 0 || used[index] {
				continue
			}
			used[index] = true
			candidate := chunks[index]
			probe := out
			probe.Excerpts = append(append([]AttachmentExcerpt(nil), out.Excerpts...), candidate)
			encoded, _ := json.Marshal(probe)
			if len(encoded) > share {
				continue
			}
			out = probe
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
