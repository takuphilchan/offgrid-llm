package runs

import (
	"context"
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
