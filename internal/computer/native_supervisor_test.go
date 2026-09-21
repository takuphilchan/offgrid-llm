package computer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func nativeSupervisorFixture(t *testing.T, dispatch func(context.Context, PreparedControlAction) (ControlResult, error)) (*NativeSupervisor, BoundedStep, *NativeJournal) {
	t.Helper()
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { journal.Close() })
	step, _, consent, _ := controlFixture()
	now := time.Now()
	consent.IssuedAt = now.Add(-time.Second)
	consent.ExpiresAt = now.Add(time.Minute)
	supervisor := NewNativeSupervisor(journal, consent, dispatch)
	t.Cleanup(supervisor.Stop)
	return supervisor, step, journal
}

func TestNativeSupervisorFailedOutcomeIsNotSuccessfulDispatch(t *testing.T) {
	supervisor, step, _ := nativeSupervisorFixture(t, func(_ context.Context, action PreparedControlAction) (ControlResult, error) {
		return ControlResult{ActionID: action.ID, Outcome: "failed"}, nil
	})
	grant, err := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	results, err := supervisor.Execute(step, grant.ID)
	if err == nil || err.Error() != "computer_action_failed" || len(results) != 1 || results[0].Outcome != "failed" {
		t.Fatal("failed outcome reported as successful dispatch", results, err)
	}
}
func TestNativeSupervisorRequiresLocalExactConsentAndPersistsResults(t *testing.T) {
	effects := 0
	supervisor, step, journal := nativeSupervisorFixture(t, func(_ context.Context, a PreparedControlAction) (ControlResult, error) {
		effects++
		return ControlResult{ActionID: a.ID, Outcome: "verified", EvidenceID: "evidence"}, nil
	})
	if _, err := supervisor.Execute(step, "forged"); !errors.Is(err, ErrControlApproval) {
		t.Fatal(err)
	}
	if _, err := supervisor.Approve(step, func(BoundedStep) bool { return false }); !errors.Is(err, ErrControlApproval) {
		t.Fatal(err)
	}
	grant, err := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	changed := step
	changed.Binding.Actor = "other"
	if _, err := supervisor.Execute(changed, grant.ID); err == nil {
		t.Fatal("changed actor accepted")
	}
	grant, err = supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	results, err := supervisor.Execute(step, grant.ID)
	if err != nil || len(results) != 1 || results[0].Outcome != "verified" {
		t.Fatal(results, err)
	}
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("consumed grant reused")
	}
	grant, _ = supervisor.Approve(step, func(BoundedStep) bool { return true })
	if _, err := supervisor.Execute(step, grant.ID); err != nil {
		t.Fatal(err)
	}
	if effects != 1 {
		t.Fatalf("duplicate side effects: %d", effects)
	}
	var state string
	if err := journal.db.QueryRow("SELECT outcome FROM native_actions WHERE id=?", step.Actions[0].ID).Scan(&state); err != nil || state != "verified" {
		t.Fatal(state, err)
	}
}
func TestNativeSupervisorUncertainClaimSurvivesRestartAndRejectsChangedArguments(t *testing.T) {
	root := t.TempDir()
	journal, err := OpenNativeJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	step, _, consent, _ := controlFixture()
	consent.IssuedAt = time.Now().Add(-time.Second)
	consent.ExpiresAt = time.Now().Add(time.Minute)
	if _, err := journal.claim(context.Background(), step.Binding, step.Actions[0]); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	journal, err = OpenNativeJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	supervisor := NewNativeSupervisor(journal, consent, func(context.Context, PreparedControlAction) (ControlResult, error) {
		t.Fatal("uncertain action repeated after reopening journal")
		return ControlResult{}, nil
	})
	defer supervisor.Stop()
	grant, _ := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if _, err := supervisor.Execute(step, grant.ID); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	changed := step.Actions[0]
	changed.Operation.Kind = "activate"
	changed.Operation.Text = nil
	if _, err := journal.claim(context.Background(), step.Binding, changed); !errors.Is(err, ErrControlApproval) {
		t.Fatal(err)
	}
}
func TestNativeSupervisorStopRevokesBeforeAcknowledgement(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var effects atomic.Int32
	supervisor, step, _ := nativeSupervisorFixture(t, func(_ context.Context, a PreparedControlAction) (ControlResult, error) {
		effects.Add(1)
		close(entered)
		<-release
		return ControlResult{ActionID: a.ID, Outcome: "dispatched"}, nil
	})
	grant, _ := supervisor.Approve(step, func(BoundedStep) bool { return true })
	finished := make(chan struct{})
	go func() { supervisor.Execute(step, grant.ID); close(finished) }()
	<-entered
	stopped := make(chan struct{})
	go func() { supervisor.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("stop falsely acknowledged before input settled")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-finished
	<-stopped
	if err := supervisor.Check(); !errors.Is(err, ErrControlScope) {
		t.Fatal(err)
	}
	if _, err := supervisor.Approve(step, func(BoundedStep) bool { t.Fatal("prompt after stop"); return true }); err == nil {
		t.Fatal("stopped scope approved")
	}
	if effects.Load() != 1 {
		t.Fatal(effects.Load())
	}
}
func TestNativeJournalRefusesConcurrentOwnerAndFutureSchema(t *testing.T) {
	root := t.TempDir()
	journal, err := OpenNativeJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := OpenNativeJournal(root); err == nil {
		second.Close()
		t.Fatal("second journal owner")
	}
	journal.db.Exec("PRAGMA user_version=999")
	journal.Close()
	if newer, err := OpenNativeJournal(root); err == nil {
		newer.Close()
		t.Fatal("newer journal accepted")
	}
}
func TestNativeSupervisorStorageFailureCannotExecute(t *testing.T) {
	supervisor, step, journal := nativeSupervisorFixture(t, func(context.Context, PreparedControlAction) (ControlResult, error) {
		t.Fatal("input after storage failure")
		return ControlResult{}, nil
	})
	grant, _ := supervisor.Approve(step, func(BoundedStep) bool { return true })
	journal.db.Close()
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("closed journal accepted")
	}
}

func TestNativeSupervisorFailedResultCannotReplayAsSuccess(t *testing.T) {
	effects := 0
	supervisor, step, _ := nativeSupervisorFixture(t, func(_ context.Context, a PreparedControlAction) (ControlResult, error) {
		effects++
		return ControlResult{ActionID: a.ID, Outcome: "failed"}, errors.New("provider denied")
	})
	grant, _ := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("failed action succeeded")
	}
	grant, _ = supervisor.Approve(step, func(BoundedStep) bool { return true })
	if _, err := supervisor.Execute(step, grant.ID); err == nil {
		t.Fatal("replayed failure became success")
	}
	if effects != 1 {
		t.Fatal("failed action automatically repeated")
	}
}

func TestNativeSupervisorLostResultPersistenceRemainsUncertain(t *testing.T) {
	var journal *NativeJournal
	supervisor, step, opened := nativeSupervisorFixture(t, func(_ context.Context, a PreparedControlAction) (ControlResult, error) {
		journal.db.Close()
		return ControlResult{ActionID: a.ID, Outcome: "verified"}, nil
	})
	journal = opened
	grant, _ := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if _, err := supervisor.Execute(step, grant.ID); !errors.Is(err, ErrUncertain) {
		t.Fatalf("result commit failure falsely acknowledged: %v", err)
	}
}

func TestNativeJournalCompletedResultSurvivesReopen(t *testing.T) {
	root := t.TempDir()
	journal, err := OpenNativeJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	step, _, _, _ := controlFixture()
	if _, err := journal.claim(context.Background(), step.Binding, step.Actions[0]); err != nil {
		t.Fatal(err)
	}
	expected := ControlResult{ActionID: step.Actions[0].ID, Outcome: "verified", EvidenceID: nativeTestDigest()}
	if err := journal.complete(context.Background(), expected); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	journal, err = OpenNativeJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	saved, err := journal.claim(context.Background(), step.Binding, step.Actions[0])
	if err != nil || saved == nil || *saved != expected {
		t.Fatal("completed result lost after reopen", err)
	}
}

func nativeTestDigest() string {
	return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
