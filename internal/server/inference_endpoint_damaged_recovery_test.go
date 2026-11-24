package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestEndpointDamagedWorkspaceRecoveryPreservesOriginalAndRequiresReview(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		format := "plain"
		if encrypted {
			format = "encrypted"
		}
		for _, damaged := range []string{"configuration", "journal", "policy", "archive-marker", "receipt", "policy-history"} {
			t.Run(format+"/"+damaged, func(t *testing.T) {
				s, entry, _ := endpointPolicyMaintenanceFixture(t, 1)
				record := endpointOwnerSnapshotFixture(t, s, "").Records[0]
				archiveEndpointOwnerFixture(t, s)
				retired := retireEndpointPolicyFixture(t, s)
				if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err != nil {
					t.Fatal("healthy reviewed fixture was not selectable", err)
				}
				parent := endpointOwnerFixtureDirectory(t)
				backup := filepath.Join(parent, "known-good.backup")
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
				if _, err := storage.BackupWorkspaceWithOptions(context.Background(), s.config.DataDir, backup, "0.4.32", write); err != nil {
					t.Fatal(err)
				}
				paths := map[string]string{"configuration": s.endpointConfigurationPath(), "journal": filepath.Join(s.config.DataDir, endpointOwnerJournalFilename), "policy": filepath.Join(s.config.DataDir, endpointPolicyFilename), "archive-marker": filepath.Join(s.config.DataDir, endpointOwnerArchiveMarker), "receipt": endpointOwnerReceiptPath(s.config.DataDir, record.RequestID), "policy-history": endpointPolicyHistoryPath(s.config.DataDir, retired.History)}
				originals := map[string][]byte{}
				for name, path := range paths {
					raw, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					originals[name] = raw
				}
				broken := []byte("{interrupted-or-damaged-json")
				if err := os.WriteFile(paths[damaged], broken, 0600); err != nil {
					t.Fatal(err)
				}
				// Damage is visible and never silently normalized by browsing.
				action := "/configuration"
				if damaged == "receipt" {
					action = "/configuration/history"
				}
				if damaged == "policy" || damaged == "policy-history" {
					action = "/configuration/policy-history"
				}
				response := endpointManagementRequestMethod(s, "", action, nil, "GET")
				if damaged == "journal" || damaged == "archive-marker" {
					state := endpointOwnerSnapshotFixture(t, s, "")
					if state.WriteAvailable || state.WriteReason == "" {
						t.Fatal("damaged journal retained write authority")
					}
				} else if response.Code == 200 {
					t.Fatal("damaged state presented as readable", damaged, response.Body.String())
				}
				if damaged != "receipt" {
					if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
						t.Fatal("damaged admission metadata preserved endpoint access", damaged)
					}
				}
				restored := filepath.Join(parent, "recovered")
				if _, err := storage.RestoreWorkspaceWithOptions(context.Background(), backup, restored, "0.4.32", read); err != nil {
					t.Fatal(err)
				}
				for name, path := range paths {
					actual, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					expected := originals[name]
					if name == damaged {
						expected = broken
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Fatal("recovery changed damaged original", name)
					}
					recovered, err := os.ReadFile(filepath.Join(restored, filepath.Base(path)))
					if err != nil || !reflect.DeepEqual(recovered, originals[name]) {
						t.Fatal("recovered evidence differs from verified backup", name, err)
					}
				}
				next := restartEndpointOwnerFixture(t, restored)
				if next.endpointPolicy.allowed(entry) == nil {
					t.Fatal("recovery reactivated the archived grant")
				}
				receipts := allEndpointOwnerHistoryFixture(t, next)
				if len(receipts) != 1 || !reflect.DeepEqual(receipts[0].Record, record) {
					t.Fatal("recovery lost original receipt")
				}
				if duplicate := commitEndpointOwnerFixture(next, "", record.ReviewID, record.RequestID); duplicate.Code != 200 {
					t.Fatal("recovered duplicate proof unavailable", duplicate.Code)
				}
				if next.endpointPolicy.allowed(entry) == nil {
					t.Fatal("duplicate receipt activated access")
				}
				reviewEndpointFixture(t, next, entry)
				if next.endpointPolicy.allowed(entry) != nil {
					t.Fatal("fresh recovery review failed")
				}
			})
		}
	}
}
