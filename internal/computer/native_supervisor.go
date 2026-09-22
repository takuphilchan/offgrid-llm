package computer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// NativeJournal is host-local dispatch evidence, NOT a second task store. The
// service's agent checkpoint remains authoritative for task history. Never roll
// this journal back with a workspace backup. Unacknowledged claims cannot replay.
type NativeJournal struct {
	db    *sql.DB
	owner *storage.Ownership
}

func OpenNativeJournal(directory string) (*NativeJournal, error) {
	owner, err := storage.AcquireOwnership(directory)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "native-dispatch.sqlite")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		owner.Close()
		return nil, ErrControlScope
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		owner.Close()
		return nil, err
	}
	err = file.Chmod(0600)
	file.Close()
	if err != nil {
		owner.Close()
		return nil, err
	}
	db, err := storage.OpenSQLite(path)
	if err != nil {
		owner.Close()
		return nil, err
	}
	fail := func(err error) (*NativeJournal, error) { db.Close(); owner.Close(); return nil, err }
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version != 0 && version != 1 {
		return fail(fmt.Errorf("computer_journal_incompatible"))
	}
	if version == 0 {
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		_, e = tx.Exec(`CREATE TABLE native_actions(id TEXT PRIMARY KEY, digest TEXT NOT NULL, outcome TEXT NOT NULL CHECK(outcome IN ('executing','verified','dispatched','failed','uncertain')), result BLOB); PRAGMA user_version=1`)
		if e != nil {
			tx.Rollback()
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
	}
	var integrity string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fail(fmt.Errorf("computer_journal_corrupt"))
	}
	return &NativeJournal{db: db, owner: owner}, nil
}
func (j *NativeJournal) Close() error {
	err := j.db.Close()
	e := j.owner.Close()
	if err != nil {
		return err
	}
	return e
}
func actionDigest(binding ControlBinding, action PreparedControlAction) string {
	data, _ := json.Marshal([]any{binding, action})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (j *NativeJournal) claim(ctx context.Context, binding ControlBinding, action PreparedControlAction) (*ControlResult, error) {
	digest := actionDigest(binding, action)
	result, err := j.db.ExecContext(ctx, `INSERT INTO native_actions(id,digest,outcome) VALUES(?,?,'executing') ON CONFLICT(id) DO NOTHING`, action.ID, digest)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count == 1 {
		return nil, nil
	}
	var savedDigest, state string
	var data []byte
	if err = j.db.QueryRowContext(ctx, "SELECT digest,outcome,result FROM native_actions WHERE id=?", action.ID).Scan(&savedDigest, &state, &data); err != nil {
		return nil, err
	}
	if savedDigest != digest {
		return nil, ErrControlApproval
	}
	if state == "executing" || state == "uncertain" || len(data) == 0 {
		return nil, ErrUncertain
	}
	var saved ControlResult
	if json.Unmarshal(data, &saved) != nil || saved.ActionID != action.ID || saved.Outcome != state {
		return nil, fmt.Errorf("computer_journal_corrupt")
	}
	return &saved, nil
}
func (j *NativeJournal) complete(ctx context.Context, result ControlResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	update, err := j.db.ExecContext(ctx, "UPDATE native_actions SET outcome=?,result=? WHERE id=? AND outcome='executing'", result.Outcome, data, result.ActionID)
	if err != nil {
		return err
	}
	count, err := update.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrUncertain
	}
	return nil
}

// NativeSupervisor accepts only grants issued by its local consent callback,
// not grant-shaped JSON from a renderer, model or service. Each approval covers
// a closed, exact bounded step. Stop revokes admission before waiting for any
// current dispatch to settle. The parent must kill a hung owned worker and
// report uncertainty rather than returning a false stop acknowledgement.
type NativeSupervisor struct {
	mu        sync.Mutex
	journal   *NativeJournal
	consent   LocalConsent
	grants    map[string]StepGrant
	ctx       context.Context
	cancel    context.CancelFunc
	dispatch  func(context.Context, PreparedControlAction) (ControlResult, error)
	remaining int
}

func NewNativeSupervisor(journal *NativeJournal, consent LocalConsent, dispatch func(context.Context, PreparedControlAction) (ControlResult, error)) *NativeSupervisor {
	if !consent.ApprovalMode.Valid() {
		consent.ApprovalMode = ApprovalAskEveryTime
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &NativeSupervisor{journal: journal, consent: consent, dispatch: dispatch, grants: map[string]StepGrant{}, ctx: ctx, cancel: cancel, remaining: 100}
}

// Revoke closes admission immediately, including while the provider or a local
// consent dialog is blocked. It is not a stop acknowledgement; Stop must still
// settle the dispatch, or the owning process must be terminated as uncertain.
func (s *NativeSupervisor) Revoke() { s.cancel() }
func (s *NativeSupervisor) Check() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || !s.consent.Active || !time.Now().Before(s.consent.ExpiresAt) || s.remaining <= 0 {
		return ErrControlScope
	}
	return nil
}
func (s *NativeSupervisor) Approve(step BoundedStep, confirm func(BoundedStep) bool) (StepGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	digest, err := step.Digest()
	if err != nil {
		return StepGrant{}, err
	}
	if s.ctx.Err() != nil || !s.consent.Active || s.consent.Binding != step.Binding || !now.Before(s.consent.ExpiresAt) {
		return StepGrant{}, ErrControlScope
	}
	for id, grant := range s.grants {
		if !now.Before(grant.ExpiresAt) {
			delete(s.grants, id)
		}
	}
	if len(s.grants) >= 32 {
		return StepGrant{}, ErrControlApproval
	}
	if !confirm(step) || s.ctx.Err() != nil {
		return StepGrant{}, ErrControlApproval
	}
	now = time.Now()
	expiry := now.Add(5 * time.Minute)
	if s.consent.ExpiresAt.Before(expiry) {
		expiry = s.consent.ExpiresAt
	}
	grant := StepGrant{ID: randomID(), ConsentID: s.consent.ID, Binding: step.Binding, StepDigest: digest, IssuedAt: now, ExpiresAt: expiry}
	if err := grant.Validate(step, s.consent, now); err != nil {
		return StepGrant{}, err
	}
	s.grants[grant.ID] = grant
	return grant, nil
}
func (s *NativeSupervisor) Execute(step BoundedStep, grantID string) ([]ControlResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return nil, ErrControlApproval
	}
	delete(s.grants, grantID)
	if s.ctx.Err() != nil {
		return nil, ErrControlScope
	}
	if err := grant.Validate(step, s.consent, time.Now()); err != nil {
		return nil, err
	}
	results := []ControlResult{}
	for _, action := range step.Actions {
		if s.ctx.Err() != nil || s.remaining <= 0 {
			return results, ErrControlScope
		}
		if err := grant.Validate(step, s.consent, time.Now()); err != nil {
			return results, err
		}
		saved, err := s.journal.claim(s.ctx, step.Binding, action)
		if err != nil {
			return results, err
		}
		if saved != nil {
			results = append(results, *saved)
			if saved.Uncertain || saved.Outcome == "uncertain" {
				return results, ErrUncertain
			}
			if saved.Outcome == "failed" {
				return results, fmt.Errorf("computer_action_failed")
			}
			continue
		}
		s.remaining--
		result, dispatchErr := s.dispatch(s.ctx, action)
		if result.ActionID != action.ID || !validNativeOutcome(result.Outcome) {
			result = ControlResult{ActionID: action.ID, Outcome: "uncertain", Uncertain: true}
			dispatchErr = ErrUncertain
		}
		// Persist the outcome even after cancellation. Failure to commit leaves the
		// original executing claim intact; do not acknowledge successful input.
		if err := s.journal.complete(context.Background(), result); err != nil {
			return results, ErrUncertain
		}
		results = append(results, result)
		if dispatchErr != nil {
			return results, dispatchErr
		}
		if result.Uncertain || result.Outcome == "uncertain" {
			return results, ErrUncertain
		}
		if result.Outcome == "failed" {
			return results, fmt.Errorf("computer_action_failed")
		}
	}
	return results, nil
}
func validNativeOutcome(outcome string) bool {
	return outcome == "verified" || outcome == "dispatched" || outcome == "failed" || outcome == "uncertain"
}
func (s *NativeSupervisor) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consent.Active = false
	s.grants = map[string]StepGrant{}
}
