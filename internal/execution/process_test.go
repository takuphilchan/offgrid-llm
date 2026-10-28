package execution

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testCommand(script string) ProcessSpec {
	if runtime.GOOS == "windows" {
		return ProcessSpec{Program: os.Getenv("COMSPEC"), Arguments: []string{"/D", "/S", "/C", script}}
	}
	return ProcessSpec{Program: "/bin/sh", Arguments: []string{"-c", script}}
}

func TestProcessFixtureHelper(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "offgrid-stream-fixture" {
			fmt.Println("early streamed output")
			time.Sleep(30 * time.Second)
			os.Exit(0)
		}
	}
}

func TestProcessOutputArrivesBeforeCompletionAndCanBePaged(t *testing.T) {
	s, b, _ := fixture(t)
	s.config.Permissions.Processes = true
	m := NewProcessManager(s)
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := m.Start(ctx, b, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessFixtureHelper$", "--", "offgrid-stream-fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	var first ProcessPage
	for deadline := time.Now().Add(5 * time.Second); ; {
		first, err = m.Output(ctx, b, p.Handle, 0, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Output) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no live process output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if first.State != "running" || first.Complete || first.NextOffset == nil {
		t.Fatal(first)
	}
	next, err := m.Output(ctx, b, p.Handle, *first.NextOffset, MaxPageBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(append(first.Output, next.Output...)), "early streamed output") {
		t.Fatal(first, next)
	}
	if _, err = m.Stop(ctx, b, p.Handle); err != nil {
		t.Fatal(err)
	}
}

func TestProcessDeadlineIsReportedTruthfully(t *testing.T) {
	s, b, _ := fixture(t)
	s.config.Permissions.Processes = true
	m := NewProcessManager(s)
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := m.Start(ctx, b, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessFixtureHelper$", "--", "offgrid-stream-fixture"}, TimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Wait(ctx, b, p.Handle)
	if err != nil || result.State != "timed_out" {
		t.Fatal(result, err)
	}
}

func TestProcessExitOutputAndJobOwnership(t *testing.T) {
	s, b, _ := fixture(t)
	s.config.Permissions.Processes = true
	m := NewProcessManager(s)
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := m.Start(ctx, b, testCommand("echo verified-output"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Wait(ctx, b, p.Handle)
	if err != nil || r.State != "exited" || r.ExitCode == nil || *r.ExitCode != 0 || !strings.Contains(string(r.Output), "verified-output") {
		t.Fatal(r, err)
	}
	p, err = m.Start(ctx, b, testCommand("exit 7"))
	if err != nil {
		t.Fatal(err)
	}
	r, err = m.Wait(ctx, b, p.Handle)
	if err != nil || r.State != "failed" || r.ExitCode == nil || *r.ExitCode != 7 {
		t.Fatal(r, err)
	}
	wrong := b
	wrong.Job = "another-job"
	if _, err = m.Inspect(ctx, wrong, p.Handle); err == nil {
		t.Fatal("cross-job process read")
	}
}

func TestProcessCancellationAndRevocation(t *testing.T) {
	for _, revocation := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "revoke"}[revocation], func(t *testing.T) {
			s, b, _ := fixture(t)
			s.config.Permissions.Processes = true
			m := NewProcessManager(s)
			defer m.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			script := "sleep 30"
			if runtime.GOOS == "windows" {
				script = "ping -n 30 127.0.0.1 >NUL"
			}
			p, err := m.Start(ctx, b, testCommand(script))
			if err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			owned := m.processes[p.Handle]
			m.mu.Unlock()
			start := time.Now()
			if revocation {
				s.Revoke()
			} else {
				cancel()
			}
			select {
			case <-owned.done:
			case <-time.After(5 * time.Second):
				t.Fatal("owned process tree did not stop")
			}
			if time.Since(start) > 3*time.Second {
				t.Fatal("slow local stop")
			}
			if owned.snapshot().State != "cancelled" {
				t.Fatal(owned.snapshot())
			}
		})
	}
}

func TestProcessOutputBoundAndCredentialIsolation(t *testing.T) {
	var output processOutput
	payload := make([]byte, MaxProcessOutput+100)
	if n, err := output.Write(payload); n != len(payload) || err != nil {
		t.Fatal(n, err)
	}
	data, truncated := output.snapshot()
	if len(data) != MaxProcessOutput || !truncated {
		t.Fatal(len(data), truncated)
	}
	t.Setenv("OFFGRID_API_KEY", "fixture-do-not-forward")
	t.Setenv("OFFGRID_EXECUTOR_TOKEN", strings.Repeat("a", 64))
	for _, env := range processEnvironment() {
		if strings.HasPrefix(env, "OFFGRID_") {
			t.Fatal("forwarded supervisor credential")
		}
	}
}
