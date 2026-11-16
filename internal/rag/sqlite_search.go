package rag

import (
	"container/heap"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/takuphilchan/offgrid-llm/internal/textsearch"
)

const retrievalChannelCandidates = 20

// Score one vector at a time; retain at most twenty candidates per channel.
// The SQLite cursor streams its rows. Source bodies and document metadata are
// read only after fusion. Callers hold s.mu; selection is applied before ranking.
func (s *SQLiteStore) boundedSearch(queryEmbedding []float32, query string, opts SearchOptions, alpha float32, lexical bool) ([]SearchResult, error) {
	for _, value := range queryEmbedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("invalid query embedding")
		}
	}
	if opts.TopK <= 0 {
		return []SearchResult{}, nil
	}
	alpha = max(float32(0), min(float32(1), alpha))
	filter, args := sqliteDocumentSelection(opts.DocumentFilter)
	keywords := map[string]int{}
	if literal := textsearch.FTSQuery(query); lexical && literal != "" {
		values := append([]any{literal}, args...)
		rows, err := s.db.Query(`SELECT c.id FROM chunk_terms JOIN chunks c ON c.rowid=chunk_terms.rowid WHERE chunk_terms MATCH ?`+filter+` ORDER BY bm25(chunk_terms),c.id LIMIT 20`, values...)
		if err != nil {
			return nil, fmt.Errorf("keyword retrieval unavailable: %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			keywords[id] = len(keywords) + 1
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("keyword retrieval unavailable: %w", err)
		}
	}
	h := &searchResultHeap{}
	heap.Init(h)
	if len(queryEmbedding) > 0 {
		rows, err := s.db.Query(`SELECT c.id,COALESCE(c.embedding_v1,c.embedding) FROM chunks c WHERE 1=1`+filter+` ORDER BY c.id`, args...)
		if err != nil {
			return nil, fmt.Errorf("semantic retrieval unavailable: %w", err)
		}
		for rows.Next() {
			var id string
			var data []byte
			if err := rows.Scan(&id, &data); err != nil {
				rows.Close()
				return nil, err
			}
			var vector []float32
			if len(data) >= 4 && string(data[:4]) == "OGV1" {
				vector, err = decodeVector(data)
			} else {
				err = json.Unmarshal(data, &vector)
			}
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("semantic vector unavailable: %w", err)
			}
			if len(vector) == 0 {
				continue
			}
			if len(vector) != len(queryEmbedding) {
				rows.Close()
				return nil, fmt.Errorf("semantic vector dimension differs from query")
			}
			score := cosineSimilarity(queryEmbedding, vector)
			candidate := SearchResult{Chunk: &Chunk{ID: id}, Score: score}
			if h.Len() < retrievalChannelCandidates {
				heap.Push(h, candidate)
			} else if score > (*h)[0].Score {
				heap.Pop(h)
				heap.Push(h, candidate)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("semantic retrieval unavailable: %w", err)
		}
	}
	semantic := make([]SearchResult, h.Len())
	for i := len(semantic) - 1; i >= 0; i-- {
		semantic[i] = heap.Pop(h).(SearchResult)
	}
	sort.SliceStable(semantic, func(i, j int) bool {
		if semantic[i].Score == semantic[j].Score {
			return semantic[i].Chunk.ID < semantic[j].Chunk.ID
		}
		return semantic[i].Score > semantic[j].Score
	})
	scores := map[string]float32{}
	for i, item := range semantic {
		if len(keywords) == 0 {
			scores[item.Chunk.ID] = item.Score
		} else {
			scores[item.Chunk.ID] = alpha * 61 / float32(61+i)
		}
	}
	for id, rank := range keywords {
		scores[id] += (1 - alpha) * 61 / float32(60+rank)
	}
	type ranked struct {
		id    string
		score float32
	}
	order := make([]ranked, 0, len(scores))
	for id, score := range scores {
		if score >= opts.MinScore {
			order = append(order, ranked{id, score})
		}
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].score == order[j].score {
			return order[i].id < order[j].id
		}
		return order[i].score > order[j].score
	})
	if len(order) > opts.TopK {
		order = order[:opts.TopK]
	}
	results := make([]SearchResult, 0, len(order))
	for _, candidate := range order {
		result, err := s.readSearchCandidate(candidate.id, filter, args)
		if err != nil {
			return nil, err
		}
		result.Score = candidate.score
		if !opts.IncludeContent {
			result.Chunk.Content = ""
			result.Chunk.Embedding = nil
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *SQLiteStore) readSearchCandidate(id, filter string, args []any) (SearchResult, error) {
	var result SearchResult
	var chunk Chunk
	var encoded []byte
	var metadata sql.NullString
	var rangesJSON string
	values := append([]any{id}, args...)
	err := s.db.QueryRow(`SELECT c.id,c.document_id,c.content,c.chunk_index,c.start_char,c.end_char,c.page,c.section,c.content_hash,COALESCE(c.embedding_v1,c.embedding),d.name,d.metadata,d.source_ranges,COALESCE(d.extraction_revision,'') FROM chunks c JOIN documents d ON d.id=c.document_id WHERE c.id=?`+filter, values...).Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &chunk.Index, &chunk.StartChar, &chunk.EndChar, &chunk.Page, &chunk.Section, &chunk.ContentHash, &encoded, &result.DocName, &metadata, &rangesJSON, &result.ExtractionRevision)
	if err != nil {
		return result, err
	}
	if len(encoded) >= 4 && string(encoded[:4]) == "OGV1" {
		chunk.Embedding, err = decodeVector(encoded)
	} else {
		err = json.Unmarshal(encoded, &chunk.Embedding)
	}
	if err != nil {
		return result, err
	}
	var ranges []SourceRange
	if err = json.Unmarshal([]byte(rangesJSON), &ranges); err != nil {
		return result, fmt.Errorf("invalid retained source ranges: %w", err)
	}
	bindChunkSource(&chunk, ranges)
	if chunk.ContentHash == "" {
		chunk.ContentHash = GenerateContentHash(chunk.Content)
	}
	if metadata.Valid && metadata.String != "" {
		if err = json.Unmarshal([]byte(metadata.String), &result.Metadata); err != nil {
			return result, err
		}
	}
	result.Chunk = &chunk
	result.DocumentID = chunk.DocumentID
	result.Locator = locatorForResult(result)
	return result, nil
}
