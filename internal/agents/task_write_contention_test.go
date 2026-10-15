package agents

import (
	"testing"
	"time"
)

func TestTaskSnapshotWaitsForWriterBeforeReading(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			if err := m.StorageError(); err != nil {
				t.Fatal(err)
			}
			task := &Task{ID: "contention", Actor: "alice", Status: TaskPending}
			if existing {
				if err := persistTask(dir, task); err != nil {
					t.Fatal(err)
				}
			}
			// Open both connections before holding the competing writer lock.
			db, err := openTaskDatabase(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			other, err := openTaskDatabase(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			writer, err := other.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			if _, err := writer.Exec(`UPDATE agent_tasks SET id=id`); err != nil {
				t.Fatal(err)
			}
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			task.Result = "saved once"
			done := make(chan error, 1)
			go func() { done <- putTaskTransaction(tx, task) }()
			select {
			case err := <-done:
				t.Fatalf("task write did not wait for the existing writer: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			if err := writer.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			// Saving an identical snapshot must still produce no extra event.
			if err := persistTask(dir, task); err != nil {
				t.Fatal(err)
			}
			var events int
			if err := db.QueryRow(`SELECT COUNT(*) FROM agent_task_events WHERE task_id=?`, task.ID).Scan(&events); err != nil {
				t.Fatal(err)
			}
			want := 1
			if existing {
				want = 2
			}
			if events != want {
				t.Fatalf("events = %d, want %d", events, want)
			}
		})
	}
}
