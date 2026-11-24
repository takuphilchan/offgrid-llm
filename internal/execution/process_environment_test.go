package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProcessEnvironmentFixture(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "offgrid-environment-fixture" {
		return
	}
	fmt.Printf("LANG=%s;TOKEN=%s", os.Getenv("LANG"), os.Getenv("OFFGRID_EXECUTOR_TOKEN"))
	os.Exit(0)
}

func TestProcessUsesSessionEnvironmentAfterAmbientChange(t *testing.T) {
	t.Setenv("LANG", "fixture-original")
	t.Setenv("OFFGRID_EXECUTOR_TOKEN", "private-never-forward")
	s, binding, _ := fixture(t)
	s.config.Permissions.Processes = true
	before := s.environment.descriptor()
	t.Setenv("LANG", "fixture-changed")
	manager := NewProcessManager(s)
	defer manager.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started, err := manager.Start(ctx, binding, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessEnvironmentFixture$", "--", "offgrid-environment-fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Wait(ctx, binding, started.Handle)
	if err != nil || result.ExitCode == nil || *result.ExitCode != 0 || string(result.Output) != "LANG=fixture-original;TOKEN=" {
		t.Fatal(result, err)
	}
	if !reflect.DeepEqual(before, s.environment.descriptor()) {
		t.Fatal("ambient change replaced session template")
	}
}

func TestWorkingCopyInheritsExactEnvironmentWithoutSharingStorage(t *testing.T) {
	t.Setenv("LANG", "original-parent-environment")
	endpoint, store, binding, _ := workingCopyFixture(t)
	parent := endpoint.Worker.Session.environment.descriptor()
	t.Setenv("LANG", "new-ambient-environment")
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "environment"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Inspect(context.Background(), child.Record.ID, binding)
	if err != nil || record.Environment == nil || !reflect.DeepEqual(*record.Environment, parent) || !reflect.DeepEqual(child.Endpoint.Worker.Session.environment.descriptor(), parent) {
		t.Fatal("worker did not pin parent template", record.Environment, err)
	}
	raw, _ := json.Marshal(record.Environment)
	if strings.Contains(string(raw), "original-parent-environment") || strings.Contains(string(raw), "new-ambient-environment") {
		t.Fatal("descriptor disclosed environment values")
	}
	child.Endpoint.Worker.Session.environment.values[0] = "fixture-child-only=value"
	if !reflect.DeepEqual(endpoint.Worker.Session.environment.descriptor(), parent) {
		t.Fatal("child changed parent environment storage")
	}
}

func TestProcessResolutionUsesPinnedPathAndExtensions(t *testing.T) {
	approved, changed, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	name := "fixture-command"
	if runtime.GOOS == "windows" {
		name += ".EXE"
	}
	for _, directory := range []string{approved, changed, cwd} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("resolution-only executable"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", approved)
	t.Setenv("PATHEXT", ".EXE")
	environment := captureProcessEnvironment()
	t.Setenv("PATH", changed)
	t.Setenv("PATHEXT", ".COM")
	selected, err := environment.program("fixture-command", cwd)
	if err != nil || selected != filepath.Join(approved, name) {
		t.Fatal("ambient lookup selected a different executable", selected, err)
	}
	withoutPath := processEnvironmentSnapshot{values: []string{"PATH=.", "PATHEXT=.EXE"}}
	if selected, err := withoutPath.program("fixture-command", cwd); err == nil || selected != "" {
		t.Fatal("implicitly selected cwd executable", selected, err)
	}
	selected, err = withoutPath.program("./"+name, cwd)
	if err != nil || selected != filepath.Join(cwd, name) {
		t.Fatal("explicit relative invocation lost cwd", selected, err)
	}
}
