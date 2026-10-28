package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func enrollmentFixture(t *testing.T) (*Enrollment, *LocalEndpoint, Context, Pack, string) {
	t.Helper()
	root := t.TempDir()
	b := Binding{Actor: "alice", Workspace: "workspace", Host: "host", Session: "session", Grant: "grant", Revision: 1, Job: BootstrapJob}
	config := Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: Permissions{Read: true, Write: true}}
	session, err := OpenSession(config)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := &Worker{Session: session, Journal: journal, Processes: NewProcessManager(session), Token: strings.Repeat("a", 64)}
	endpoint := NewLocalEndpoint(worker)
	hosts := NewHosts()
	enrollment := NewEnrollment(hosts, "development")
	t.Cleanup(func() { enrollment.Close(); endpoint.Close(); session.Close(); journal.Close() })
	file := "offgrid-executor"
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	pack := Pack{Schema: 1, Protocol: 1, Revision: PackRevision, Version: "development", Platform: runtime.GOOS, Architecture: runtime.GOARCH, File: file, SHA256: strings.Repeat("d", 64)}
	return enrollment, endpoint, config, pack, strings.Repeat("b", 64)
}

func TestEnrollmentRechecksActorAuthorityAndNeverRepublishesRevokedHost(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	ref := onlineEnrollment(t, e, endpoint, config, pack, token)
	e.MayUse = func(actor, workspace string) bool { return false }
	if err := e.Heartbeat(ref.Session, token); err == nil {
		t.Fatal("revoked actor retained authority")
	}
	if len(e.hosts.List(context.Background(), config.Binding.Actor, config.Binding.Workspace)) != 0 {
		t.Fatal("revoked actor still ready")
	}
	if _, err := e.Poll(context.Background(), ref.Session, token); err == nil {
		t.Fatal("removed peer reused")
	}
}

func onlineEnrollment(t *testing.T, e *Enrollment, endpoint HostEndpoint, config Context, pack Pack, token string) Reference {
	t.Helper()
	pair, err := e.Pair(config.Binding.Actor, config.Binding.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := e.Enroll(pair.Code, token, config, pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.hosts.List(context.Background(), pair.Actor, pair.Workspace)) != 0 {
		t.Fatal("enrollment itself granted readiness")
	}
	message, err := e.Poll(context.Background(), ref.Session, token)
	if err != nil || message.Kind != "check" {
		t.Fatal(message, err)
	}
	reply := DispatchBridgeMessage(context.Background(), endpoint, config, message)
	ready, err := e.Reply(context.Background(), ref.Session, token, reply)
	if err != nil || !ready {
		t.Fatal(reply, ready, err)
	}
	if _, err = e.Reply(context.Background(), ref.Session, token, reply); err != nil {
		t.Fatal("identical ack not idempotent", err)
	}
	if len(e.hosts.List(context.Background(), pair.Actor, pair.Workspace)) != 1 {
		t.Fatal("confirmed host not published")
	}
	return ref
}

func serveEnrollmentFixture(t *testing.T, e *Enrollment, endpoint HostEndpoint, config Context, token string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			message, err := e.Poll(ctx, config.Binding.Session, token)
			if err != nil {
				return
			}
			reply := DispatchBridgeMessage(ctx, endpoint, config, message)
			if _, err = e.Reply(ctx, config.Binding.Session, token, reply); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
}

func TestEnrolledHostUsesLocalScopeAndCanonicalJob(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	ref := onlineEnrollment(t, e, endpoint, config, pack, token)
	serveEnrollmentFixture(t, e, endpoint, config, token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.hosts.Admit(ctx, config.Binding.Actor, config.Binding.Workspace, "canonical-job", ref); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"path": "exact.txt", "data": []byte("café 日本語\n1| actual content")})
	response, err := e.hosts.Call(ctx, config.Binding.Actor, config.Binding.Workspace, "canonical-job", "write-once", ref, "file_write", args)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	actual, err := os.ReadFile(filepath.Join(config.Root, "exact.txt"))
	if err != nil || string(actual) != "café 日本語\n1| actual content" {
		t.Fatal(string(actual), err)
	}
	if err = e.RevokeOwner("bob", config.Binding.Workspace, ref); !errors.Is(err, ErrAuthority) {
		t.Fatal("cross-actor revoke", err)
	}
	if err = e.RevokeOwner(config.Binding.Actor, config.Binding.Workspace, ref); err != nil {
		t.Fatal(err)
	}
	if _, err = e.hosts.Resolve(ctx, config.Binding.Actor, config.Binding.Workspace, ref); err == nil {
		t.Fatal("revoked authority survived")
	}
}

func TestEnrollmentRejectsForgedScopeSkewAndWrongTokens(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	pair, err := e.Pair(config.Binding.Actor, config.Binding.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	wrong := config
	wrong.Binding.Actor = "bob"
	if _, err = e.Enroll(pair.Code, token, wrong, pack); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
	badPack := pack
	badPack.Revision = "unknown"
	if _, err = e.Enroll(pair.Code, token, config, badPack); err == nil {
		t.Fatal("pack skew accepted")
	}
	ref, err := e.Enroll(pair.Code, token, config, pack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Enroll(pair.Code, token, config, pack); err == nil {
		t.Fatal("pairing reused")
	}
	badToken := strings.Repeat("f", 64)
	if _, err = e.Poll(context.Background(), ref.Session, badToken); !errors.Is(err, ErrAuthority) {
		t.Fatal("forged poll", err)
	}
	if err = e.Heartbeat(ref.Session, badToken); !errors.Is(err, ErrAuthority) {
		t.Fatal("forged heartbeat", err)
	}
	if err = e.RevokeHost(ref.Session, badToken); !errors.Is(err, ErrAuthority) {
		t.Fatal("forged revoke", err)
	}
	message, err := e.Poll(context.Background(), ref.Session, token)
	if err != nil {
		t.Fatal(err)
	}
	reply := DispatchBridgeMessage(context.Background(), endpoint, config, message)
	changed := *reply.Snapshot
	changed.Permissions.Processes = true
	reply.Snapshot = &changed
	if ready, err := e.Reply(context.Background(), ref.Session, token, reply); ready || !errors.Is(err, ErrAuthority) {
		t.Fatal("claimed expansion became authority", ready, err)
	}
	if len(e.hosts.List(context.Background(), pair.Actor, pair.Workspace)) != 0 {
		t.Fatal("unconfirmed grant visible")
	}
}

func TestLostHostReplyCannotRedeliverOrReplayMutation(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	ref := onlineEnrollment(t, e, endpoint, config, pack, token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	admitted := make(chan error, 1)
	go func() { admitted <- e.hosts.Admit(ctx, config.Binding.Actor, config.Binding.Workspace, "job", ref) }()
	message, err := e.Poll(ctx, ref.Session, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Reply(ctx, ref.Session, token, DispatchBridgeMessage(ctx, endpoint, config, message)); err != nil {
		t.Fatal(err)
	}
	if err = <-admitted; err != nil {
		t.Fatal(err)
	}
	operationctx, stopOperation := context.WithCancel(ctx)
	finished := make(chan error, 1)
	args, _ := json.Marshal(map[string]any{"path": "once.txt", "data": []byte("written once")})
	go func() {
		_, err := e.hosts.Call(operationctx, config.Binding.Actor, config.Binding.Workspace, "job", "mutation", ref, "file_write", args)
		finished <- err
	}()
	message, err = e.Poll(ctx, ref.Session, token)
	if err != nil {
		t.Fatal(err)
	}
	reply := DispatchBridgeMessage(ctx, endpoint, config, message)
	if reply.Error != "" || reply.Response == nil || reply.Response.Receipt.State != "complete" {
		t.Fatal(reply)
	}
	// Simulate HTTP response loss: do not acknowledge the locally persisted result.
	readctx, endRead := context.WithTimeout(ctx, 20*time.Millisecond)
	redelivery, err := e.Poll(readctx, ref.Session, token)
	endRead()
	if !errors.Is(err, context.DeadlineExceeded) || redelivery.Kind != "heartbeat" {
		t.Fatal("operation redelivered", redelivery, err)
	}
	stopOperation()
	if err = <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = e.hosts.Call(ctx, config.Binding.Actor, config.Binding.Workspace, "job", "mutation", ref, "file_write", args); err == nil {
		t.Fatal("closed peer replayed")
	}
	actual, _ := os.ReadFile(filepath.Join(config.Root, "once.txt"))
	if string(actual) != "written once" {
		t.Fatal(string(actual))
	}
}

func TestEnrollmentExpiryRevokesReadinessWithoutRestoringGrants(t *testing.T) {
	e, endpoint, config, pack, token := enrollmentFixture(t)
	now := time.Now()
	e.now = func() time.Time { return now }
	ref := onlineEnrollment(t, e, endpoint, config, pack, token)
	now = now.Add(HostHeartbeatTimeout + time.Second)
	e.Sweep()
	if len(e.hosts.List(context.Background(), config.Binding.Actor, config.Binding.Workspace)) != 0 {
		t.Fatal("expired host listed")
	}
	if err := e.Heartbeat(ref.Session, token); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal(err)
	}
}
