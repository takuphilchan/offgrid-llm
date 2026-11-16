package agents

import (
	"errors"
	"testing"
	"time"
)

func deletionFixture(t *testing.T) (*Manager, *AgentProfile, *Task) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	p, err := m.SaveProfile("workspace", "alice", "", 0, ProfileSettings{Name: "Writer", Model: "local", MaxIterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	task := &Task{ID: "delete-fixture", Actor: "alice", Status: TaskCompleted, Prompt: "private prompt", Result: "private result", CreatedAt: time.Now().UTC(), Config: AgentConfig{Profile: p}}
	if err = m.saveTask(task); err != nil {
		t.Fatal(err)
	}
	m.tasks[task.ID] = task
	return m, p, task
}

func TestProfileDeletionReviewIsolationRestartAndStaleAdmission(t *testing.T) {
	m, p, task := deletionFixture(t)
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO agent_memory_entries(id,workspace,actor,profile,revision,label,content,source_job,updated_at) VALUES('memory','workspace','alice',?,1,'private','secret',?,'now')`, p.ID, task.ID)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range [][2]string{{"workspace", "bob"}, {"elsewhere", "alice"}} {
		if _, err = m.ReviewProfileDeletion(owner[0], owner[1], p.ID); !errors.Is(err, ErrProfileNotFound) {
			t.Fatal("foreign review", err)
		}
	}
	r, err := m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil || r.Jobs != 1 || r.Memory != 1 || r.BlockedJobs != 0 || r.Token == "" {
		t.Fatal(r, err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, "stale"); !errors.Is(err, ErrProfileConflict) {
		t.Fatal(err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if _, err = restored.GetProfile("workspace", "alice", p.ID, 1); !errors.Is(err, ErrProfileNotFound) {
		t.Fatal(err)
	}
	if !restored.IsDeleted(task.ID) {
		t.Fatal("task revived")
	}
	if err = restored.saveTask(task); !errors.Is(err, ErrProfileNotFound) {
		t.Fatal("stale task resurrected profile", err)
	}
	task.ID = "late-admission"
	if err = restored.saveTask(task); !errors.Is(err, ErrProfileNotFound) {
		t.Fatal("late task admitted", err)
	}
	db, err = restored.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"agent_profile_revisions", "agent_memory_entries", "agent_task_events"} {
		var n int
		if err = db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatal(table, n, err)
		}
	}
}

func TestProfileDeletionReviewDetectsMemoryReplacement(t *testing.T) {
	m, p, task := deletionFixture(t)
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO agent_memory_entries(id,workspace,actor,profile,revision,label,content,source_job,updated_at) VALUES('old','workspace','alice',?,1,'private','secret',?,'now')`, p.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	review, err := m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Same count and revision total, but a different reviewed record.
	if _, err = db.Exec(`UPDATE agent_memory_entries SET id='new' WHERE id='old'`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, review.Revision, review.Token); !errors.Is(err, ErrProfileConflict) {
		t.Fatalf("stale memory review accepted: %v", err)
	}
	if _, err = m.GetProfile("workspace", "alice", p.ID, 0); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_memory_entries WHERE id='new'`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal(remaining, err)
	}
}

func TestProfileDeletionChangedReviewAndLinkedWorkAreAtomic(t *testing.T) {
	m, p, task := deletionFixture(t)
	r, err := m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	task.Status = TaskRunning
	if err = m.saveTask(task); err != nil {
		t.Fatal(err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); !errors.Is(err, ErrProfileConflict) {
		t.Fatal(err)
	}
	r, err = m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil || r.BlockedJobs != 1 {
		t.Fatal(r, err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); !errors.Is(err, ErrProfileDeleteBlocked) {
		t.Fatal(err)
	}
	if _, err = m.GetProfile("workspace", "alice", p.ID, 1); err != nil {
		t.Fatal("partial profile deletion", err)
	}
	if m.IsDeleted(task.ID) {
		t.Fatal("partial job deletion")
	}
}

func TestProfileDeletionDependenciesAndRollback(t *testing.T) {
	m, p, task := deletionFixture(t)
	parent, err := m.SaveProfile("workspace", "alice", "", 0, ProfileSettings{Name: "Coordinator", Model: "local", MaxIterations: 10, Delegates: []string{p.ID}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil || r.Dependencies != 1 {
		t.Fatal(r, err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); !errors.Is(err, ErrProfileDeleteBlocked) {
		t.Fatal(err)
	}
	settings := parent.Settings
	settings.Delegates = nil
	if _, err = m.SaveProfile("workspace", "alice", parent.ID, parent.Revision, settings); err != nil {
		t.Fatal(err)
	}
	r, err = m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.profileDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER prevent_profile_delete BEFORE DELETE ON agent_profiles BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); err == nil {
		t.Fatal("expected rollback")
	}
	if m.IsDeleted(task.ID) {
		t.Fatal("in-memory deletion published before commit")
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if restored.IsDeleted(task.ID) {
		t.Fatal("partial durable deletion")
	}
	if _, err = restored.GetProfile("workspace", "alice", p.ID, 1); err != nil {
		t.Fatal(err)
	}
}

func TestProfileDeletionPinnedCollaboratorBlocksAfterProfileEdit(t *testing.T) {
	m, p, _ := deletionFixture(t)
	parent := &Task{ID: "other-work", Actor: "alice", Status: TaskRunning, CreatedAt: time.Now().UTC(), Config: AgentConfig{DelegateProfiles: map[string]AgentConfig{p.ID: {Profile: p}}}}
	if err := m.saveTask(parent); err != nil {
		t.Fatal(err)
	}
	r, err := m.ReviewProfileDeletion("workspace", "alice", p.ID)
	if err != nil || r.Dependencies != 1 {
		t.Fatal(r, err)
	}
	if _, err = m.DeleteProfile("workspace", "alice", p.ID, r.Revision, r.Token); !errors.Is(err, ErrProfileDeleteBlocked) {
		t.Fatal(err)
	}
}
