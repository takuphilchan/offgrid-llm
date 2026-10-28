package agents

import (
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func TestProfileSaveDoesNotMutateSharedInputSlicesOrPointers(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	settings := ProfileSettings{Name: "Shared input", Model: "model", MaxIterations: 10, Knowledge: []string{"document"}, Execution: &execution.Permissions{Read: true}}
	var wg sync.WaitGroup
	profiles := make(chan *AgentProfile, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := m.SaveProfile("work", "alice", "", 0, settings)
			if err != nil {
				errs <- err
			} else {
				profiles <- p
			}
		}()
	}
	wg.Wait()
	close(profiles)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for p := range profiles {
		p.Settings.Knowledge[0] = "changed"
		p.Settings.Execution.Read = false
	}
	if settings.Knowledge[0] != "document" || !settings.Execution.Read {
		t.Fatal("caller data aliased saved profile")
	}
}
