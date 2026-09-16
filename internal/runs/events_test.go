package runs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLogPersistsSequencesAndReplays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	log, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		event, err := NewEvent("run-1", ModelDelta, map[string]int{"part": i})
		if err != nil {
			t.Fatal(err)
		}
		if err := log.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	events, err := log.Replay("run-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Sequence != 2 {
		t.Fatalf("unexpected replay: %#v", events)
	}
}

func TestSlowSubscriberDoesNotBlockExecution(t *testing.T) {
	log, err := NewLog(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	events, cancel := log.Subscribe(1)
	defer cancel()
	for i := 0; i < 5; i++ {
		event, _ := NewEvent("test", ModelDelta, map[string]int{"part": i})
		if err := log.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	for range events {
		count++
	}
	if count != 1 {
		t.Fatalf("unexpected queued events %d", count)
	}
	replay, err := log.Replay("test", 1)
	if err != nil || len(replay) != 4 {
		t.Fatalf("lost durable events: %v %v", replay, err)
	}
}

func TestReplayRejectsInvalidSequence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(`{"id":"a","run_id":"test","sequence":2,"type":"model.delta","time":"2026-09-16T00:00:00Z"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLog(path); err == nil {
		t.Fatal("non-contiguous log accepted")
	}
}

func TestStorageFailureStopsFurtherAppends(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	log, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	event, _ := NewEvent("test", RunStarted, nil)
	if err := log.Publish(context.Background(), event); err == nil {
		t.Fatal("write failure not reported")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := log.Publish(context.Background(), event); err == nil {
		t.Fatal("appended after uncertain storage failure")
	}
}
