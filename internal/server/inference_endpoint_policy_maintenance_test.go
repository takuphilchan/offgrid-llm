package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func endpointPolicyMaintenanceFixture(t *testing.T, count int) (*Server, inferenceEndpointEnrollment, endpointPolicySnapshot) {
	t.Helper()
	s, _, _ := settledEndpointOwnerFixture(t)
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	entry := s.installedEndpoints[0]
	reviewEndpointFixture(t, s, entry)
	s.endpointPolicy.mu.Lock()
	defer s.endpointPolicy.mu.Unlock()
	state, _, err := s.endpointPolicy.readLocked()
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		old := entry
		old.Revision = fmt.Sprintf("old-%d", index)
		state.Decisions[endpointKey(old)] = endpointDecision{Pin: endpointPin(old), Enabled: index%2 == 0, Actor: "local-admin", At: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	if err := storage.WriteDurableJSON(filepath.Join(s.config.DataDir, endpointPolicyFilename), state); err != nil {
		t.Fatal(err)
	}
	return s, entry, state
}

func retireEndpointPolicyFixture(t *testing.T, s *Server) endpointPolicyMaintenanceResult {
	t.Helper()
	view := endpointOwnerSnapshotFixture(t, s, "")
	if view.PolicyMaintenance == nil {
		t.Fatal("maintenance preview unavailable")
	}
	response := endpointManagementRequest(s, "", "/configuration/retire-decisions", endpointPolicyMaintenanceInput{Expected: view.FileRevision, PolicyRevision: view.PolicyMaintenance.Revision, Endpoints: view.PolicyMaintenance.Retirable})
	var result endpointPolicyMaintenanceResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil {
		t.Fatal(response.Code, response.Body.String())
	}
	return result
}

func TestEndpointPolicyMaintenanceArchivesWithoutRestoringGrants(t *testing.T) {
	s, entry, before := endpointPolicyMaintenanceFixture(t, 3)
	result := retireEndpointPolicyFixture(t, s)
	if len(result.Retired) != 3 || result.Revision == before.Revision {
		t.Fatal(result)
	}
	history, err := readEndpointPolicyHistory(s.config.DataDir, result.History)
	if err != nil || !reflect.DeepEqual(history.Snapshot, before) {
		t.Fatal("original policy was not archived", history, err)
	}
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	state, _, err := s.endpointPolicy.readLocked()
	if err != nil || len(state.Decisions) != 1 || state.Decisions[endpointKey(entry)] != before.Decisions[endpointKey(entry)] || state.PreviousSnapshot != result.History {
		t.Fatal("current decision changed", state, err)
	}
	if err := s.endpointPolicy.allowed(entry); err != nil {
		t.Fatal("maintenance revoked current reviewed access", err)
	}
	old := entry
	old.Revision = "old-0"
	if err := s.endpointPolicy.allowed(old); err == nil {
		t.Fatal("archive reactivated old enabled decision")
	}
	if err := os.Remove(endpointPolicyHistoryPath(s.config.DataDir, result.History)); err != nil {
		t.Fatal(err)
	}
	if err := s.endpointPolicy.allowed(entry); err == nil {
		t.Fatal("missing linked audit was silently accepted")
	}
}

func TestEndpointPolicyMaintenanceProtectsConfiguredInstalledAndActive(t *testing.T) {
	s, entry, _ := endpointPolicyMaintenanceFixture(t, 3)
	installed := entry
	installed.Revision = "old-0"
	s.installedEndpoints = append(s.installedEndpoints, installed)
	active := entry
	active.Revision = "old-2"
	_, release, err := s.endpointPolicy.acquire(context.Background(), active)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	view := endpointOwnerSnapshotFixture(t, s, "")
	if !reflect.DeepEqual(view.PolicyMaintenance.Retirable, []string{entry.ID + "@old-1"}) {
		t.Fatal(view.PolicyMaintenance)
	}
	for _, key := range []string{endpointKey(entry), endpointKey(installed), endpointKey(active)} {
		response := endpointManagementRequest(s, "", "/configuration/retire-decisions", endpointPolicyMaintenanceInput{Expected: view.FileRevision, PolicyRevision: view.PolicyMaintenance.Revision, Endpoints: []string{key}})
		if response.Code != 409 {
			t.Fatal("protected decision retired", key, response.Code)
		}
	}
	result := retireEndpointPolicyFixture(t, s)
	if !reflect.DeepEqual(result.Retired, view.PolicyMaintenance.Retirable) {
		t.Fatal(result)
	}
}

func TestEndpointPolicyMaintenanceFailurePreservesAuditAndFencesAuthority(t *testing.T) {
	for _, phase := range []string{"archive-before", "archive-after", "policy-before", "policy-after"} {
		t.Run(phase, func(t *testing.T) {
			s, entry, before := endpointPolicyMaintenanceFixture(t, 1)
			lease, release, err := s.endpointPolicy.acquire(context.Background(), entry)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			view := endpointOwnerSnapshotFixture(t, s, "")
			write := func(path string, value any) error {
				if phase == "archive-before" || phase == "policy-before" {
					return errors.New("fixture before publication")
				}
				if err := storage.WriteDurableJSON(path, value); err != nil {
					return err
				}
				return errors.New("fixture lost acknowledgement")
			}
			if phase == "archive-before" || phase == "archive-after" {
				s.endpointOwner.write = write
			} else {
				s.endpointPolicy.writeSnapshot = write
			}
			response := endpointManagementRequest(s, "", "/configuration/retire-decisions", endpointPolicyMaintenanceInput{Expected: view.FileRevision, PolicyRevision: view.PolicyMaintenance.Revision, Endpoints: view.PolicyMaintenance.Retirable})
			if response.Code == 200 {
				t.Fatal("publication failure acknowledged")
			}
			if phase == "policy-before" || phase == "policy-after" {
				if lease.Err() == nil || s.endpointPolicy.allowed(entry) == nil {
					t.Fatal("uncertain policy retained authority")
				}
			} else if lease.Err() != nil {
				t.Fatal("archive-only failure changed current authority")
			}
			s = restartEndpointOwnerFixture(t, s.config.DataDir)
			state, _, err := s.endpointPolicy.readLocked()
			if err != nil {
				t.Fatal(err)
			}
			if phase == "policy-after" {
				if len(state.Decisions) != 1 {
					t.Fatal("published state was replayed or lost", state)
				}
				history, err := readEndpointPolicyHistory(s.config.DataDir, state.PreviousSnapshot)
				if err != nil || !reflect.DeepEqual(history.Snapshot, before) {
					t.Fatal("audit lost after publication", err)
				}
			} else if !reflect.DeepEqual(state, before) {
				t.Fatal("failed maintenance changed policy", state)
			}
		})
	}
}

func TestEndpointPolicyMaintenanceFreesCapacityWithoutEnablingNewIdentity(t *testing.T) {
	s, entry, before := endpointPolicyMaintenanceFixture(t, 63)
	missing := entry
	missing.Revision = "new-revision"
	if err := s.endpointPolicy.revokeForOwner(missing, before.Revision, "local-admin", func() error { return nil }); err != nil {
		t.Fatal("already-denied identity blocked revocation at capacity", err)
	}
	if err := s.endpointPolicy.allowed(missing); err == nil {
		t.Fatal("absence became access")
	}
	if err := s.endpointPolicy.update(missing, before.Revision, "local-admin", true, func() error { return nil }); err == nil {
		t.Fatal("unmaintained full store enabled a new identity")
	}
	result := retireEndpointPolicyFixture(t, s)
	if len(result.Retired) != 63 {
		t.Fatal(result)
	}
	state, _, err := s.endpointPolicy.readLocked()
	if err != nil || len(state.Decisions) != 1 {
		t.Fatal(state, err)
	}
	if err := s.endpointPolicy.revokeForOwner(missing, state.Revision, "local-admin", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := s.endpointPolicy.allowed(missing); err == nil {
		t.Fatal("maintenance implicitly enabled new identity")
	}
}
