package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"sync"
	"testing"
)

func TestAudioEngineIsWorkspaceScopedAndStable(t *testing.T) {
	a := &Server{config: &config.Config{DataDir: t.TempDir(), ModelsDir: t.TempDir()}}
	b := &Server{config: &config.Config{DataDir: t.TempDir(), ModelsDir: t.TempDir()}}
	first, err := a.getAudioEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	other, err := b.getAudioEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if first == other {
		t.Fatal("different workspaces share speech engine")
	}
	var wg sync.WaitGroup
	results := make(chan *audio.Engine, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, err := a.getAudioEngine()
			if err != nil {
				t.Error(err)
			}
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != first {
			t.Fatal("polling replaces speech engine")
		}
	}
}
