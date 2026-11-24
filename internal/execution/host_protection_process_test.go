package execution

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type protectionProcessFixture struct{ Directory, Root, Journal, Mode string }

func TestHostProtectionProcessHelper(t *testing.T) {
	data := os.Getenv("OFFGRID_HOST_PROTECTION_TEST_FIXTURE")
	if data == "" {
		return
	}
	var fixture protectionProcessFixture
	if json.Unmarshal([]byte(data), &fixture) != nil {
		os.Exit(2)
	}
	registry := &HostProtectionRegistry{directory: fixture.Directory}
	fmt.Println("ready")
	if !bufio.NewScanner(os.Stdin).Scan() {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	for {
		if fixture.Mode == "browser" {
			_, err = registry.RegisterBrowserRoot(ctx, fixture.Root)
		} else {
			_, err = registry.ReserveExecution(ctx, protectionTestConfig(fixture.Root, "process-owned-lease"), fixture.Journal)
		}
		if !errors.Is(err, ErrHostProtectionBusy) {
			break
		}
		select {
		case <-ctx.Done():
			os.Exit(3)
		case <-time.After(5 * time.Millisecond):
		}
	}
	result := "ok"
	if errors.Is(err, ErrHostProtectionConflict) {
		result = "conflict"
	} else if err != nil {
		result = "error: " + err.Error()
	}
	fmt.Println(result)
	// Deliberately leave any reservation unsettled. Process exit/lock release
	// must not turn an execution row into cleanup evidence.
	os.Exit(0)
}

func TestHostProtectionCrossProcessAtomicAdmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	registry := protectionTestRegistry(t)
	workspace := protectionTestRoot(t)
	browser := protectionTestDirectory(t, workspace, "browser")
	journal := protectionTestRoot(t)
	fixtures := []protectionProcessFixture{{registry.directory, browser, journal, "browser"}, {registry.directory, workspace, journal, "execution"}}
	type child struct {
		cmd        *exec.Cmd
		scanner    *bufio.Scanner
		start      func()
		diagnostic *bytes.Buffer
	}
	var children []child
	for _, fixture := range fixtures {
		data, _ := json.Marshal(fixture)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostProtectionProcessHelper$")
		cmd.Env = append(os.Environ(), "OFFGRID_HOST_PROTECTION_TEST_FIXTURE="+string(data))
		input, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		diagnostic := new(bytes.Buffer)
		cmd.Stderr = diagnostic
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = input.Close()
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
		scanner := bufio.NewScanner(output)
		if !scanner.Scan() || scanner.Text() != "ready" {
			t.Fatalf("fixture did not become ready: %s", diagnostic)
		}
		children = append(children, child{cmd, scanner, func() { _, _ = fmt.Fprintln(input, "admit"); _ = input.Close() }, diagnostic})
	}
	for _, child := range children {
		child.start()
	}
	results := map[string]int{}
	for _, child := range children {
		if !child.scanner.Scan() {
			t.Fatalf("fixture response absent: %s", child.diagnostic)
		}
		results[child.scanner.Text()]++
		if err := child.cmd.Wait(); err != nil {
			t.Fatalf("fixture failed: %v %s", err, child.diagnostic)
		}
	}
	if results["ok"] != 1 || results["conflict"] != 1 {
		t.Fatalf("admission was not atomic: %v", results)
	}
	if err := registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		if len(store.records) != 1 {
			t.Fatalf("unexpected durable rows: %d", len(store.records))
		}
		for _, row := range store.records {
			if row.Kind == "execution" && row.State != "reserved" {
				t.Fatal("process exit released execution", row.State)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Whichever side won, a later conflicting execution grant remains fenced.
	if _, err := registry.ReserveExecution(ctx, protectionTestConfig(browser, "later"), journal); err == nil {
		// Execution-vs-execution may coexist; this branch identifies the winner
		// and checks its still-reserved row against browser admission instead.
		if _, err = registry.RegisterBrowserRoot(ctx, browser); !errors.Is(err, ErrHostProtectionConflict) {
			t.Fatalf("crash row no longer excludes browser: %v", err)
		}
	} else if !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(registry.directory, "protection.sqlite")); err != nil {
		t.Fatal(err)
	}
}
