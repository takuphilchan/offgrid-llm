package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

const endpointOwnerArchiveMarker = "inference-endpoint-owner-archive.json"
const maxEndpointOwnerArchivedReceiptBytes = 4096

type endpointOwnerArchiveIdentity struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
}

type endpointOwnerArchivedReceipt struct {
	Version    int                 `json:"version"`
	ArchiveID  string              `json:"archive_id"`
	ArchivedBy string              `json:"archived_by"`
	ArchivedAt string              `json:"archived_at"`
	Record     endpointOwnerRecord `json:"record"`
}

// Archive files preserve historical request identity; they are not enrollment
// or access authority. Direct hashed lookup has bounded work independent of the
// number of archived receipts. Files stay inside the owned workspace backup.
func endpointOwnerReceiptPath(directory, requestID string) string {
	return filepath.Join(directory, "inference-endpoint-receipt-"+endpointOwnerDigest([]byte(requestID))+".json")
}

func readEndpointOwnerArchiveID(directory, expected string) (string, error) {
	var identity endpointOwnerArchiveIdentity
	_, err := readEndpointOwnerJSON(filepath.Join(directory, endpointOwnerArchiveMarker), 1024, &identity)
	if errors.Is(err, os.ErrNotExist) && expected == "" {
		return "", nil
	}
	if err != nil || identity.Version != 1 || !validEndpointOwnerDigest(identity.ID) || expected != "" && expected != identity.ID {
		return "", errors.New("endpoint receipt archive identity is unavailable or changed")
	}
	return identity.ID, nil
}

func readArchivedEndpointOwnerRecord(directory, archiveID, requestID string) (*endpointOwnerRecord, error) {
	if !inferenceEndpointName.MatchString(requestID) {
		return nil, errors.New("invalid endpoint receipt identity")
	}
	id, err := readEndpointOwnerArchiveID(directory, archiveID)
	if err != nil || id == "" {
		return nil, err
	}
	receipt, err := readEndpointOwnerArchivedReceipt(directory, id, endpointOwnerDigest([]byte(requestID)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &receipt.Record, nil
}

func readEndpointOwnerArchivedReceipt(directory, archiveID, digest string) (*endpointOwnerArchivedReceipt, error) {
	if !validEndpointOwnerDigest(archiveID) || !validEndpointOwnerDigest(digest) {
		return nil, errors.New("invalid endpoint archive identity")
	}
	var receipt endpointOwnerArchivedReceipt
	_, err := readEndpointOwnerJSON(filepath.Join(directory, "inference-endpoint-receipt-"+digest+".json"), maxEndpointOwnerArchivedReceiptBytes, &receipt)
	if err != nil {
		return nil, err
	}
	_, timeErr := time.Parse(time.RFC3339Nano, receipt.ArchivedAt)
	if receipt.Version != 1 || receipt.ArchiveID != archiveID || receipt.ArchivedBy == "" || len(receipt.ArchivedBy) > 256 || timeErr != nil || endpointOwnerDigest([]byte(receipt.Record.RequestID)) != digest || (receipt.Record.State != "committed" && receipt.Record.State != "not_applied") || validateEndpointOwnerJournal(endpointOwnerJournal{Version: 1, Records: []endpointOwnerRecord{receipt.Record}}) != nil {
		return nil, errors.New("archived endpoint receipt is invalid")
	}
	return &receipt, nil
}

func endpointOwnerJournalRevision(journal endpointOwnerJournal) string {
	raw, _ := json.Marshal(journal)
	return endpointOwnerDigest(raw)
}

type endpointOwnerArchiveResult struct {
	Archived        int    `json:"archived"`
	JournalRevision string `json:"journal_revision"`
}

// Only an explicit administrator action can retire settled active receipts.
// Every receipt is durably copied and verified before publishing the smaller
// active journal. A failed copy retains all active records. A lost final write
// acknowledgement requires a fresh read; it cannot replay a configuration effect.
func (s *Server) archiveEndpointOwner(r *http.Request, expectedFile, expectedJournal string) (endpointOwnerArchiveResult, error) {
	var result endpointOwnerArchiveResult
	owner := s.endpointOwner
	if owner == nil || s.endpointPolicy == nil {
		return result, errors.New("endpoint owner configuration is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return result, err
	}
	authorize := func() error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		current, err := s.endpointAdministrator(r)
		if err != nil {
			return err
		}
		if current != actor || len(actor) > 256 {
			return errors.New("endpoint archival administrator changed")
		}
		return nil
	}
	if err := authorize(); err != nil {
		return result, err
	}
	if owner.fault != nil {
		return result, owner.fault
	}
	host, hostPin, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil {
		return result, err
	}
	release, err := acquireEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		return result, err
	}
	defer release()
	_, pin, err := s.readEndpointConfiguration()
	if err != nil || pin != expectedFile {
		return result, errors.New("endpoint owner file changed; refresh before archival")
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		return result, err
	}
	if endpointOwnerJournalRevision(journal) != expectedJournal {
		return result, errors.New("endpoint journal changed; refresh before archival")
	}
	for _, record := range journal.Records {
		if record.State != "committed" && record.State != "not_applied" {
			return result, errEndpointOwnerUncertain
		}
	}
	if len(journal.Records) == 0 {
		return endpointOwnerArchiveResult{JournalRevision: expectedJournal}, nil
	}
	id, err := readEndpointOwnerArchiveID(s.config.DataDir, journal.ArchiveID)
	if err != nil {
		return result, err
	}
	if id == "" {
		id, err = endpointOwnerNonce()
		if err != nil {
			return result, err
		}
		if err := s.writeEndpointOwner(filepath.Join(s.config.DataDir, endpointOwnerArchiveMarker), endpointOwnerArchiveIdentity{Version: 1, ID: id}); err != nil {
			return result, err
		}
	}
	for _, record := range journal.Records {
		existing, err := readArchivedEndpointOwnerRecord(s.config.DataDir, id, record.RequestID)
		if err != nil {
			return result, err
		}
		if existing != nil {
			if !reflect.DeepEqual(*existing, record) {
				return result, errors.New("archived endpoint receipt conflicts with active history")
			}
			continue
		}
		if err := authorize(); err != nil {
			return result, err
		}
		receipt := endpointOwnerArchivedReceipt{Version: 1, ArchiveID: id, ArchivedBy: actor, ArchivedAt: time.Now().UTC().Format(time.RFC3339Nano), Record: record}
		encoded, err := json.Marshal(receipt)
		if err != nil || len(encoded) > maxEndpointOwnerArchivedReceiptBytes {
			return result, errors.New("endpoint receipt exceeds the archive read bound; active history is retained")
		}
		if err := s.writeEndpointOwner(endpointOwnerReceiptPath(s.config.DataDir, record.RequestID), receipt); err != nil {
			return result, err
		}
		stored, err := readArchivedEndpointOwnerRecord(s.config.DataDir, id, record.RequestID)
		if err != nil || stored == nil || !reflect.DeepEqual(*stored, record) {
			return result, errors.New("archived endpoint receipt verification failed")
		}
	}
	if err := authorize(); err != nil {
		return result, err
	}
	_, currentHostPin, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil || currentHostPin != hostPin {
		return result, errors.New("installation endpoint policy changed during archival")
	}
	_, currentPin, err := s.readEndpointConfiguration()
	if err != nil || currentPin != expectedFile {
		return result, errors.New("endpoint owner file changed during archival")
	}
	next := endpointOwnerJournal{Version: 2, ArchiveID: id, Records: []endpointOwnerRecord{}}
	if err := s.writeEndpointOwner(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), next); err != nil {
		return result, errors.New("endpoint archival publication is uncertain; refresh actual journal state before further maintenance")
	}
	return endpointOwnerArchiveResult{Archived: len(journal.Records), JournalRevision: endpointOwnerJournalRevision(next)}, nil
}
