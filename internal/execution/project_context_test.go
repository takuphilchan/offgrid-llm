package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectHostIdentityPersistsWithoutRegeneratingDamagedState(t *testing.T) {
	root, private := t.TempDir(), filepath.Join(t.TempDir(), "identity")
	one, err := loadProjectHostIdentity(private, root)
	if err != nil || !validCredential(one) {
		t.Fatal(one, err)
	}
	two, err := loadProjectHostIdentity(private, root)
	if err != nil || one != two {
		t.Fatal("identity changed across sessions", err)
	}
	other, err := loadProjectHostIdentity(filepath.Join(t.TempDir(), "identity"), root)
	if err != nil || one == other {
		t.Fatal("distinct host accounts conflated", err)
	}
	if _, err = loadProjectHostIdentity(filepath.Join(root, "private"), root); err == nil {
		t.Fatal("identity placed in model-visible root")
	}
	if err = os.WriteFile(filepath.Join(private, "identity"), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadProjectHostIdentity(private, root); !errors.Is(err, ErrRevision) {
		t.Fatal("damaged identity regenerated", err)
	}
}

func TestProjectContextBoundedReadIdentityAndAuthority(t *testing.T) {
	e, _, b, root := workingCopyFixture(t)
	e.Worker.Session.config.ProjectHostID = strings.Repeat("b", 64)
	ctx := context.Background()
	one, err := e.ProjectContext(ctx, b)
	if err != nil || one.Present || !validCopyID(one.Identity) {
		t.Fatal(one, err)
	}
	if err = os.Mkdir(filepath.Join(root, ".offgrid"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, ".offgrid", "project.toml")
	data := []byte("version = 1\n[preferences]\nmodel = 'local'\n")
	if err = os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	two, err := e.ProjectContext(ctx, b)
	if err != nil || !two.Present || two.Identity != one.Identity || string(two.Configuration) != string(data) || two.SHA256 != digest(data) {
		t.Fatal(two, err)
	}
	private, _ := json.Marshal(e.Worker.Session.config)
	for _, operation := range []string{"workspace_info", "directory_change"} {
		presented, err := PresentReceipt(reference(b), operation, Receipt{State: "complete", Result: private})
		if err != nil || strings.Contains(presented, "project_host_id") || !strings.Contains(string(private), "project_host_id") {
			t.Fatal("presentation leaked identity or rewrote private receipt", err)
		}
	}
	foreign := b
	foreign.Job = "other"
	if err = e.Admit(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ProjectContext(ctx, foreign); !errors.Is(err, ErrUnsupported) {
		t.Fatal("non-supervisor retrieved project metadata", err)
	}
	if _, err = NewIntent(b, "model", "supervisor_project_context", json.RawMessage(`{}`), false); err == nil {
		t.Fatal("private read exposed as a model operation")
	}
	if err = os.WriteFile(file, make([]byte, (64<<10)+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = e.ProjectContext(ctx, b); !errors.Is(err, ErrLimit) {
		t.Fatal("unbounded project read", err)
	}
	e.Worker.Session.Revoke()
	if _, err = e.ProjectContext(ctx, b); !errors.Is(err, ErrAuthority) {
		t.Fatal("revoked read succeeded", err)
	}
}

func TestProjectContextRejectsUnsupportedAndEscapingInput(t *testing.T) {
	e, _, b, root := workingCopyFixture(t)
	if _, err := e.ProjectContext(context.Background(), b); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	e.Worker.Session.config.ProjectHostID = strings.Repeat("b", 64)
	e.Worker.Session.config.Permissions.Read = false
	if _, err := e.ProjectContext(context.Background(), b); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	e.Worker.Session.config.Permissions.Read = true
	if err := os.Mkdir(filepath.Join(root, ".offgrid"), 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.toml")
	if err := os.WriteFile(outside, []byte("version = 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".offgrid", "project.toml")); err != nil {
		t.Skip("symlink creation unavailable:", err)
	}
	if _, err := e.ProjectContext(context.Background(), b); err == nil {
		t.Fatal("followed project metadata symlink")
	}
}

func TestProjectContextCrossesAuthenticatedBridgeWithoutJobAdmission(t *testing.T) {
	enrollment, endpoint, config, pack, token := enrollmentFixture(t)
	config.ProjectHostID = strings.Repeat("b", 64)
	endpoint.Worker.Session.config = config
	ref := onlineEnrollment(t, enrollment, endpoint, config, pack, token)
	serveEnrollmentFixture(t, enrollment, endpoint, config, token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer := enrollment.peers[ref.Session].peer
	observed, err := peer.ProjectContext(ctx, config.Binding)
	if err != nil || !validCopyID(observed.Identity) {
		t.Fatal(observed, err)
	}
	endpoint.Worker.Session.mu.Lock()
	jobs := len(endpoint.Worker.Session.jobs)
	endpoint.Worker.Session.mu.Unlock()
	if jobs != 1 {
		t.Fatal("metadata retrieval admitted work")
	}
	config.ProjectHostID = strings.Repeat("c", 64)
	endpoint.Worker.Session.mu.Lock()
	endpoint.Worker.Session.config = config
	endpoint.Worker.Session.mu.Unlock()
	if _, err = enrollment.hosts.ProjectContext(ctx, config.Binding.Actor, config.Binding.Workspace, ref); err == nil {
		t.Fatal("changed host identity accepted")
	}
}
