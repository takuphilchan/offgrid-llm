package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func (s *Server) prepareEndpointOwner(r *http.Request, input endpointOwnerInput) (endpointOwnerDraft, error) {
	var draft endpointOwnerDraft
	owner := s.endpointOwner
	if owner == nil || s.endpointPolicy == nil {
		return draft, errors.New("endpoint owner configuration is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.fault != nil {
		return draft, owner.fault
	}
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return draft, err
	}
	if err := endpointOwnerJournalReady(s.config.DataDir); err != nil {
		return draft, err
	}
	host, hostPin, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil {
		return draft, err
	}
	release, err := acquireEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		return draft, err
	}
	defer release()
	configuration, pin, err := s.readEndpointConfiguration()
	if err != nil {
		return draft, err
	}
	if input.Expected != pin {
		return draft, errors.New("endpoint owner file changed; prepare a new review")
	}
	entry, err := input.entry(host)
	if err != nil {
		return draft, err
	}
	for _, candidate := range entry.Actors {
		if !s.scheduleActorPermission(candidate, users.PermissionChat) {
			return draft, errors.New("configured endpoint actor lacks current chat permission")
		}
	}
	configuration, err = replaceEndpointOwnerEntry(configuration, input.Replace, entry)
	if err != nil {
		return draft, err
	}
	s.endpointPolicy.mu.Lock()
	state, _, err := s.endpointPolicy.readLocked()
	s.endpointPolicy.mu.Unlock()
	if err != nil {
		return draft, err
	}
	epoch := state.Epoch
	if epoch == "" {
		epoch, err = storage.EnsureWorkspaceActivationEpoch(s.config.DataDir)
		if err != nil {
			return draft, err
		}
	}
	now := time.Now()
	for key, value := range owner.drafts {
		if !now.Before(value.Expires) {
			delete(owner.drafts, key)
		}
	}
	if len(owner.drafts) >= 16 {
		return draft, errors.New("endpoint review capacity reached; wait for an existing review to expire")
	}
	id, err := endpointOwnerNonce()
	if err != nil {
		return draft, err
	}
	draft = endpointOwnerDraft{ID: id, Actor: actor, Epoch: epoch, Expires: now.Add(5 * time.Minute), Expected: pin, HostPin: hostPin, Configuration: configuration, Entry: entry}
	owner.drafts[id] = draft
	return draft, nil
}

func (s *Server) writeEndpointOwner(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > 64<<10 {
		return errors.New("endpoint owner snapshot exceeds its bound")
	}
	if s.endpointOwner.write != nil {
		return s.endpointOwner.write(path, value)
	}
	return storage.WriteDurableJSON(path, value)
}

func (s *Server) blockEndpointOwner() {
	s.endpointOwner.fault = errEndpointOwnerUncertain
	policy := s.endpointPolicy
	policy.mu.Lock()
	defer policy.mu.Unlock()
	policy.ownerFault = errEndpointOwnerUncertain
	for _, requests := range policy.active {
		for request := range requests {
			request.cancel()
		}
	}
}

func (s *Server) commitEndpointOwner(r *http.Request, reviewID, requestID string) (endpointOwnerRecord, error) {
	var record endpointOwnerRecord
	owner := s.endpointOwner
	if owner == nil || s.endpointPolicy == nil {
		return record, errors.New("endpoint owner configuration is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return record, err
	}
	if !inferenceEndpointName.MatchString(requestID) || len(reviewID) != 64 {
		return record, errors.New("a bounded review and request identity are required")
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		return record, err
	}
	for _, existing := range journal.Records {
		if existing.RequestID == requestID {
			if existing.Actor != actor || existing.ReviewID != reviewID {
				return record, errors.New("request identity belongs to another review")
			}
			if existing.State == "pending" || existing.State == "conflict" || owner.fault != nil {
				return existing, errEndpointOwnerUncertain
			}
			return existing, nil
		}
	}
	archived, err := readArchivedEndpointOwnerRecord(s.config.DataDir, journal.ArchiveID, requestID)
	if err != nil {
		return record, err
	}
	if archived != nil {
		if archived.Actor != actor || archived.ReviewID != reviewID {
			return record, errors.New("request identity belongs to another archived review")
		}
		return *archived, nil
	}
	if owner.fault != nil {
		return record, owner.fault
	}
	if err := endpointOwnerJournalReady(s.config.DataDir); err != nil {
		return record, err
	}
	draft, ok := owner.drafts[reviewID]
	if !ok || draft.Actor != actor || !time.Now().Before(draft.Expires) {
		return record, errors.New("endpoint review is unavailable or expired")
	}
	if len(journal.Records) >= 64 {
		return record, errors.New("endpoint owner audit capacity reached; explicit archival maintenance is required")
	}
	host, hostPin, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil || hostPin != draft.HostPin {
		return record, errors.New("installation endpoint policy changed; prepare a new review")
	}
	release, err := acquireEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		return record, err
	}
	defer release()
	current, pin, err := s.readEndpointConfiguration()
	if err != nil {
		return record, err
	}
	epoch, err := storage.ReadRestoreEpoch(s.config.DataDir)
	if err != nil || epoch != draft.Epoch || pin != draft.Expected {
		return record, errors.New("workspace or owner file changed; prepare a new review")
	}
	for _, candidate := range draft.Entry.Actors {
		if !s.scheduleActorPermission(candidate, users.PermissionChat) {
			return record, errors.New("configured actor authority changed")
		}
	}
	// Revocation precedes any file mutation. Even a byte-identical later restore
	// of the old entry cannot reactivate the prior policy grant.
	entries := []inferenceEndpointEnrollment{draft.Entry}
	for _, entry := range current.Endpoints {
		if entry.ID == draft.Entry.ID && !slices.ContainsFunc(entries, func(value inferenceEndpointEnrollment) bool { return endpointKey(value) == endpointKey(entry) }) {
			entries = append(entries, entry)
		}
	}
	for _, entry := range entries {
		policy := s.endpointPolicy
		policy.mu.Lock()
		state, _, readErr := policy.readLocked()
		policy.mu.Unlock()
		if readErr != nil {
			return record, readErr
		}
		if err := policy.revokeForOwner(entry, state.Revision, actor, func() error { _, err := s.endpointAdministrator(r); return err }); err != nil {
			return record, err
		}
	}
	encoded, err := json.Marshal(draft.Configuration)
	if err != nil {
		return record, err
	}
	record = endpointOwnerRecord{RequestID: requestID, ReviewID: reviewID, Actor: actor, Epoch: epoch, Before: pin, After: endpointOwnerDigest(encoded), Endpoint: endpointKey(draft.Entry), State: "pending", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	journal.Records = append(journal.Records, record)
	journalPath := filepath.Join(s.config.DataDir, endpointOwnerJournalFilename)
	if err := s.writeEndpointOwner(journalPath, journal); err != nil {
		s.blockEndpointOwner()
		return record, errors.Join(errEndpointOwnerUncertain, err)
	}
	delete(owner.drafts, reviewID)
	if err := verifyEndpointOwnerPath(filepath.Dir(s.endpointConfigurationPath()), true); err != nil {
		s.blockEndpointOwner()
		return record, errors.Join(errEndpointOwnerUncertain, err)
	}
	// Recheck after intent publication, immediately before the one permitted
	// owner-file write. No failed write is automatically retried.
	_, pin, err = s.readEndpointConfiguration()
	if err != nil || pin != draft.Expected {
		s.blockEndpointOwner()
		return record, errEndpointOwnerUncertain
	}
	if err := s.writeEndpointOwner(s.endpointConfigurationPath(), draft.Configuration); err != nil {
		s.blockEndpointOwner()
		return record, errors.Join(errEndpointOwnerUncertain, err)
	}
	record.State = "published"
	journal.Records[len(journal.Records)-1] = record
	if err := s.writeEndpointOwner(journalPath, journal); err != nil {
		s.blockEndpointOwner()
		return record, errors.Join(errEndpointOwnerUncertain, err)
	}
	return record, nil
}

// Reconciliation is a distinct administrator action over actual stored bytes.
// It never repeats configuration publication or enables any endpoint grant.
func (s *Server) reconcileEndpointOwner(r *http.Request, requestID, expected string) (endpointOwnerRecord, error) {
	var record endpointOwnerRecord
	owner := s.endpointOwner
	if owner == nil || s.endpointPolicy == nil {
		return record, errors.New("endpoint owner configuration is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	actor, err := s.endpointAdministrator(r)
	if err != nil {
		return record, err
	}
	host, _, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil {
		return record, err
	}
	release, err := acquireEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		return record, err
	}
	defer release()
	_, pin, err := s.readEndpointConfiguration()
	if err != nil || pin != expected {
		return record, errors.New("actual endpoint owner file changed; review its current digest")
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		return record, err
	}
	index := slices.IndexFunc(journal.Records, func(value endpointOwnerRecord) bool { return value.RequestID == requestID })
	if index < 0 {
		archived, archiveErr := readArchivedEndpointOwnerRecord(s.config.DataDir, journal.ArchiveID, requestID)
		if archiveErr != nil {
			return record, archiveErr
		}
		if archived != nil {
			return *archived, nil
		}
		return record, errors.New("endpoint owner intent is unavailable")
	}
	record = journal.Records[index]
	// A later configuration cannot change the historical outcome of a settled
	// publication. Repeated inspection returns the same receipt, with no write.
	if record.State == "committed" || record.State == "not_applied" {
		return record, nil
	}
	epoch, err := storage.ReadRestoreEpoch(s.config.DataDir)
	if err != nil || epoch == "" {
		return record, errors.New("workspace activation epoch is unavailable")
	}
	if pin == record.After {
		record.State = "committed"
	} else if pin == record.Before {
		record.State = "not_applied"
	} else {
		record.State = "conflict"
	}
	record.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.ResolvedBy = actor
	record.ResolvedEpoch = epoch
	journal.Records[index] = record
	if err := s.writeEndpointOwner(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), journal); err != nil {
		s.blockEndpointOwner()
		return record, errors.Join(errEndpointOwnerUncertain, err)
	}
	if record.State == "conflict" {
		return record, errEndpointOwnerUncertain
	}
	if err := endpointOwnerJournalReady(s.config.DataDir); err != nil {
		return record, err
	}
	owner.fault = nil
	s.endpointPolicy.mu.Lock()
	s.endpointPolicy.ownerFault = nil
	s.endpointPolicy.mu.Unlock()
	return record, nil
}
