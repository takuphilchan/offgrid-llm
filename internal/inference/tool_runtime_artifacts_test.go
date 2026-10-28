//go:build !llama

package inference

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func ownedArtifactFixture(t *testing.T) (*LlamaCPPProcess, *ModelCache, ToolRuntimeArtifacts) {
	t.Helper()
	p, cache := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"build_info":"fixture","chat_template":"template","default_generation_settings":{"n_ctx":8192}}`)
	})
	open := func(name, content string) *os.File {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { file.Close() })
		return file
	}
	model := open("model.gguf", "fixture model bytes")
	dependency := open("dependency", "runtime dependency")
	instance := cache.instances["selected"]
	instance.ModelPath, instance.Cmd.Args[2] = model.Name(), model.Name()
	executable, err := os.Open(instance.Cmd.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { executable.Close() })
	// The fake owned child and the service are this test process. Production
	// supplies their distinct handles; role identity remains part of the digest.
	return p, cache, ToolRuntimeArtifacts{Model: model, Runtime: map[string]*os.File{"executable": executable, "offgrid-service": executable, "dependency": dependency}}
}

func TestOwnedRuntimeArtifactMeasurementBindsFilesAndContent(t *testing.T) {
	p, _, files := ownedArtifactFixture(t)
	first, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files)
	if err != nil || len(first.ModelSHA256) != 64 || len(first.RuntimeSHA256) != 64 || first.Observation.Generation != 7 {
		t.Fatal(first, err)
	}
	// Equal-length, restored-mtime writes still change content identity.
	stat, _ := files.Model.Stat()
	if err := os.WriteFile(files.Model.Name(), []byte("changed model bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(files.Model.Name(), stat.ModTime(), stat.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files)
	if err != nil || first.ModelSHA256 == second.ModelSHA256 || first.RuntimeSHA256 != second.RuntimeSHA256 {
		t.Fatal("model content was not measured", second, err)
	}
	if err := os.WriteFile(files.Runtime["dependency"].Name(), []byte("changed dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	third, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files)
	if err != nil || third.RuntimeSHA256 == second.RuntimeSHA256 {
		t.Fatal("dependency content was not measured", third, err)
	}
}

func TestOwnedRuntimeArtifactMeasurementRejectsForeignAndCancelled(t *testing.T) {
	p, _, original := ownedArtifactFixture(t)
	for _, change := range []func(*ToolRuntimeArtifacts){
		func(f *ToolRuntimeArtifacts) { f.Model = f.Runtime["dependency"] },
		func(f *ToolRuntimeArtifacts) { f.Projector = f.Model },
		func(f *ToolRuntimeArtifacts) { f.Runtime = map[string]*os.File{"executable": f.Model} },
		func(f *ToolRuntimeArtifacts) {
			f.Runtime = map[string]*os.File{"executable": original.Runtime["executable"], "offgrid-service": f.Model}
		},
		func(f *ToolRuntimeArtifacts) {
			f.Runtime = map[string]*os.File{"executable": original.Runtime["executable"]}
		},
		func(f *ToolRuntimeArtifacts) { f.Runtime = nil },
	} {
		files := original
		change(&files)
		if _, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files); err == nil {
			t.Fatal("foreign/missing artifact accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.MeasureToolRuntimeArtifacts(ctx, "selected", 8192, original); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

func TestOwnedRuntimeArtifactMeasurementRejectsGenerationChange(t *testing.T) {
	p, cache, files := ownedArtifactFixture(t)
	// Replace just the metadata endpoint with a fixture that changes generation
	// on the second observation; no real process is restarted or controlled.
	var requests atomic.Int32
	other, endpointCache := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 2 {
			cache.mu.Lock()
			cache.instances["selected"].Generation++
			cache.mu.Unlock()
		}
		fmt.Fprint(w, `{"build_info":"fixture","chat_template":"template","default_generation_settings":{"n_ctx":8192}}`)
	})
	_ = other
	instance := cache.instances["selected"]
	instance.Port = endpointCache.instances["selected"].Port
	instance.Cmd.Args[4] = endpointCache.instances["selected"].Cmd.Args[4]
	if _, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files); err == nil {
		t.Fatal("changed process generation accepted")
	}
}

func TestOwnedRuntimeArtifactRejectsCopiedServiceImage(t *testing.T) {
	p, _, files := ownedArtifactFixture(t)
	copy, err := os.Create(filepath.Join(t.TempDir(), "service-copy"))
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	service := files.Runtime["offgrid-service"]
	info, err := service.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(copy, io.NewSectionReader(service, 0, info.Size())); err != nil {
		t.Fatal(err)
	}
	// A byte-identical file is not proof of the process doing this evaluation.
	// A changed installation path must not replace the running image's identity.
	files.Runtime["offgrid-service"] = copy
	if _, err := p.MeasureToolRuntimeArtifacts(context.Background(), "selected", 8192, files); err == nil {
		t.Fatal("copied service executable substituted for running image")
	}
}
