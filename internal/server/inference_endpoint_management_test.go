package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func endpointManagementRequest(s *Server, key, action string, body any) *httptest.ResponseRecorder {
	method := http.MethodPost
	if action == "" {
		method = http.MethodGet
	}
	return endpointManagementRequestMethod(s, key, action, body, method)
}

func endpointManagementRequestMethod(s *Server, key, action string, body any, method string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, inferenceEndpointsPath+action, bytes.NewReader(raw))
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(s.config.RequireAuth)
	middleware.Wrap(http.HandlerFunc(s.handleInferenceEndpoints)).ServeHTTP(w, r)
	return w
}

func reviewEndpointFixture(t *testing.T, s *Server, entry inferenceEndpointEnrollment) {
	t.Helper()
	key := ""
	if s.config.RequireAuth {
		_, created, err := s.userStore.CreateUser("endpoint-review-admin", "isolated-review-password", users.RoleAdmin)
		if err != nil {
			t.Fatal(err)
		}
		key = created
	}
	state := endpointSnapshotFixture(t, s, key)
	enabled := true
	w := endpointManagementRequest(s, key, "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &enabled})
	if w.Code != 200 {
		t.Fatal("explicit endpoint review failed", w.Code, w.Body.String())
	}
}

func endpointSnapshotFixture(t *testing.T, s *Server, key string) endpointManagementSnapshot {
	t.Helper()
	w := endpointManagementRequest(s, key, "", nil)
	var snapshot endpointManagementSnapshot
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &snapshot) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	return snapshot
}

func endpointPolicyFixture(t *testing.T) (*Server, inferenceEndpointEnrollment) {
	t.Helper()
	entry := endpointEnrollmentFixture()
	path := filepath.Join(t.TempDir(), "endpoints.json")
	writeEndpointEnrollment(t, path, entry)
	t.Setenv(entry.CredentialEnv, "isolated-management-secret")
	s := &Server{config: &config.Config{InferenceEndpointsFile: path, DataDir: t.TempDir()}, providerRegistry: inference.NewProviderRegistry()}
	if err := s.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	return s, entry
}

func TestEndpointManagementRequiresReviewAndRevokesAdmittedRequests(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	state := endpointSnapshotFixture(t, s, "")
	if state.PolicyState != "review_required" || len(state.Items) != 1 || state.Items[0].Enabled || !state.Items[0].ReviewRequired {
		t.Fatal(state)
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("missing review accepted")
	}
	reviewEndpointFixture(t, s, entry)
	state = endpointSnapshotFixture(t, s, "")
	if !state.Items[0].Enabled || !state.Items[0].ProbeAllowed || state.Items[0].ReviewRequired {
		t.Fatal(state)
	}
	admitted, release, err := s.endpointPolicy.acquire(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// Selection does not grant a future dispatch after a queue wait.
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err != nil {
		t.Fatal(err)
	}
	enabled := false
	body := endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &enabled}
	w := endpointManagementRequest(s, "", "/review", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if admitted.Err() != context.Canceled {
		t.Fatal("admitted request was not canceled")
	}
	if _, _, err := s.endpointPolicy.acquire(context.Background(), entry); err == nil {
		t.Fatal("queued/new request acquired revoked authority")
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("revoked endpoint selected")
	}
	if endpointManagementRequest(s, "", "/review", body).Code != http.StatusConflict {
		t.Fatal("stale review accepted")
	}
	if strings.Contains(w.Body.String(), "isolated-management-secret") || strings.Contains(w.Body.String(), entry.CredentialEnv) {
		t.Fatal("credential authority exposed")
	}
	restarted := &Server{config: s.config, providerRegistry: inference.NewProviderRegistry()}
	if err := restarted.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := restarted.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("restart re-enabled endpoint")
	}
}

func TestEndpointManagementRestoreCannotReplayArchivedEnabledGrant(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	reviewEndpointFixture(t, s, entry)
	old := endpointSnapshotFixture(t, s, "")
	archive := filepath.Join(t.TempDir(), "workspace.zip")
	if _, err := storage.BackupWorkspace(context.Background(), s.config.DataDir, archive, "0.4.3"); err != nil {
		t.Fatal(err)
	}
	no := false
	if w := endpointManagementRequest(s, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: old.PolicyRevision, Enabled: &no}); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	restored := filepath.Join(t.TempDir(), "restored")
	if _, err := storage.RestoreWorkspace(context.Background(), archive, restored, "0.4.3"); err != nil {
		t.Fatal(err)
	}
	copyConfig := *s.config
	copyConfig.DataDir = restored
	next := &Server{config: &copyConfig, providerRegistry: inference.NewProviderRegistry()}
	if err := next.InstallConfiguredHTTPEndpoints(); err != nil {
		t.Fatal(err)
	}
	state := endpointSnapshotFixture(t, next, "")
	if state.PolicyState != "restore_review_required" || state.Items[0].Enabled || state.PolicyRevision == old.PolicyRevision {
		t.Fatal(state)
	}
	if _, _, err := next.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("archived enabled decision reactivated")
	}
	yes := true
	if w := endpointManagementRequest(next, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: old.PolicyRevision, Enabled: &yes}); w.Code != http.StatusConflict {
		t.Fatal("old review replayed", w.Code)
	}
	reviewEndpointFixture(t, next, entry)
	if _, _, err := next.selectEndpointChat(context.Background(), endpointKey(entry)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(restored, storage.RestoreEpochFilename)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := next.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("missing activation marker preserved restored grant")
	}
	if err := os.WriteFile(filepath.Join(restored, storage.RestoreEpochFilename), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := next.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("corrupt restore marker preserved grant")
	}
}

func TestEndpointManagementInitialReviewCreatesDurableEpochAndMissingMarkerCannotMatchArchive(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	state := endpointSnapshotFixture(t, s, "")
	if state.PolicyState != "review_required" {
		t.Fatal(state)
	}
	if _, err := os.Stat(filepath.Join(s.config.DataDir, storage.RestoreEpochFilename)); !os.IsNotExist(err) {
		t.Fatal("read-only status wrote activation identity", err)
	}
	reviewEndpointFixture(t, s, entry)
	epoch, err := storage.ReadRestoreEpoch(s.config.DataDir)
	if err != nil || len(epoch) != 64 {
		t.Fatal(epoch, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
	if err != nil {
		t.Fatal(err)
	}
	var policy endpointPolicySnapshot
	if json.Unmarshal(raw, &policy) != nil || policy.Epoch != epoch {
		t.Fatal("grant lacks nonempty activation binding")
	}
	if err := os.Remove(filepath.Join(s.config.DataDir, storage.RestoreEpochFilename)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("missing marker became empty original epoch")
	}
}

func TestEndpointManagementCorruptionAndChangedConfigFailClosed(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	reviewEndpointFixture(t, s, entry)
	state := endpointSnapshotFixture(t, s, "")
	changed := entry
	changed.URL = "https://changed.example.test"
	writeEndpointEnrollment(t, s.config.InferenceEndpointsFile, changed)
	current := endpointSnapshotFixture(t, s, "")
	if current.Items[0].Installed || current.Items[0].Enabled || !current.Items[0].ReviewRequired {
		t.Fatal(current)
	}
	yes := true
	if w := endpointManagementRequest(s, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(changed), PolicyRevision: state.PolicyRevision, Enabled: &yes}); w.Code != http.StatusConflict {
		t.Fatal("changed installation enabled", w.Code)
	}
	writeEndpointEnrollment(t, s.config.InferenceEndpointsFile, entry)
	if err := os.WriteFile(filepath.Join(s.config.DataDir, endpointPolicyFilename), []byte(`{"version":1,"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
		t.Fatal("corrupt policy accepted")
	}
	if w := endpointManagementRequest(s, "", "", nil); w.Code != http.StatusServiceUnavailable {
		t.Fatal(w.Code)
	}
	if w := endpointManagementRequest(s, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &yes}); w.Code != http.StatusConflict {
		t.Fatal("corrupt policy implicitly repaired", w.Code)
	}
}

func TestEndpointManagementPublicationFailureCancelsAndPoisonsAuthority(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-publication", true: "after-publication"}[published], func(t *testing.T) {
			s, entry := endpointPolicyFixture(t)
			reviewEndpointFixture(t, s, entry)
			state := endpointSnapshotFixture(t, s, "")
			ctx, release, err := s.endpointPolicy.acquire(context.Background(), entry)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			s.endpointPolicy.writeSnapshot = func(path string, value any) error {
				if published {
					if err := storage.WriteDurableJSON(path, value); err != nil {
						return err
					}
				}
				return errors.New("injected publication/durability failure")
			}
			enabled := false
			w := endpointManagementRequest(s, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &enabled})
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "endpoint_policy_publication_uncertain") {
				t.Fatal("failed publication claimed success", w.Code)
			}
			if ctx.Err() != context.Canceled {
				t.Fatal("uncertain disable retained admitted request")
			}
			if _, _, err := s.endpointPolicy.acquire(context.Background(), entry); err == nil {
				t.Fatal("uncertain state admitted a new request")
			}
			s.endpointPolicy.writeSnapshot = nil
			if _, _, err := s.selectEndpointChat(context.Background(), endpointKey(entry)); err == nil {
				t.Fatal("ordinary retry cleared publication uncertainty")
			}
			var disk endpointPolicySnapshot
			raw, err := os.ReadFile(filepath.Join(s.config.DataDir, endpointPolicyFilename))
			if err != nil || json.Unmarshal(raw, &disk) != nil {
				t.Fatal(err)
			}
			if disk.Decisions[endpointKey(entry)].Enabled == published {
				t.Fatal("fixture did not exercise expected publication boundary")
			}
		})
	}
}

func TestEndpointManagementNeverDisplaysCredentialsFromChangedOwnerURL(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	for _, address := range []string{"https://user:changed-secret@inference.example.test", "https://inference.example.test?token=changed-secret", "https://inference.example.test/changed-secret", "https://inference.example.test#changed-secret"} {
		entry.URL = address
		writeEndpointEnrollment(t, s.config.InferenceEndpointsFile, entry)
		response := endpointManagementRequest(s, "", "", nil)
		if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "changed-secret") {
			t.Fatal("changed owner URL leaked unvalidated credentials/path", response.Code, response.Body.String())
		}
	}
}
