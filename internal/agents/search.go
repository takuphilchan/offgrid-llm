package agents

import (
	"context"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// SearchRequests mirrors list/read visibility without cloning transcripts,
// checkpoints or credentials. Child work remains discoverable through its parent.
func (m *Manager) SearchRequests(ctx context.Context, actor string, admin bool, query string, limit int) ([]api.WorkSearchResult, bool, error) {
	m.mu.RLock()
	results := []api.WorkSearchResult{}
	if m.storageErr != nil {
		m.mu.RUnlock()
		return results, true, ErrRunStorage
	}
	// Copy only immutable strings and timestamps while holding the runner lock.
	// Case folding and ranking must not hold up Stop or task state persistence.
	candidates := make([]api.WorkSearchResult, 0, min(len(m.tasks), 4096))
	partial := false
	scanned := 0
	for _, task := range m.tasks {
		scanned++
		if scanned > 4096 {
			partial = true
			break
		}
		if task.DeletedAt != nil || task.ParentID != "" || task.Actor != "" && task.Actor != actor || !admin && (task.Actor != actor || task.Config.Profile == nil) {
			continue
		}
		candidates = append(candidates, api.WorkSearchResult{Kind: "task", ID: task.ID, Title: task.Prompt, UpdatedAt: task.CreatedAt})
	}
	m.mu.RUnlock()
	if limit < 1 || limit > 20 {
		limit = 20
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return results, false, nil
	}
	for _, task := range candidates {
		if err := ctx.Err(); err != nil {
			return results, true, err
		}
		if !strings.Contains(strings.ToLower(task.Title), needle) {
			continue
		}
		title := []rune(task.Title)
		if len(title) > 200 {
			title = append(title[:199], '…')
		}
		results = append(results, api.WorkSearchResult{Kind: "task", ID: task.ID, Title: string(title), UpdatedAt: task.UpdatedAt})
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
	return results, partial, nil
}
