package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func endpointOwnerFixture(t *testing.T) *Server {
	t.Helper()
	directory := endpointOwnerFixtureDirectory(t)
	return restartEndpointOwnerFixture(t, directory)
}

func restartEndpointOwnerFixture(t *testing.T, directory string) *Server {
	t.Helper()
	s := &Server{config: &config.Config{DataDir: directory}, providerRegistry: inference.NewProviderRegistry()}
	if err := s.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	return s
}

func endpointOwnerSnapshotFixture(t *testing.T, s *Server, key string) endpointOwnerSnapshot {
	t.Helper()
	// This GET traverses the same live administrator check as the management UI.
	r := endpointManagementRequestMethod(s, key, "/configuration", nil, "GET")
	var state endpointOwnerSnapshot
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil {
		t.Fatal(r.Code, r.Body.String())
	}
	return state
}

func prepareEndpointOwnerFixture(t *testing.T, s *Server, key string, input endpointOwnerInput) string {
	t.Helper()
	state := endpointOwnerSnapshotFixture(t, s, key)
	input.Expected = state.FileRevision
	w := endpointManagementRequest(s, key, "/configuration/prepare", input)
	var result struct {
		ReviewID string `json:"review_id"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.ReviewID) != 64 {
		t.Fatal(w.Code, w.Body.String())
	}
	return result.ReviewID
}

func commitEndpointOwnerFixture(s *Server, key, review, request string) *httptest.ResponseRecorder {
	return endpointManagementRequest(s, key, "/configuration/commit", map[string]string{"review_id": review, "request_id": request})
}

func reconcileEndpointOwnerFixture(t *testing.T, s *Server, key, request string) endpointOwnerRecord {
	t.Helper()
	state := endpointOwnerSnapshotFixture(t, s, key)
	w := endpointManagementRequest(s, key, "/configuration/reconcile", map[string]string{"request_id": request, "expected_revision": state.FileRevision})
	var record endpointOwnerRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &record) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	return record
}

func TestEndpointOwnerCreateUpdateRequiresReconcileRestartAndFreshReview(t *testing.T) {
	s := endpointOwnerFixture(t)
	state := endpointOwnerSnapshotFixture(t, s, "")
	if !state.WriteAvailable || len(state.Entries) != 0 {
		t.Fatal(state)
	}
	for _, name := range []string{"inference-endpoints.json", endpointPolicyFilename, endpointOwnerJournalFilename, storage.RestoreEpochFilename} {
		if _, err := os.Lstat(filepath.Join(s.config.DataDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("status mutated workspace", name, err)
		}
	}
	input := endpointOwnerInputFixture()
	review := prepareEndpointOwnerFixture(t, s, "", input)
	if _, err := os.Stat(s.endpointConfigurationPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation wrote owner file", err)
	}
	w := commitEndpointOwnerFixture(s, "", review, "create-fixture")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	state = endpointOwnerSnapshotFixture(t, s, "")
	if len(state.Entries) != 1 || len(state.Records) != 1 || state.Records[0].State != "published" || state.WriteAvailable {
		t.Fatal(state)
	}
	assertEndpointOwnerRecoveryDiscoverable(t, s)
	if _, _, err := s.selectEndpointChat(context.Background(), input.ID+"@"+input.Revision); err == nil {
		t.Fatal("create installed a route in the running service")
	}
	if replay := commitEndpointOwnerFixture(s, "", review, "create-fixture"); replay.Code != 200 {
		t.Fatal("same request did not return receipt", replay.Code, replay.Body.String())
	}
	if replay := commitEndpointOwnerFixture(s, "", review, "different-request"); replay.Code == 200 {
		t.Fatal("consumed draft was replayed")
	}
	record := reconcileEndpointOwnerFixture(t, s, "", "create-fixture")
	if record.State != "committed" {
		t.Fatal(record)
	}
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	management, err := s.endpointManagementState("local-admin")
	if err != nil || len(management.Items) != 1 || !management.Items[0].ReviewRequired || management.Items[0].ReviewedBy != "" || management.Items[0].Enabled {
		t.Fatal("owner revocation was reported as a human access review", management, err)
	}
	entry, err := input.entry(endpointOwnerHost{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("restart implicitly enabled created route")
	}
	reviewEndpointFixture(t, s, entry)
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err != nil {
		t.Fatal("explicit review did not activate exact configured route", err)
	}
	lease, release, err := s.endpointPolicy.acquire(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	input.Replace = endpointKey(entry)
	input.Revision = "r2"
	review = prepareEndpointOwnerFixture(t, s, "", input)
	w = commitEndpointOwnerFixture(s, "", review, "update-fixture")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if lease.Err() == nil {
		t.Fatal("owner update retained active old endpoint lease")
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("old route survived owner update")
	}
	reconcileEndpointOwnerFixture(t, s, "", "update-fixture")
	s = restartEndpointOwnerFixture(t, s.config.DataDir)
	next, _ := input.entry(endpointOwnerHost{})
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(next)); err == nil {
		t.Fatal("updated route inherited old review")
	}
	reviewEndpointFixture(t, s, next)
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(next)); err != nil {
		t.Fatal(err)
	}
}

func TestEndpointOwnerUncertainPublicationNeverReplaysOnRestart(t *testing.T) {
	for _, boundary := range []string{"config-before", "config-after", "receipt-before", "receipt-after"} {
		t.Run(boundary, func(t *testing.T) {
			s := endpointOwnerFixture(t)
			input := endpointOwnerInputFixture()
			review := prepareEndpointOwnerFixture(t, s, "", input)
			writes := 0
			journalWrites := 0
			s.endpointOwner.write = func(path string, value any) error {
				if path == s.endpointConfigurationPath() {
					writes++
					if boundary == "config-before" {
						return io.ErrUnexpectedEOF
					}
					if err := storage.WriteDurableJSON(path, value); err != nil {
						return err
					}
					if boundary == "config-after" {
						return io.ErrUnexpectedEOF
					}
					return nil
				}
				journalWrites++
				if journalWrites == 2 && boundary == "receipt-before" {
					return io.ErrUnexpectedEOF
				}
				if err := storage.WriteDurableJSON(path, value); err != nil {
					return err
				}
				if journalWrites == 2 && boundary == "receipt-after" {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			w := commitEndpointOwnerFixture(s, "", review, "uncertain-fixture")
			if w.Code != 503 {
				t.Fatal(w.Code, w.Body.String())
			}
			if writes != 1 {
				t.Fatal("owner write was skipped or repeated", writes)
			}
			w = commitEndpointOwnerFixture(s, "", review, "uncertain-fixture")
			if w.Code != 503 || writes != 1 {
				t.Fatal("uncertain commit automatically replayed", w.Code, writes)
			}
			directory := s.config.DataDir
			s = restartEndpointOwnerFixture(t, directory)
			if err := endpointOwnerJournalReady(directory); err == nil {
				t.Fatal("restart forgot unresolved owner intent")
			}
			if _, _, err := s.selectEndpointChat(context.Background(), input.ID+"@"+input.Revision); err == nil {
				t.Fatal("uncertain owner write authorized request")
			}
			record := reconcileEndpointOwnerFixture(t, s, "", "uncertain-fixture")
			wanted := "committed"
			if boundary == "config-before" {
				wanted = "not_applied"
			}
			if record.State != wanted {
				t.Fatal(record)
			}
			if _, _, err := s.selectEndpointChat(context.Background(), input.ID+"@"+input.Revision); err == nil {
				t.Fatal("reconciliation enabled endpoint access")
			}
		})
	}
}

func TestEndpointOwnerStaleDraftAndCurrentAuthorityRejectWithoutPublication(t *testing.T) {
	for _, change := range []string{"file", "expiry", "host", "admin"} {
		t.Run(change, func(t *testing.T) {
			s := endpointOwnerFixture(t)
			input := endpointOwnerInputFixture()
			key := ""
			actor := ""
			if change == "admin" {
				s.config.RequireAuth = true
				s.userStore = users.NewUserStore("")
				admin, created, err := s.userStore.CreateUser("owner-admin", "isolated-owner-password", users.RoleAdmin)
				if err != nil {
					t.Fatal(err)
				}
				key = created
				actor = admin.ID
				input.Actors = []string{actor}
			}
			review := prepareEndpointOwnerFixture(t, s, key, input)
			switch change {
			case "file":
				if err := storage.WriteDurableJSON(s.endpointConfigurationPath(), inferenceEndpointFile{Version: 1, Endpoints: []inferenceEndpointEnrollment{}}); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				draft := s.endpointOwner.drafts[review]
				draft.Expires = time.Now().Add(-time.Minute)
				s.endpointOwner.drafts[review] = draft
			case "host":
				s.config.InferenceEndpointOwnerPolicyFile = filepath.Join(s.config.DataDir, "host-policy.json")
				if err := storage.WriteDurableJSON(s.config.InferenceEndpointOwnerPolicyFile, endpointOwnerHost{Version: 1}); err != nil {
					t.Fatal(err)
				}
			case "admin":
				if err := s.userStore.UpdateUser(actor, map[string]any{"disabled": true}); err != nil {
					t.Fatal(err)
				}
			}
			w := commitEndpointOwnerFixture(s, key, review, "stale-fixture")
			if w.Code == 200 {
				t.Fatal("stale owner review committed", change)
			}
			if _, err := os.Stat(filepath.Join(s.config.DataDir, endpointOwnerJournalFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("stale review wrote durable intent", err)
			}
		})
	}
}
