package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// This helper writes only the private directory explicitly passed by its parent
// test. Exiting skips Go cleanup and exercises restart from actual stored bytes.
func TestEndpointOwnerArchiveCrashHelper(t *testing.T) {
	directory := os.Getenv("OFFGRID_OWNER_ARCHIVE_CRASH_DIRECTORY")
	if directory == "" {
		return
	}
	phase := os.Getenv("OFFGRID_OWNER_ARCHIVE_CRASH_PHASE")
	s := restartEndpointOwnerFixture(t, directory)
	s.endpointOwner.write = func(path string, value any) error {
		if err := storage.WriteDurableJSON(path, value); err != nil {
			return err
		}
		match := phase == "marker" && filepath.Base(path) == endpointOwnerArchiveMarker || phase == "receipt" && strings.HasPrefix(filepath.Base(path), "inference-endpoint-receipt-") || phase == "journal" && filepath.Base(path) == endpointOwnerJournalFilename
		if match {
			os.Exit(79)
		}
		return nil
	}
	archiveEndpointOwnerFixture(t, s)
	t.Fatal("crash checkpoint was not reached")
}

func TestEndpointOwnerArchiveSurvivesProcessExit(t *testing.T) {
	for _, phase := range []string{"marker", "receipt", "journal"} {
		t.Run(phase, func(t *testing.T) {
			s, review, record := settledEndpointOwnerFixture(t)
			before, err := os.ReadFile(s.endpointConfigurationPath())
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestEndpointOwnerArchiveCrashHelper$")
			child.Env = append(os.Environ(), "OFFGRID_OWNER_ARCHIVE_CRASH_DIRECTORY="+s.config.DataDir, "OFFGRID_OWNER_ARCHIVE_CRASH_PHASE="+phase)
			output, err := child.CombinedOutput()
			if err == nil || child.ProcessState.ExitCode() != 79 {
				t.Fatalf("helper did not crash at %s: %v %s", phase, err, output)
			}
			s = restartEndpointOwnerFixture(t, s.config.DataDir)
			s.endpointOwner.write = func(string, any) error { t.Fatal("restart/duplicate attempted a write"); return nil }
			response := commitEndpointOwnerFixture(s, "", review, record.RequestID)
			var actual endpointOwnerRecord
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &actual) != nil || !reflect.DeepEqual(actual, record) {
				t.Fatal("crash lost historical outcome", response.Code, response.Body.String())
			}
			after, err := os.ReadFile(s.endpointConfigurationPath())
			if err != nil || string(before) != string(after) {
				t.Fatal("archival replayed owner configuration", err)
			}
		})
	}
}
