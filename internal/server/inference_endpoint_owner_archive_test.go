package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func settledEndpointOwnerFixture(t *testing.T) (*Server, string, endpointOwnerRecord) {
	t.Helper()
	s := endpointOwnerFixture(t)
	review := prepareEndpointOwnerFixture(t, s, "", endpointOwnerInputFixture())
	if response := commitEndpointOwnerFixture(s, "", review, "archival-fixture"); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	return s, review, reconcileEndpointOwnerFixture(t, s, "", "archival-fixture")
}

func archiveEndpointOwnerFixture(t *testing.T, s *Server) endpointOwnerArchiveResult {
	t.Helper()
	state := endpointOwnerSnapshotFixture(t, s, "")
	response := endpointManagementRequest(s, "", "/configuration/archive", map[string]string{"expected_revision": state.FileRevision, "expected_journal_revision": state.JournalRevision})
	var result endpointOwnerArchiveResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil {
		t.Fatal(response.Code, response.Body.String())
	}
	return result
}

func TestEndpointOwnerArchivePreservesReceiptAndDuplicateProtection(t *testing.T) {
	s, review, record := settledEndpointOwnerFixture(t)
	configuration, err := os.ReadFile(s.endpointConfigurationPath())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
	if err != nil {
		t.Fatal(err)
	}
	if result := archiveEndpointOwnerFixture(t, s); result.Archived != 1 || !validEndpointOwnerDigest(result.JournalRevision) {
		t.Fatal("invalid archival result", result)
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || journal.Version != 2 || len(journal.Records) != 0 || journal.ArchiveID == "" {
		t.Fatal("active journal capacity was not released", journal, err)
	}
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	s.endpointOwner.write = func(string, any) error {
		t.Error("historical lookup attempted a write")
		return errors.New("unexpected write")
	}
	response := commitEndpointOwnerFixture(s, "", review, record.RequestID)
	var duplicate endpointOwnerRecord
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &duplicate) != nil || !reflect.DeepEqual(duplicate, record) {
		t.Fatal("archived duplicate lost its original receipt", response.Code, response.Body.String())
	}
	if got := reconcileEndpointOwnerFixture(t, s, "", record.RequestID); !reflect.DeepEqual(got, record) {
		t.Fatal("historical reconciliation rewrote the receipt")
	}
	input := endpointOwnerInputFixture()
	input.Replace, input.Revision = input.ID+"@"+input.Revision, "r2"
	newReview := prepareEndpointOwnerFixture(t, s, "", input)
	if response := commitEndpointOwnerFixture(s, "", newReview, record.RequestID); response.Code != 409 {
		t.Fatal("archived request identity accepted a different review", response.Code)
	}
	for path, before := range map[string][]byte{s.endpointConfigurationPath(): configuration, filepath.Join(s.config.DataDir, endpointPolicyFilename): policy} {
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(before) {
			t.Fatal("archival changed enrollment or access", path, err)
		}
	}
}

func TestEndpointOwnerArchiveRetainsHistoryAcrossWriteFailures(t *testing.T) {
	for _, phase := range []string{"marker", "receipt", "journal"} {
		for _, after := range []bool{false, true} {
			t.Run(phase+map[bool]string{false: "/before", true: "/after"}[after], func(t *testing.T) {
				s, review, record := settledEndpointOwnerFixture(t)
				state := endpointOwnerSnapshotFixture(t, s, "")
				matched := false
				s.endpointOwner.write = func(path string, value any) error {
					match := phase == "marker" && filepath.Base(path) == endpointOwnerArchiveMarker || phase == "receipt" && strings.HasPrefix(filepath.Base(path), "inference-endpoint-receipt-") || phase == "journal" && filepath.Base(path) == endpointOwnerJournalFilename
					if match && !after {
						matched = true
						return errors.New("fixture write failure")
					}
					if err := storage.WriteDurableJSON(path, value); err != nil {
						return err
					}
					if match {
						matched = true
						return errors.New("fixture lost durability acknowledgement")
					}
					return nil
				}
				response := endpointManagementRequest(s, "", "/configuration/archive", map[string]string{"expected_revision": state.FileRevision, "expected_journal_revision": state.JournalRevision})
				if response.Code == 200 || !matched {
					t.Fatal("failure was not surfaced", response.Code, response.Body.String())
				}
				s = restartEndpointOwnerFixture(t, s.config.DataDir)
				response = commitEndpointOwnerFixture(s, "", review, record.RequestID)
				var actual endpointOwnerRecord
				if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &actual) != nil || !reflect.DeepEqual(actual, record) {
					t.Fatal("failed archival lost duplicate protection", response.Code, response.Body.String())
				}
				// A fresh explicit request can finish partial archival; the first
				// request is never automatically replayed.
				archiveEndpointOwnerFixture(t, s)
				if got := reconcileEndpointOwnerFixture(t, s, "", record.RequestID); !reflect.DeepEqual(got, record) {
					t.Fatal("receipt changed")
				}
			})
		}
	}
}

func TestEndpointOwnerArchiveRejectsUnsettledOrStaleJournal(t *testing.T) {
	for _, state := range []string{"pending", "published", "conflict", "stale"} {
		t.Run(state, func(t *testing.T) {
			s, _, record := settledEndpointOwnerFixture(t)
			view := endpointOwnerSnapshotFixture(t, s, "")
			if state != "stale" {
				record.State = state
				if state != "conflict" {
					record.ResolvedAt, record.ResolvedBy, record.ResolvedEpoch = "", "", ""
				}
				if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), endpointOwnerJournal{Version: 1, Records: []endpointOwnerRecord{record}}); err != nil {
					t.Fatal(err)
				}
				view = endpointOwnerSnapshotFixture(t, s, "")
			} else {
				view.JournalRevision = strings.Repeat("0", 64)
			}
			s.endpointOwner.write = func(string, any) error {
				t.Error("rejected archival attempted a write")
				return errors.New("unexpected write")
			}
			response := endpointManagementRequest(s, "", "/configuration/archive", map[string]string{"expected_revision": view.FileRevision, "expected_journal_revision": view.JournalRevision})
			if response.Code == 200 {
				t.Fatal("unsettled/stale archival was accepted")
			}
		})
	}
}

func TestEndpointOwnerArchiveMissingIdentityOrJournalFailsClosed(t *testing.T) {
	for _, name := range []string{endpointOwnerArchiveMarker, endpointOwnerJournalFilename} {
		t.Run(name, func(t *testing.T) {
			s, _, _ := settledEndpointOwnerFixture(t)
			archiveEndpointOwnerFixture(t, s)
			if err := os.Remove(filepath.Join(s.config.DataDir, name)); err != nil {
				t.Fatal(err)
			}
			if err := endpointOwnerJournalReady(s.config.DataDir); err == nil {
				t.Fatal("missing archival identity silently reset history")
			}
		})
	}
}

func TestEndpointOwnerArchiveReleasesFullJournalCapacity(t *testing.T) {
	s, _, record := settledEndpointOwnerFixture(t)
	journal := endpointOwnerJournal{Version: 1, Records: make([]endpointOwnerRecord, 64)}
	for index := range journal.Records {
		copy := record
		copy.RequestID = fmt.Sprintf("historical-%d", index)
		copy.ReviewID = endpointOwnerDigest([]byte(copy.RequestID))
		journal.Records[index] = copy
	}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		t.Fatal(err)
	}
	if result := archiveEndpointOwnerFixture(t, s); result.Archived != 64 {
		t.Fatal(result)
	}
	for _, record := range journal.Records {
		response := commitEndpointOwnerFixture(s, "", record.ReviewID, record.RequestID)
		if response.Code != 200 {
			t.Fatal("full journal lost historical identity", response.Code, response.Body.String())
		}
	}
	input := endpointOwnerInputFixture()
	input.Replace, input.Revision = input.ID+"@"+input.Revision, "r2"
	review := prepareEndpointOwnerFixture(t, s, "", input)
	if response := commitEndpointOwnerFixture(s, "", review, "after-archival"); response.Code != 200 {
		t.Fatal("capacity remained blocked", response.Code, response.Body.String())
	}
}

func TestEndpointOwnerArchiveCancellationRetainsActiveRecords(t *testing.T) {
	s, _, _ := settledEndpointOwnerFixture(t)
	state := endpointOwnerSnapshotFixture(t, s, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.endpointOwner.write = func(path string, value any) error {
		if err := storage.WriteDurableJSON(path, value); err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(path), "inference-endpoint-receipt-") {
			cancel()
		}
		return nil
	}
	request := httptest.NewRequest("POST", inferenceEndpointsPath+"/configuration/archive", nil).WithContext(ctx)
	if _, err := s.archiveEndpointOwner(request, state.FileRevision, state.JournalRevision); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation was ignored", err)
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || len(journal.Records) != 1 {
		t.Fatal("cancelled archival cleared active history", journal, err)
	}
}

func TestEndpointOwnerArchiveRejectsCorruptReceiptWithoutOverwriting(t *testing.T) {
	s, review, record := settledEndpointOwnerFixture(t)
	archiveEndpointOwnerFixture(t, s)
	path := endpointOwnerReceiptPath(s.config.DataDir, record.RequestID)
	corrupt := []byte(`{"version":1,"record":{}}`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if response := commitEndpointOwnerFixture(s, "", review, record.RequestID); response.Code != 409 {
		t.Fatal("corrupt history accepted", response.Code)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(corrupt) {
		t.Fatal("corrupt evidence overwritten", err)
	}
}

func TestEndpointOwnerArchiveRejectsOversizeReceiptBeforeCopy(t *testing.T) {
	s, _, record := settledEndpointOwnerFixture(t)
	// RFC3339 permits fractional precision beyond nanoseconds. Such historical
	// input can fit the active journal but exceed the per-receipt archive bound.
	record.CreatedAt = "2026-11-21T01:00:00." + strings.Repeat("0", 4096) + "Z"
	journal := endpointOwnerJournal{Version: 1, Records: []endpointOwnerRecord{record}}
	if err := validateEndpointOwnerJournal(journal); err != nil {
		t.Fatal("invalid historical fixture", err)
	}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		t.Fatal(err)
	}
	state := endpointOwnerSnapshotFixture(t, s, "")
	response := endpointManagementRequest(s, "", "/configuration/archive", map[string]string{"expected_revision": state.FileRevision, "expected_journal_revision": state.JournalRevision})
	if response.Code != 409 {
		t.Fatal("oversize archive was accepted", response.Code)
	}
	if _, err := os.Lstat(endpointOwnerReceiptPath(s.config.DataDir, record.RequestID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unreadable archival receipt was published", err)
	}
	retained, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || !reflect.DeepEqual(retained, journal) {
		t.Fatal("oversize archival changed active history", err)
	}
}
