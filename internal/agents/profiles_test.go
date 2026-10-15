package agents

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestProfilesImmutableRevisionsIsolationArchiveAndRestore(t *testing.T) {
	directory := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	settings := ProfileSettings{Name: "Research", Model: "installed-model", Instructions: "Cite sources.", MaxIterations: 10, Knowledge: []string{"collection"}}
	first, err := m.SaveProfile("workspace", "alice", "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	settings.Instructions = "Use concise citations."
	second, err := m.SaveProfile("workspace", "alice", first.ID, first.Revision, settings)
	if err != nil || second.Revision != 2 || second.Digest == first.Digest {
		t.Fatal(second, err)
	}
	if _, err = m.SaveProfile("workspace", "alice", first.ID, 1, settings); !errors.Is(err, ErrProfileConflict) {
		t.Fatal("lost update", err)
	}
	for _, owner := range [][2]string{{"workspace", "bob"}, {"another", "alice"}} {
		if _, err = m.GetProfile(owner[0], owner[1], first.ID, 1); !errors.Is(err, ErrProfileNotFound) {
			t.Fatal("foreign read", err)
		}
		if _, err = m.SaveProfile(owner[0], owner[1], first.ID, 2, settings); !errors.Is(err, ErrProfileNotFound) {
			t.Fatal("foreign edit", err)
		}
		if err = m.ArchiveProfile(owner[0], owner[1], first.ID, 2); !errors.Is(err, ErrProfileNotFound) {
			t.Fatal("foreign archive", err)
		}
	}
	// Mutating returned memory cannot change a historical snapshot or digest.
	first.Settings.Knowledge[0] = "changed by caller"
	restored := NewManagerWithPersistence(nil, nil, nil, directory)
	history, err := restored.GetProfile("workspace", "alice", first.ID, 1)
	if err != nil || history.Settings.Instructions != "Cite sources." || history.Settings.Knowledge[0] != "collection" {
		t.Fatal(history, err)
	}
	if err = restored.ArchiveProfile("workspace", "alice", first.ID, 1); !errors.Is(err, ErrProfileConflict) {
		t.Fatal(err)
	}
	if err = restored.ArchiveProfile("workspace", "alice", first.ID, 2); err != nil {
		t.Fatal(err)
	}
	active, err := restored.ListProfiles("workspace", "alice", false)
	if err != nil || len(active) != 0 {
		t.Fatal(active, err)
	}
	history, err = restored.GetProfile("workspace", "alice", first.ID, 1)
	if err != nil || !history.Archived {
		t.Fatal(history, err)
	}
	if _, err = restored.SaveProfile("workspace", "alice", first.ID, 2, settings); !errors.Is(err, ErrProfileConflict) {
		t.Fatal("edited archived profile", err)
	}
	all, err := restored.ListProfiles("workspace", "alice", true)
	if err != nil || len(all) != 1 || all[0].Revision != 2 {
		t.Fatal(all, err)
	}
}

func TestConcurrentProfileEditsOnlyOneExpectedRevisionWins(t *testing.T) {
	directory := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	other := NewManagerWithPersistence(nil, nil, nil, directory)
	settings := ProfileSettings{Name: "Agent", Model: "model", MaxIterations: 5}
	p, err := m.SaveProfile("workspace", "alice", "", 0, settings)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, manager := range []*Manager{m, other} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.SaveProfile("workspace", "alice", p.ID, 1, settings)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrProfileConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
}

func TestProfileMigrationAndWorkspaceBackupRetainJobsAndRevisions(t *testing.T) {
	directory := t.TempDir()
	legacyTaskFixture(t, directory, "old.json", []byte(`{"id":"old","prompt":"Existing work","status":"completed","result":"Preserved"}`))
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	if m.StorageError() != nil {
		t.Fatal(m.StorageError())
	}
	p, err := m.SaveProfile("workspace", "alice", "", 0, ProfileSettings{Name: "Writer", Model: "installed", MaxIterations: 10})
	if err != nil {
		t.Fatal(err)
	}
	settings := p.Settings
	settings.Instructions = "Keep the original constraints."
	if _, err = m.SaveProfile("workspace", "alice", p.ID, 1, settings); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "workspace.zip")
	if _, err = storage.BackupWorkspace(context.Background(), directory, archive, "test"); err != nil {
		t.Fatal(err)
	}
	restoredDirectory := filepath.Join(t.TempDir(), "restored")
	if _, err = storage.RestoreWorkspace(context.Background(), archive, restoredDirectory, "test"); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, restoredDirectory)
	job, ok := restored.GetTask("old")
	if !ok || job.Result != "Preserved" || job.Status != TaskCompleted {
		t.Fatal("existing job changed", job)
	}
	for revision := 1; revision <= 2; revision++ {
		profile, err := restored.GetProfile("workspace", "alice", p.ID, revision)
		if err != nil || profile.Revision != revision {
			t.Fatal(profile, err)
		}
	}
}

func TestProfileValidationDoesNotPersistMalformedConfiguration(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	for _, settings := range []ProfileSettings{{Name: "", Model: "model", MaxIterations: 5}, {Name: "Agent", Model: "model", MaxIterations: 51}, {Name: "Agent", Model: "model", MaxIterations: 5, Tools: []ProfileToolReference{{Provider: "mcp:test", Tool: "read", Location: "shell"}}}} {
		if _, err := m.SaveProfile("workspace", "alice", "", 0, settings); !errors.Is(err, ErrInvalidProfile) {
			t.Fatal(err)
		}
	}
	list, err := m.ListProfiles("workspace", "alice", true)
	if err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
}
