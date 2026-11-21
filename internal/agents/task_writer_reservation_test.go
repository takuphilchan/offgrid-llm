package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTaskPersistenceWritesSnapshotOnlyOnce(t *testing.T) {
	directory := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, directory)
	task, err := manager.CreateTask("write-count", strings.Repeat("fixture context ", 16384), nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := openTaskDatabase(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE fixture_task_updates (count INTEGER NOT NULL); INSERT INTO fixture_task_updates VALUES(0); CREATE TRIGGER fixture_count_task_updates AFTER UPDATE ON agent_tasks BEGIN UPDATE fixture_task_updates SET count=count+1; END`); err != nil {
		t.Fatal(err)
	}
	task.Prompt += " changed"
	if err := persistTask(directory, task); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count FROM fixture_task_updates`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("snapshot update rewrote the task row %d times: %v", count, err)
	}
}

func BenchmarkPersistTaskLargeSnapshot(b *testing.B) {
	directory := b.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, directory)
	task, err := manager.CreateTask("large-snapshot", strings.Repeat("fixture context ", 16384), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(task.Prompt)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.InstructionVersion++
		if err := persistTask(directory, task); err != nil {
			b.Fatal(err)
		}
	}
}

func TestTaskWriterReservationExcludesCompetingWriter(t *testing.T) {
	for _, populated := range []bool{false, true} {
		for _, commit := range []bool{false, true} {
			t.Run(fmt.Sprintf("populated=%t/commit=%t", populated, commit), func(t *testing.T) {
				directory := t.TempDir()
				manager := NewManagerWithPersistence(nil, nil, nil, directory)
				if populated {
					if _, err := manager.CreateTask("reservation", "fixture", nil); err != nil {
						t.Fatal(err)
					}
				}
				db, err := openTaskDatabase(directory)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				competitor, err := db.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer competitor.Close()
				if _, err := competitor.ExecContext(ctx, `PRAGMA busy_timeout=0`); err != nil {
					t.Fatal(err)
				}
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				if err := reserveTaskWriter(tx); err != nil {
					t.Fatal(err)
				}
				// A separate SQLite connection can still read, but must not write
				// between reservation and the owner's first snapshot read.
				var count int
				if err := competitor.QueryRowContext(ctx, `SELECT count(*) FROM agent_tasks`).Scan(&count); err != nil {
					t.Fatalf("reservation blocked a WAL reader: %v", err)
				}
				_, err = competitor.ExecContext(ctx, `UPDATE agent_tasks SET actor=actor WHERE 0`)
				var sqliteError interface{ Code() int }
				if !errors.As(err, &sqliteError) || sqliteError.Code() != 5 {
					t.Fatalf("competing writer must get SQLITE_BUSY before snapshot read: %v", err)
				}
				if commit {
					err = tx.Commit()
				} else {
					err = tx.Rollback()
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := competitor.ExecContext(ctx, `UPDATE agent_tasks SET actor=actor WHERE 0`); err != nil {
					t.Fatalf("reservation survived transaction end: %v", err)
				}
			})
		}
	}
}
