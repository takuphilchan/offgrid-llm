package agents

import (
	"errors"
	"sync"
	"testing"
)

func memoryFixture(t *testing.T) (*Manager, *AgentProfile) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	p, err := m.SaveProfile("work", "alice", "", 0, ProfileSettings{Name: "Worker", Model: "local", MaxIterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	return m, p
}

func TestMemoryConsentRevisionScopeAndRevocation(t *testing.T) {
	m, p := memoryFixture(t)
	c, err := m.GetMemoryConsent("work", "alice", p.ID)
	if err != nil || c.Enabled || c.Revision != 0 {
		t.Fatal(c, err)
	}
	if _, err = m.ListMemory("work", "alice", p.ID, 1); !errors.Is(err, ErrMemoryConsent) {
		t.Fatal(err)
	}
	c, err = m.SetMemoryConsent("work", "alice", p.ID, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	m.tasks["root"] = &Task{ID: "root", Actor: "alice", Status: TaskRunning, Config: AgentConfig{Profile: p, MemoryConsentRevision: c.Revision}}
	w := MemoryWrite{Operation: "root:call1", Label: "Preference", Content: "Use concise Unicode — café 日本語", SourceJob: "root"}
	entry, err := m.WriteMemory("work", "alice", p.ID, c.Revision, w)
	if err != nil {
		t.Fatal(err)
	}
	again, err := m.WriteMemory("work", "alice", p.ID, c.Revision, w)
	if err != nil || again.ID != entry.ID || again.Revision != 1 {
		t.Fatal(again, err)
	}
	w.Content = "changed payload"
	if _, err = m.WriteMemory("work", "alice", p.ID, c.Revision, w); !errors.Is(err, ErrContextConflict) {
		t.Fatal(err)
	}
	for _, scope := range [][2]string{{"work", "bob"}, {"other", "alice"}} {
		if _, err = m.GetMemoryConsent(scope[0], scope[1], p.ID); !errors.Is(err, ErrContextNotFound) {
			t.Fatal("foreign consent", err)
		}
		if _, err = m.ListMemory(scope[0], scope[1], p.ID, 0); !errors.Is(err, ErrContextNotFound) {
			t.Fatal("foreign entries", err)
		}
		if err = m.DeleteMemory(scope[0], scope[1], p.ID, entry.ID, 1); !errors.Is(err, ErrContextConflict) {
			t.Fatal("foreign delete", err)
		}
	}
	c, err = m.SetMemoryConsent("work", "alice", p.ID, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.ListMemory("work", "alice", p.ID, 1); !errors.Is(err, ErrMemoryConsent) {
		t.Fatal("revoked read", err)
	}
	if _, err = m.WriteMemory("work", "alice", p.ID, 1, w); !errors.Is(err, ErrMemoryConsent) {
		t.Fatal("revoked write", err)
	}
	if _, err = m.SetMemoryConsent("work", "alice", p.ID, c.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ListMemory("work", "alice", p.ID, 1); !errors.Is(err, ErrMemoryConsent) {
		t.Fatal("old job reauthorized", err)
	}
	entries, err := m.ListMemory("work", "alice", p.ID, 0)
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
}

func TestMemoryConcurrentCASDeleteAndRestart(t *testing.T) {
	m, p := memoryFixture(t)
	write := MemoryWrite{Operation: "user:create", Label: "Preference", Content: "original"}
	entry, err := m.WriteMemory("work", "alice", p.ID, 0, write)
	if err != nil {
		t.Fatal(err)
	}
	other := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, manager := range []*Manager{m, other} {
		wg.Add(1)
		go func(i int, manager *Manager) {
			defer wg.Done()
			w := write
			w.ID = entry.ID
			w.ExpectedRevision = 1
			w.Content = "new"
			if i == 0 {
				w.Operation = "user:first"
			} else {
				w.Operation = "user:second"
			}
			_, e := manager.WriteMemory("work", "alice", p.ID, 0, w)
			errs <- e
		}(i, manager)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, ErrContextConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	if err = m.DeleteMemory("work", "alice", p.ID, entry.ID, 1); !errors.Is(err, ErrContextConflict) {
		t.Fatal("stale delete", err)
	}
	if err = m.DeleteMemory("work", "alice", p.ID, entry.ID, 2); err != nil {
		t.Fatal(err)
	}
	if entries, e := other.ListMemory("work", "alice", p.ID, 0); e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
	if _, err = other.WriteMemory("work", "alice", p.ID, 0, write); !errors.Is(err, ErrContextConflict) {
		t.Fatal("deleted response replayed", err)
	}
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var content string
	var size int
	if err = db.QueryRow(`SELECT content FROM agent_memory_entries WHERE id=?`, entry.ID).Scan(&content); err != nil || content != "" {
		t.Fatal(content, err)
	}
	if err = db.QueryRow(`SELECT sum(length(result)) FROM agent_memory_operations WHERE entry_id=?`, entry.ID).Scan(&size); err != nil || size != 0 {
		t.Fatal(size, err)
	}
}

func TestMemoryConsentConcurrentCASAndArchiveInspection(t *testing.T) {
	m, p := memoryFixture(t)
	other := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, manager := range []*Manager{m, other} {
		wg.Add(1)
		go func(manager *Manager) {
			defer wg.Done()
			_, err := manager.SetMemoryConsent("work", "alice", p.ID, 0, true)
			errs <- err
		}(manager)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrContextConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	entry, err := m.WriteMemory("work", "alice", p.ID, 0, MemoryWrite{Operation: "user:test", Label: "label", Content: "inspect after archive"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ArchiveProfile("work", "alice", p.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ListMemory("work", "alice", p.ID, 1); !errors.Is(err, ErrMemoryConsent) {
		t.Fatal("archived automatic use", err)
	}
	if entries, err := other.ListMemory("work", "alice", p.ID, 0); err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
	if err = other.DeleteMemory("work", "alice", p.ID, entry.ID, 1); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryCommitFailureDoesNotPublishAnEntryOrReceipt(t *testing.T) {
	m, p := memoryFixture(t)
	db, err := m.contextDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER memory_fixture_failure BEFORE INSERT ON agent_memory_entries BEGIN SELECT RAISE(ABORT,'fixture persistence failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.WriteMemory("work", "alice", p.ID, 0, MemoryWrite{Operation: "failure", Label: "label", Content: "must not publish"}); err == nil {
		t.Fatal("write reported success")
	}
	entries, err := m.ListMemory("work", "alice", p.ID, 0)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM agent_memory_operations WHERE profile=?`, p.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
