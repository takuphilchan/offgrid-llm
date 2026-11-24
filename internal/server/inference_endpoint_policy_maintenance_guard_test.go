package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestEndpointPolicyMaintenanceRejectsStaleReviewBeforeArchival(t *testing.T) {
	s, _, before := endpointPolicyMaintenanceFixture(t, 1)
	view := endpointOwnerSnapshotFixture(t, s, "")
	input := endpointPolicyMaintenanceInput{Expected: view.FileRevision, PolicyRevision: view.PolicyMaintenance.Revision, Endpoints: view.PolicyMaintenance.Retirable}
	for _, changed := range []endpointPolicyMaintenanceInput{
		{Expected: strings.Repeat("a", 64), PolicyRevision: input.PolicyRevision, Endpoints: input.Endpoints},
		{Expected: input.Expected, PolicyRevision: strings.Repeat("b", 64), Endpoints: input.Endpoints},
		{Expected: input.Expected, PolicyRevision: input.PolicyRevision, Endpoints: append(append([]string{}, input.Endpoints...), input.Endpoints[0])},
	} {
		response := endpointManagementRequest(s, "", "/configuration/retire-decisions", changed)
		if response.Code != 409 {
			t.Fatal("stale or ambiguous review accepted", response.Code, response.Body.String())
		}
	}
	if _, err := os.Stat(endpointPolicyHistoryPath(s.config.DataDir, endpointPolicyHistoryDigest(before))); !os.IsNotExist(err) {
		t.Fatal("rejected review wrote history", err)
	}
}

func TestEndpointPolicyMaintenanceRechecksChangesDuringArchival(t *testing.T) {
	for _, change := range []string{"cancel", "configuration", "epoch", "policy"} {
		t.Run(change, func(t *testing.T) {
			s, entry, before := endpointPolicyMaintenanceFixture(t, 1)
			view := endpointOwnerSnapshotFixture(t, s, "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			policyPath := filepath.Join(s.config.DataDir, endpointPolicyFilename)
			expected, err := os.ReadFile(policyPath)
			if err != nil {
				t.Fatal(err)
			}
			s.endpointOwner.write = func(path string, value any) error {
				if err := storage.WriteDurableJSON(path, value); err != nil {
					return err
				}
				switch change {
				case "cancel":
					cancel()
				case "configuration":
					raw, err := os.ReadFile(s.endpointConfigurationPath())
					if err != nil {
						return err
					}
					return os.WriteFile(s.endpointConfigurationPath(), append(raw, '\n'), 0600)
				case "epoch":
					return os.WriteFile(filepath.Join(s.config.DataDir, storage.RestoreEpochFilename), []byte(strings.Repeat("f", 64)), 0600)
				case "policy":
					// Even a change that keeps the compare-and-swap token must not
					// be overwritten by a stale pre-archive in-memory snapshot.
					changed := before
					changed.Decisions = make(map[string]endpointDecision, len(before.Decisions))
					for key, decision := range before.Decisions {
						changed.Decisions[key] = decision
					}
					decision := changed.Decisions[endpointKey(entry)]
					decision.Enabled = false
					changed.Decisions[endpointKey(entry)] = decision
					if err := storage.WriteDurableJSON(policyPath, changed); err != nil {
						return err
					}
					expected, err = os.ReadFile(policyPath)
					return err
				}
				return nil
			}
			request := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
			_, err = s.retireEndpointDecisions(request, endpointPolicyMaintenanceInput{Expected: view.FileRevision, PolicyRevision: view.PolicyMaintenance.Revision, Endpoints: view.PolicyMaintenance.Retirable})
			if err == nil {
				t.Fatal("changed review was published")
			}
			actual, err := os.ReadFile(policyPath)
			if err != nil || string(actual) != string(expected) {
				t.Fatal("maintenance overwrote live policy", err)
			}
			history, err := readEndpointPolicyHistory(s.config.DataDir, endpointPolicyHistoryDigest(before))
			if err != nil || !reflect.DeepEqual(history.Snapshot, before) {
				t.Fatal("original audit lost", err)
			}
		})
	}
}

func TestEndpointPolicyMaintenanceHistorySurvivesFreshEpochWithoutRestoringGrants(t *testing.T) {
	s, entry, _ := endpointPolicyMaintenanceFixture(t, 1)
	result := retireEndpointPolicyFixture(t, s)
	if err := os.WriteFile(filepath.Join(s.config.DataDir, storage.RestoreEpochFilename), []byte(strings.Repeat("f", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	if s.endpointPolicy.allowed(entry) == nil {
		t.Fatal("historical policy survived a fresh restore epoch")
	}
	reviewEndpointFixture(t, s, entry)
	raw, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
	var after endpointPolicySnapshot
	if err != nil || json.Unmarshal(raw, &after) != nil || after.PreviousSnapshot != result.History || len(after.Decisions) != 1 {
		t.Fatal("fresh review lost history or restored stale decisions", err)
	}
	if err := s.endpointPolicy.allowed(entry); err != nil {
		t.Fatal("fresh access review did not enable current identity", err)
	}
}
