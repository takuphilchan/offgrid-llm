package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestEndpointMaintenanceWorkspaceBackupRestoresAuditWithoutAuthority(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		name := "plain"
		if encrypted {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			s, entry, _ := endpointPolicyMaintenanceFixture(t, 1)
			before := endpointOwnerSnapshotFixture(t, s, "")
			record := before.Records[0]
			archiveEndpointOwnerFixture(t, s)
			retired := retireEndpointPolicyFixture(t, s)
			files := map[string][]byte{}
			for _, path := range []string{s.endpointConfigurationPath(), filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), filepath.Join(s.config.DataDir, endpointOwnerArchiveMarker), endpointOwnerReceiptPath(s.config.DataDir, record.RequestID), filepath.Join(s.config.DataDir, endpointPolicyFilename), endpointPolicyHistoryPath(s.config.DataDir, retired.History)} {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files[filepath.Base(path)] = raw
			}
			parent := endpointOwnerFixtureDirectory(t)
			archive := filepath.Join(parent, "workspace.backup")
			var write storage.BackupWriteOptions
			var read storage.BackupReadOptions
			if encrypted {
				identity, err := age.GenerateX25519Identity()
				if err != nil {
					t.Fatal(err)
				}
				write.Recipients = []age.Recipient{identity.Recipient()}
				read.Identities, read.RequireEncryption = []age.Identity{identity}, true
			}
			if _, err := storage.BackupWorkspaceWithOptions(context.Background(), s.config.DataDir, archive, "0.4.32", write); err != nil {
				t.Fatal(err)
			}
			// Revoke after the backup. Restoring its earlier enabled snapshot must
			// never bypass the fresh activation epoch or mutate this live source.
			policy, _, err := s.endpointPolicy.readLocked()
			if err != nil {
				t.Fatal(err)
			}
			if err := s.endpointPolicy.update(entry, policy.Revision, "local-admin", false, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			live, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
			if err != nil {
				t.Fatal(err)
			}
			restored := filepath.Join(parent, "restored")
			if _, err := storage.RestoreWorkspaceWithOptions(context.Background(), archive, restored, "0.4.32", read); err != nil {
				t.Fatal(err)
			}
			for name, expected := range files {
				actual, err := os.ReadFile(filepath.Join(restored, name))
				if err != nil || string(actual) != string(expected) {
					t.Fatal("restored audit/configuration bytes changed", name, err)
				}
			}
			next := restartEndpointOwnerFixture(t, restored)
			if next.endpointPolicy.allowed(entry) == nil {
				t.Fatal("restored historical grant became active")
			}
			response := commitEndpointOwnerFixture(next, "", record.ReviewID, record.RequestID)
			var duplicate endpointOwnerRecord
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &duplicate) != nil || !reflect.DeepEqual(record, duplicate) {
				t.Fatal("restored historical duplicate changed", response.Code, response.Body.String())
			}
			input := endpointOwnerInputFixture()
			input.Replace, input.Revision = endpointKey(entry), "restored-new-revision"
			draft := prepareEndpointOwnerFixture(t, next, "", input)
			if response := commitEndpointOwnerFixture(next, "", draft, record.RequestID); response.Code != 409 {
				t.Fatal("restore reused archived request for a different effect", response.Code)
			}
			reviewEndpointFixture(t, next, entry)
			if next.endpointPolicy.allowed(entry) != nil {
				t.Fatal("fresh restored-workspace review was rejected")
			}
			old := entry
			old.Revision = "old-0"
			if next.endpointPolicy.allowed(old) == nil {
				t.Fatal("fresh review revived a retired identity")
			}
			actual, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
			if err != nil || string(actual) != string(live) {
				t.Fatal("restore changed original workspace", err)
			}
		})
	}
}
