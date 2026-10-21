package server

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestNativeDocumentTaskIntentRoutesOnlyDocumentRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt string
		want   bool
	}{
		{name: "browser search", prompt: "Open Microsoft Edge and search for exactly climate news.", want: false},
		{name: "native document", prompt: "Open Notepad, create a blank document and type exactly hello.", want: true},
		{name: "writer", prompt: "Open LibreOffice Writer and edit the document.", want: true},
		{name: "empty internal stage", prompt: "", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &agents.Task{Prompt: tc.prompt}
			if got := nativeDocumentTaskIntent(task); got != tc.want {
				t.Fatalf("intent=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestNativeDocumentObservationRecoveryRequiresFreshObservation(t *testing.T) {
	task := &agents.Task{Checkpoint: &agents.Checkpoint{NativeReadRepair: &agents.NativeReadRepair{Tool: "computer_documents"}}}
	if !nativeDocumentObservationRecoveryPending(task) {
		t.Fatal("document read repair was not detected")
	}
	for _, name := range []string{"computer_documents", "computer_read_text", "computer_replace_text", "computer_insert_text"} {
		if nativeDocumentRecoveryToolAllowed(name) {
			t.Fatalf("unsafe recovery tool allowed: %s", name)
		}
	}
	if !nativeDocumentRecoveryToolAllowed("computer_observe") {
		t.Fatal("fresh observation was not allowed")
	}
}
