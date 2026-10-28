package execution

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"unicode/utf8"
)

type Match struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Text      string `json:"text"`
	Revision  string `json:"revision"`
	Truncated bool   `json:"truncated"`
}
type SearchResult struct {
	Location
	Matches   []Match `json:"matches"`
	Complete  bool    `json:"complete"`
	FilesRead int     `json:"files_read"`
	Skipped   int     `json:"skipped"`
}

// Search is literal, case-sensitive and bounded by files, bytes, entries and
// matches. Incomplete results are never evidence that the remaining tree has no
// matches. Links and special files are skipped, never recursively followed.
func (s *Session) Search(ctx context.Context, binding Binding, path, query string, limit int) (SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return SearchResult{}, err
	}
	if !s.config.Permissions.Read {
		return SearchResult{}, ErrAuthority
	}
	if query == "" || len(query) > 256 || !utf8.ValidString(query) || limit < 1 || limit > 128 {
		return SearchResult{}, ErrLimit
	}
	name, err := s.resolve(path, s.config.Directory)
	if err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Location: s.location(name), Matches: []Match{}, Complete: true}
	entries, totalBytes := 0, 0
	stop := errors.New("bounded search complete")
	err = fs.WalkDir(s.root.FS(), name, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			result.Skipped++
			result.Complete = false
			return nil
		}
		entries++
		if entries > MaxDirectoryEntries || result.FilesRead >= 128 || totalBytes >= 32*1024*1024 {
			result.Complete = false
			return stop
		}
		if d.IsDir() {
			return nil
		}
		if !utf8.ValidString(path) {
			result.Skipped++
			result.Complete = false
			return nil
		}
		if !d.Type().IsRegular() {
			result.Skipped++
			result.Complete = false
			return nil
		}
		data, readErr := s.read(ctx, path)
		if readErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			result.Skipped++
			result.Complete = false
			return nil
		}
		result.FilesRead++
		totalBytes += len(data)
		if totalBytes > 32*1024*1024 {
			result.Complete = false
			return stop
		}
		if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
			result.Skipped++
			result.Complete = false
			return nil
		}
		revision := digest(data)
		for line, text := range strings.Split(string(data), "\n") {
			if !strings.Contains(text, query) {
				continue
			}
			// Long lines are previews only; file reads retain the original bytes.
			truncated := len(text) > 1024
			if truncated {
				n := 1024
				for n > 0 && !utf8.RuneStart(text[n]) {
					n--
				}
				text = text[:n]
			}
			result.Matches = append(result.Matches, Match{Path: path, Line: line + 1, Text: text, Revision: revision, Truncated: truncated})
			if len(result.Matches) == limit {
				result.Complete = false
				return stop
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return SearchResult{}, err
	}
	return result, nil
}
