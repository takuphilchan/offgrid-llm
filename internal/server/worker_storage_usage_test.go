package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestWorkerStorageUsageDoesNotExposeGrantOrCountReleasedFiles(t *testing.T) {
	task := &agents.Task{Checkpoint: &agents.Checkpoint{StorageBudget: &agents.StorageBudget{Version: 1, LimitBytes: 1000, ChargedBytes: 300, Reservations: map[string]agents.StorageReservation{
		"private-copy-one": {State: "ready", ChargedBytes: 200, UsageMeasured: true, MeasuredBytes: 50, Request: execution.WorkingCopyRequest{Parent: execution.Binding{Grant: "private-grant"}}},
		"private-copy-two": {State: "reserved", ChargedBytes: 100},
		"private-copy-old": {State: "released", MeasuredBytes: 80, UsageMeasured: true},
	}}}}
	usage := taskResourceUsage(task)
	if usage.Storage == nil || usage.Storage.ChargedBytes != 300 || usage.Storage.MeasuredBytes != 50 || usage.Storage.RetainedCopies != 2 || usage.Storage.UnmeasuredCopies != 1 {
		t.Fatal(usage)
	}
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-") {
		t.Fatal("private storage binding exposed", string(data))
	}
}
