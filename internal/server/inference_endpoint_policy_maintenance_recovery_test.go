package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestEndpointPolicyMaintenanceAcrossSixtySixReviewedPublications(t *testing.T) {
	s := endpointOwnerFixture(t)
	archived, retired := 0, 0
	for index := 0; index < 66; index++ {
		view := endpointOwnerSnapshotFixture(t, s, "")
		if len(view.Records) == 64 {
			archived += archiveEndpointOwnerFixture(t, s).Archived
		}
		if view.PolicyMaintenance != nil && view.PolicyMaintenance.Count == 64 {
			retired += len(retireEndpointPolicyFixture(t, s).Retired)
		}
		input := endpointOwnerInputFixture()
		input.Revision = fmt.Sprintf("r%d", index)
		if index > 0 {
			input.Replace = fmt.Sprintf("%s@r%d", input.ID, index-1)
		}
		review := prepareEndpointOwnerFixture(t, s, "", input)
		requestID := fmt.Sprintf("reviewed-publication-%d", index)
		if response := commitEndpointOwnerFixture(s, "", review, requestID); response.Code != 200 {
			t.Fatal(index, response.Code, response.Body.String())
		}
		reconcileEndpointOwnerFixture(t, s, "", requestID)
		s = restartEndpointOwnerFixture(t, s.config.DataDir)
		entry := s.installedEndpoints[0]
		if err := s.endpointPolicy.allowed(entry); err == nil {
			t.Fatal("publication or maintenance activated revision", index)
		}
		reviewEndpointFixture(t, s, entry)
		if err := s.endpointPolicy.allowed(entry); err != nil {
			t.Fatal("fresh explicit review did not activate revision", index, err)
		}
	}
	if archived != 64 || retired != 63 {
		t.Fatal("journey did not exercise both capacity limits", archived, retired)
	}
	state, _, err := s.endpointPolicy.readLocked()
	if err != nil || state.PreviousSnapshot == "" || len(state.Decisions) != 3 {
		t.Fatal("retirement history or current decisions incorrect", state, err)
	}
	history, err := readEndpointPolicyHistory(s.config.DataDir, state.PreviousSnapshot)
	if err != nil || len(history.Snapshot.Decisions) != 64 {
		t.Fatal("full prior policy was not retained", err)
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil || len(journal.Records) != 2 {
		t.Fatal("active journal is incorrect", journal, err)
	}
	first, err := readArchivedEndpointOwnerRecord(s.config.DataDir, journal.ArchiveID, "reviewed-publication-0")
	if err != nil || first == nil || first.State != "committed" {
		t.Fatal("first outcome lost after maintenance", first, err)
	}
	if receipts := allEndpointOwnerHistoryFixture(t, s); len(receipts) != 64 {
		t.Fatal("real publication history cannot be browsed completely", len(receipts))
	}
}

func TestEndpointPolicyMaintenanceCrashHelper(t *testing.T) {
	directory := os.Getenv("OFFGRID_POLICY_MAINTENANCE_CRASH_DIRECTORY")
	if directory == "" {
		return
	}
	s := restartEndpointOwnerFixture(t, directory)
	phase := os.Getenv("OFFGRID_POLICY_MAINTENANCE_CRASH_PHASE")
	write := func(path string, value any) error {
		if err := storage.WriteDurableJSON(path, value); err != nil {
			return err
		}
		if phase == "archive" && filepath.Base(path) != endpointPolicyFilename || phase == "policy" && filepath.Base(path) == endpointPolicyFilename {
			os.Exit(79)
		}
		return nil
	}
	s.endpointOwner.write, s.endpointPolicy.writeSnapshot = write, write
	retireEndpointPolicyFixture(t, s)
	t.Fatal("crash checkpoint was not reached")
}

func TestEndpointPolicyMaintenanceSurvivesProcessExit(t *testing.T) {
	for _, phase := range []string{"archive", "policy"} {
		t.Run(phase, func(t *testing.T) {
			s, entry, before := endpointPolicyMaintenanceFixture(t, 1)
			child := exec.Command(os.Args[0], "-test.run=^TestEndpointPolicyMaintenanceCrashHelper$")
			child.Env = append(os.Environ(), "OFFGRID_POLICY_MAINTENANCE_CRASH_DIRECTORY="+s.config.DataDir, "OFFGRID_POLICY_MAINTENANCE_CRASH_PHASE="+phase)
			output, err := child.CombinedOutput()
			if err == nil || child.ProcessState.ExitCode() != 79 {
				t.Fatalf("helper did not exit at %s: %v %s", phase, err, output)
			}
			s = restartEndpointOwnerFixture(t, s.config.DataDir)
			state, _, err := s.endpointPolicy.readLocked()
			if err != nil {
				t.Fatal(err)
			}
			if phase == "archive" && !reflect.DeepEqual(state, before) || phase == "policy" && len(state.Decisions) != 1 {
				t.Fatal("startup replayed or lost maintenance", state)
			}
			history, err := readEndpointPolicyHistory(s.config.DataDir, endpointPolicyHistoryDigest(before))
			if err != nil || !reflect.DeepEqual(history.Snapshot, before) {
				t.Fatal("crash lost audit snapshot", err)
			}
			if state.Decisions[endpointKey(entry)] != before.Decisions[endpointKey(entry)] {
				t.Fatal("crash changed current grant")
			}
		})
	}
}
