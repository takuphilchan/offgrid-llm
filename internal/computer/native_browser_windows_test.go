//go:build windows && amd64

package computer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Opt-in, disposable browser profile and loopback fixture only. Never attaches
// to an existing personal Chrome profile or visits a public website.
func TestNativeOwnedBrowserSearch(t *testing.T) {
	binary := os.Getenv("OFFGRID_NATIVE_BROWSER_FIXTURE")
	if binary == "" {
		t.Skip("requires explicit owned browser fixture executable")
	}
	if err := NativeWindowsTestEnvironment(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	searched := make(chan string, 1)
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query := r.URL.Query().Get("q"); query != "" {
			select {
			case searched <- query:
			default:
			}
		}
		if r.URL.Path == "/ready" {
			once.Do(func() { close(ready) })
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>OffGrid owned browser fixture</title><h1>Public research</h1><form><label>Search tourism<input aria-label="Search tourism" name="q"></label><button type="submit">Search fixture</button></form><script>fetch('/ready')</script>`)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--user-data-dir="+t.TempDir(), "--no-first-run", "--no-default-browser-check", "--disable-background-networking", "--disable-sync", "--disable-extensions", "--proxy-server=direct://", "--app="+server.URL)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
		if t.Failed() {
			t.Logf("owned browser diagnostics: %s", diagnostics.String())
		}
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("fixture page never loaded")
	case err := <-exited:
		t.Fatalf("owned browser exited: %v %s", err, diagnostics.String())
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver, err := NewNativeWindows()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for ctx.Err() == nil {
		select {
		case err := <-exited:
			t.Fatalf("owned browser exited: %v %s", err, diagnostics.String())
		default:
		}
		targets, err := driver.Targets()
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range targets {
			if !strings.HasPrefix(target.Identity.ProcessGeneration, fmt.Sprintf("%d:", cmd.Process.Pid)) {
				continue
			}
			t.Logf("owned browser surface: %s", target.Title)
			if _, err = driver.Select(target.Identity.ID); err != nil {
				t.Fatal(err)
			}
			view, err := driver.Observe(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("ready observation: %d elements", len(view.Elements))
			for _, e := range view.Elements {
				if e.Name == "Search tourism" && e.Writable {
					step, _, consent, _ := controlFixture()
					step.Binding.Target = target.Identity
					consent.Binding = step.Binding
					consent.IssuedAt, consent.ExpiresAt = time.Now().Add(-time.Second), time.Now().Add(time.Minute)
					journal, err := OpenNativeJournal(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					defer journal.Close()
					supervisor := NewNativeSupervisor(journal, consent, driver.Dispatch)
					defer supervisor.Stop()
					execute := func(op Operation, current NativeView) {
						t.Helper()
						action, err := driver.Prepare(step.Binding, op, current.Observation)
						if err != nil {
							t.Fatal("prepare", err)
						}
						step.Actions = []PreparedControlAction{action}
						grant, err := supervisor.Approve(step, func(proposed BoundedStep) bool { return proposed.Binding.Target == target.Identity })
						if err != nil {
							t.Fatal("approve", err)
						}
						if err := driver.FocusSelected(); err != nil {
							t.Fatal("focus", err)
						}
						results, err := supervisor.Execute(step, grant.ID)
						if err != nil || len(results) != 1 || results[0].Uncertain {
							t.Fatal("dispatch", results, err)
						}
					}
					query := "tourism research 2026"
					execute(Operation{Kind: "replace_text", Element: e.ID, Text: &query}, view)
					fresh, err := driver.Observe(ctx)
					if err != nil {
						t.Fatal(err)
					}
					for _, control := range fresh.Elements {
						if control.Name == "Search fixture" && control.Invokable {
							execute(Operation{Kind: "activate", Element: control.ID}, fresh)
							select {
							case got := <-searched:
								if got != query {
									t.Fatalf("independent HTTP oracle: %q", got)
								}
							case <-ctx.Done():
								t.Fatal("search was not received by the fixture")
							}
							return
						}
					}
					t.Fatal("search submit control unavailable")
					return
				}
			}
			t.Fatalf("browser controls not ready: %d elements", len(view.Elements))
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("owned browser surface unavailable", ctx.Err())
}
