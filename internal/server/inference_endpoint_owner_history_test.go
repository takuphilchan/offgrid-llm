package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func endpointOwnerHistoryFixture(t *testing.T, s *Server, query string) endpointOwnerHistoryPage {
	t.Helper()
	response := endpointManagementRequestMethod(s, "", "/configuration/history"+query, nil, "GET")
	var page endpointOwnerHistoryPage
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
		t.Fatal(response.Code, response.Body.String())
	}
	return page
}

func allEndpointOwnerHistoryFixture(t *testing.T, s *Server) []endpointOwnerArchivedReceipt {
	t.Helper()
	query, last := "", ""
	seen := map[string]bool{}
	result := []endpointOwnerArchivedReceipt{}
	for count := 0; count < 100; count++ {
		page := endpointOwnerHistoryFixture(t, s, query)
		if len(page.Receipts) > endpointHistoryPageSize {
			t.Fatal("unbounded page")
		}
		for _, receipt := range page.Receipts {
			id := receipt.Record.RequestID
			digest := endpointOwnerDigest([]byte(id))
			if seen[id] || digest <= last {
				t.Fatal("history duplicated or reordered", id)
			}
			seen[id], last = true, digest
			result = append(result, receipt)
		}
		if page.Next == "" {
			return result
		}
		if page.Next != last || len(page.Receipts) != endpointHistoryPageSize {
			t.Fatal("invalid continuation", page)
		}
		query = "?after=" + page.Next + "&journal_revision=" + page.JournalRevision
	}
	t.Fatal("history failed to terminate")
	return nil
}

func TestEndpointOwnerHistoryPagesRetainedReceiptsWithoutWrites(t *testing.T) {
	s, _, original := settledEndpointOwnerFixture(t)
	for batch := 0; batch < 2; batch++ {
		journal, err := readEndpointOwnerJournal(s.config.DataDir)
		if err != nil {
			t.Fatal(err)
		}
		journal.Records = []endpointOwnerRecord{}
		for index := 0; index < 35; index++ {
			record := original
			record.RequestID = fmt.Sprintf("history-%d-%02d", batch, index)
			journal.Records = append(journal.Records, record)
		}
		if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
			t.Fatal(err)
		}
		archiveEndpointOwnerFixture(t, s)
	}
	before := map[string]string{}
	for _, name := range []string{endpointOwnerJournalFilename, endpointPolicyFilename, "inference-endpoints.json", endpointOwnerArchiveMarker} {
		raw, err := os.ReadFile(filepath.Join(s.config.DataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = string(raw)
	}
	if records := allEndpointOwnerHistoryFixture(t, s); len(records) != 70 {
		t.Fatal("historical receipts missing", len(records))
	}
	for name, expected := range before {
		actual, err := os.ReadFile(filepath.Join(s.config.DataDir, name))
		if err != nil || string(actual) != expected {
			t.Fatal("history read mutated workspace", name, err)
		}
	}
	first := endpointOwnerHistoryFixture(t, s, "")
	journal, _ := readEndpointOwnerJournal(s.config.DataDir)
	journal.Records = []endpointOwnerRecord{original}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		t.Fatal(err)
	}
	response := endpointManagementRequestMethod(s, "", "/configuration/history?after="+first.Next+"&journal_revision="+first.JournalRevision, nil, "GET")
	if response.Code != 409 {
		t.Fatal("stale continuation accepted", response.Code)
	}
}

func TestEndpointOwnerHistoryExcludesUnretiredCopiesAndRejectsCorruption(t *testing.T) {
	s, _, record := settledEndpointOwnerFixture(t)
	// Preserve an original active record while a successful archival copy exists,
	// as happens when publication fails before replacing the active journal.
	archiveEndpointOwnerFixture(t, s)
	journal, _ := readEndpointOwnerJournal(s.config.DataDir)
	journal.Records = []endpointOwnerRecord{record}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		t.Fatal(err)
	}
	if page := endpointOwnerHistoryFixture(t, s, ""); len(page.Receipts) != 0 {
		t.Fatal("active copy presented as retired", page)
	}
	journal.Records = []endpointOwnerRecord{}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		t.Fatal(err)
	}
	path := endpointOwnerReceiptPath(s.config.DataDir, record.RequestID)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, damage := range []string{"wrong-identity", "wrong-archive", "oversized", "directory"} {
		t.Run(damage, func(t *testing.T) {
			var receipt endpointOwnerArchivedReceipt
			if json.Unmarshal(raw, &receipt) != nil {
				t.Fatal("fixture receipt")
			}
			switch damage {
			case "wrong-identity":
				receipt.Record.RequestID = "other-request"
			case "wrong-archive":
				receipt.ArchiveID = strings.Repeat("f", 64)
			case "oversized":
				receipt.ArchivedBy = strings.Repeat("x", maxEndpointOwnerArchivedReceiptBytes)
			case "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if damage != "directory" {
				if err := storage.WriteDurableJSON(path, receipt); err != nil {
					t.Fatal(err)
				}
			}
			response := endpointManagementRequestMethod(s, "", "/configuration/history", nil, "GET")
			if response.Code != 409 {
				t.Fatal("corrupt history disclosed", response.Code)
			}
			if damage == "directory" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEndpointOwnerHistoryQueriesAndCancellation(t *testing.T) {
	s, _, _ := settledEndpointOwnerFixture(t)
	archiveEndpointOwnerFixture(t, s)
	for _, query := range []string{"?after=../elsewhere", "?after=" + strings.Repeat("a", 64), "?journal_revision=bad", "?unexpected=" + strings.Repeat("a", 64), "?after=%zz", "?after=" + strings.Repeat("a", 64) + "&after=" + strings.Repeat("b", 64), "?journal_revision=" + strings.Repeat("a", 513)} {
		response := endpointManagementRequestMethod(s, "", "/configuration/history"+query, nil, "GET")
		if response.Code != 409 {
			t.Fatal("invalid query accepted", query, response.Code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.endpointOwnerHistory(httptest.NewRequest("GET", "/", nil).WithContext(ctx)); err == nil {
		t.Fatal("cancelled history read succeeded")
	}
}

func TestEndpointOwnerPolicyHistoryReadsEvidenceWithoutActivation(t *testing.T) {
	s, entry, before := endpointPolicyMaintenanceFixture(t, 1)
	retired := retireEndpointPolicyFixture(t, s)
	for _, query := range []string{"", "?revision=" + retired.History} {
		response := endpointManagementRequestMethod(s, "", "/configuration/policy-history"+query, nil, "GET")
		var page endpointPolicyHistoryPage
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil || page.Revision != retired.History || !reflect.DeepEqual(page.History.Snapshot, before) {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	old := entry
	old.Revision = "old-0"
	if s.endpointPolicy.allowed(old) == nil {
		t.Fatal("history read activated an old grant")
	}
	if err := os.Remove(endpointPolicyHistoryPath(s.config.DataDir, retired.History)); err != nil {
		t.Fatal(err)
	}
	if response := endpointManagementRequestMethod(s, "", "/configuration/policy-history?revision="+retired.History, nil, "GET"); response.Code != 409 {
		t.Fatal("missing history accepted")
	}
}

func TestEndpointOwnerHistoryRequiresCurrentAdministrator(t *testing.T) {
	s, _, _ := settledEndpointOwnerFixture(t)
	archiveEndpointOwnerFixture(t, s)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	_, key, err := s.userStore.CreateUser("history-reader", "isolated-history-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"/configuration/history", "/configuration/policy-history"} {
		if response := endpointManagementRequestMethod(s, key, action, nil, "GET"); response.Code != 403 {
			t.Fatal("nonadministrator read history", action, response.Code)
		}
	}
}
