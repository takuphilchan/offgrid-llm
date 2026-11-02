package sessions

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// SearchNames searches names only. It neither returns nor indexes message bodies.
// Bound directory traversal and disk reads even for a cold, very large workspace.
func (s *ScopedManager) SearchNames(ctx context.Context, query string, limit int) ([]api.WorkSearchResult, bool, error) {
	results := []api.WorkSearchResult{}
	if !s.access.All && s.access.UserID == "" {
		return results, false, ErrAccessDenied
	}
	if limit < 1 || limit > 20 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return results, false, nil
	}
	dir, err := os.Open(s.manager.sessionsDir)
	if os.IsNotExist(err) {
		return results, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	needle := strings.ToLower(query)
	partial, scanned, remaining := false, 0, int64(32<<20)
	for {
		entries, readErr := dir.ReadDir(128)
		if readErr != nil && readErr != io.EOF {
			return nil, false, readErr
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return results, true, err
			}
			scanned++
			if scanned > 4096 || remaining <= 0 {
				partial = true
				break
			}
			if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".json")
			if !strings.Contains(strings.ToLower(name), needle) {
				continue
			}
			path, err := s.manager.sessionFilePath(name)
			if err != nil {
				continue
			}
			// Share the normal reader lock so Windows atomic replacement is not blocked.
			s.manager.fileMu.RLock()
			file, err := os.Open(path)
			var data []byte
			if err == nil {
				budget := min(remaining, int64(4<<20))
				data, err = io.ReadAll(io.LimitReader(file, budget+1))
				file.Close()
				remaining -= int64(len(data))
				if int64(len(data)) > budget {
					err = io.ErrShortBuffer
				}
			}
			s.manager.fileMu.RUnlock()
			if err != nil {
				partial = true
				continue
			}
			// Decode only routing/ownership metadata; never allocate message history.
			var header struct {
				Name      string    `json:"name"`
				OwnerID   string    `json:"owner_id"`
				UpdatedAt time.Time `json:"updated_at"`
			}
			if json.Unmarshal(data, &header) != nil || header.Name != name {
				partial = true
				continue
			}
			if !s.access.allows(&Session{OwnerID: header.OwnerID}) {
				continue
			}
			if s.access.Knowledge != nil {
				// Decode references only; text bodies are never indexed or returned.
				var refs struct {
					Messages []struct {
						KnowledgeDocuments []string `json:"knowledge_documents"`
						KnowledgeSources   []struct {
							DocumentID string `json:"document_id"`
						} `json:"knowledge_sources"`
					} `json:"messages"`
					Turn *struct {
						KnowledgeDocuments []string `json:"knowledge_documents"`
						KnowledgeSources   []struct {
							DocumentID string `json:"document_id"`
						} `json:"knowledge_sources"`
					} `json:"turn"`
				}
				if json.Unmarshal(data, &refs) != nil {
					partial = true
					continue
				}
				ids := []string{}
				for _, m := range refs.Messages {
					ids = append(ids, m.KnowledgeDocuments...)
					for _, p := range m.KnowledgeSources {
						ids = append(ids, p.DocumentID)
					}
				}
				if refs.Turn != nil {
					ids = append(ids, refs.Turn.KnowledgeDocuments...)
					for _, p := range refs.Turn.KnowledgeSources {
						ids = append(ids, p.DocumentID)
					}
				}
				if s.CheckKnowledge(ids) != nil {
					continue
				}
			}
			result := api.WorkSearchResult{Kind: "chat", ID: name, Title: name, UpdatedAt: header.UpdatedAt}
			results = append(results, result)
			sort.Slice(results, func(i, j int) bool {
				if results[i].UpdatedAt.Equal(results[j].UpdatedAt) {
					return results[i].ID < results[j].ID
				}
				return results[i].UpdatedAt.After(results[j].UpdatedAt)
			})
			if len(results) > limit {
				results = results[:limit]
				partial = true
			}
		}
		if scanned > 4096 || remaining <= 0 {
			partial = true
			break
		}
		if readErr == io.EOF {
			break
		}
	}
	return results, partial, nil
}
