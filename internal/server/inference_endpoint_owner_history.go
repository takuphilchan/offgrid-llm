package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
)

const endpointHistoryPageSize = 32

// Match the workspace backup's maximum path-entry scale. Names are read in
// batches; only one page plus look-ahead is retained, never the whole directory.
const endpointHistoryScanLimit = 200002

type endpointOwnerHistoryPage struct {
	JournalRevision string                         `json:"journal_revision"`
	ArchiveID       string                         `json:"archive_id,omitempty"`
	Receipts        []endpointOwnerArchivedReceipt `json:"receipts"`
	Next            string                         `json:"next,omitempty"`
}

func endpointHistoryQuery(r *http.Request, allowed ...string) (map[string]string, error) {
	if len(r.URL.RawQuery) > 512 {
		return nil, errors.New("endpoint history query exceeds its bound")
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for key, values := range query {
		if !slices.Contains(allowed, key) || len(values) != 1 || !validEndpointOwnerDigest(values[0]) {
			return nil, errors.New("endpoint history query is invalid")
		}
		result[key] = values[0]
	}
	return result, nil
}

// Choose lexical hash order explicitly: directory enumeration order is not a
// stable pagination contract. A changed journal invalidates the continuation.
func endpointReceiptPageNames(ctx context.Context, directory, after string, active map[string]bool) ([]string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	selected := []string{}
	scanned := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		names, err := file.Readdirnames(128)
		scanned += len(names)
		if scanned > endpointHistoryScanLimit {
			return nil, errors.New("endpoint history exceeds the workspace directory scan limit")
		}
		for _, name := range names {
			if !strings.HasPrefix(name, "inference-endpoint-receipt-") {
				continue
			}
			digest := strings.TrimSuffix(strings.TrimPrefix(name, "inference-endpoint-receipt-"), ".json")
			if !strings.HasSuffix(name, ".json") || !validEndpointOwnerDigest(digest) {
				return nil, errors.New("endpoint history contains an invalid receipt filename")
			}
			if digest <= after || active[digest] {
				continue
			}
			index, found := slices.BinarySearch(selected, digest)
			if found {
				return nil, errors.New("endpoint history changed during enumeration")
			}
			if index > endpointHistoryPageSize {
				continue
			}
			selected = slices.Insert(selected, index, digest)
			if len(selected) > endpointHistoryPageSize+1 {
				selected = selected[:endpointHistoryPageSize+1]
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return selected, nil
}

func (s *Server) endpointOwnerHistory(r *http.Request) (endpointOwnerHistoryPage, error) {
	page := endpointOwnerHistoryPage{Receipts: []endpointOwnerArchivedReceipt{}}
	query, err := endpointHistoryQuery(r, "after", "journal_revision")
	if err != nil {
		return page, err
	}
	if query["after"] != "" && query["journal_revision"] == "" {
		return page, errors.New("history continuation requires its journal revision")
	}
	if s.endpointOwner == nil {
		return page, errors.New("endpoint owner history is unavailable")
	}
	s.endpointOwner.mu.Lock()
	defer s.endpointOwner.mu.Unlock()
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return page, err
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		return page, err
	}
	page.JournalRevision = endpointOwnerJournalRevision(journal)
	if query["journal_revision"] != "" && query["journal_revision"] != page.JournalRevision {
		return page, errors.New("endpoint history changed; restart browsing")
	}
	page.ArchiveID, err = readEndpointOwnerArchiveID(s.config.DataDir, journal.ArchiveID)
	if err != nil {
		return page, err
	}
	active := map[string]bool{}
	for _, record := range journal.Records {
		active[endpointOwnerDigest([]byte(record.RequestID))] = true
	}
	names, err := endpointReceiptPageNames(r.Context(), s.config.DataDir, query["after"], active)
	if err != nil {
		return page, err
	}
	if len(names) > endpointHistoryPageSize {
		names = names[:endpointHistoryPageSize]
		page.Next = names[len(names)-1]
	}
	for _, digest := range names {
		if err := r.Context().Err(); err != nil {
			return page, err
		}
		receipt, err := readEndpointOwnerArchivedReceipt(s.config.DataDir, page.ArchiveID, digest)
		if err != nil {
			return page, errors.New("endpoint history receipt is unavailable or invalid")
		}
		page.Receipts = append(page.Receipts, *receipt)
	}
	// Observe again before disclosing the page; neither a changed journal nor a
	// revoked administrator may turn an earlier observation into current authority.
	live, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || endpointOwnerJournalRevision(live) != page.JournalRevision {
		return page, errors.New("endpoint history changed while reading")
	}
	current, err := s.endpointAdministrator(r)
	if err != nil || current != actor {
		return page, errors.New("current administrator permission is required")
	}
	if err := r.Context().Err(); err != nil {
		return page, err
	}
	return page, nil
}

type endpointPolicyHistoryPage struct {
	Revision string                `json:"revision"`
	History  endpointPolicyHistory `json:"history"`
}

func (s *Server) endpointPolicyHistoryView(r *http.Request) (*endpointPolicyHistoryPage, error) {
	query, err := endpointHistoryQuery(r, "revision")
	if err != nil {
		return nil, err
	}
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return nil, err
	}
	digest := query["revision"]
	if digest == "" {
		s.nativeRuntimeMu.RLock()
		policy := s.endpointPolicy
		s.nativeRuntimeMu.RUnlock()
		if policy == nil {
			return nil, errors.New("endpoint policy is unavailable")
		}
		policy.mu.Lock()
		state, _, readErr := policy.readLocked()
		policy.mu.Unlock()
		if readErr != nil {
			return nil, readErr
		}
		digest = state.PreviousSnapshot
		if digest == "" {
			return nil, nil
		}
	}
	history, err := readEndpointPolicyHistory(s.config.DataDir, digest)
	if err != nil {
		return nil, errors.New("endpoint policy history is unavailable or invalid")
	}
	current, err := s.endpointAdministrator(r)
	if err != nil || current != actor {
		return nil, errors.New("current administrator permission is required")
	}
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	return &endpointPolicyHistoryPage{Revision: digest, History: *history}, nil
}
