//go:build linux && cgo && offgrid_native

package computer

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNativeLinuxRealATSPI(t *testing.T) {
	fixture := os.Getenv("OFFGRID_NATIVE_LINUX_FIXTURE")
	if fixture == "" {
		t.Skip("isolated GTK fixture not selected")
	}
	child := exec.Command(fixture)
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	lines := make(chan string, 4)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	read := func() string {
		select {
		case line := <-lines:
			return line
		case <-time.After(10 * time.Second):
			t.Fatal("owned fixture response timed out")
			return ""
		}
	}
	if read() != "READY" {
		t.Fatal("fixture not ready")
	}
	driver, err := NewNativeLinux()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var target NativeTarget
	for attempt := 0; attempt < 30; attempt++ {
		targets, err := driver.Targets()
		if err != nil {
			t.Fatal(err)
		}
		for _, candidate := range targets {
			if candidate.Title == "OffGrid AT-SPI fixture" && strings.HasPrefix(candidate.Identity.ProcessGeneration, fmtPID(child.Process.Pid)) {
				target = candidate
				break
			}
		}
		if target.Identity.ID != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if target.Identity.ID == "" {
		t.Fatal("owned AT-SPI target not found")
	}
	if _, err := driver.Select(target.Identity.ID); err != nil {
		t.Fatal(err)
	}
	view, err := driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	field := ""
	for _, entry := range view.Elements {
		if strings.Contains(entry.Name, "SYNTHETIC_TEST_SECRET") {
			t.Fatal("password exposed")
		}
		if entry.Writable && entry.Name == "Report title" {
			field = entry.ID
		}
	}
	if field == "" {
		t.Fatal("editable AT-SPI field missing")
	}
	step, _, consent, _ := controlFixture()
	step.Binding.Target = target.Identity
	consent.Binding = step.Binding
	consent.IssuedAt = time.Now().Add(-time.Second)
	consent.ExpiresAt = time.Now().Add(time.Minute)
	text := "Zimbabwe research — final (2026)"
	action, err := driver.Prepare(step.Binding, Operation{Kind: "replace_text", Element: field, Text: &text}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	step.Actions = []PreparedControlAction{action}
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	supervisor := NewNativeSupervisor(journal, consent, driver.Dispatch)
	defer supervisor.Stop()
	grant, err := supervisor.Approve(step, func(step BoundedStep) bool {
		return step.Binding.Target == target.Identity && *step.Actions[0].Operation.Text == text
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.FocusSelected(); err != nil {
		t.Fatal(err)
	}
	results, err := supervisor.Execute(step, grant.ID)
	if err != nil || len(results) != 1 || results[0].Outcome != "verified" {
		t.Fatal(results, err)
	}
	if _, err := in.Write([]byte("read\n")); err != nil {
		t.Fatal(err)
	}
	if read() != "VALUE "+text {
		t.Fatal("independent GTK oracle mismatch")
	}
	view, err = driver.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range view.Elements {
		if entry.Writable && entry.Name == "Report title" {
			field = entry.ID
		}
	}
	action, err = driver.Prepare(step.Binding, Operation{Kind: "replace_text", Element: field, Text: &text}, view.Observation)
	if err != nil {
		t.Fatal(err)
	}
	in.Write([]byte("manual\n"))
	if read() != "MANUAL" {
		t.Fatal("manual edit did not settle")
	}
	if _, err := driver.Dispatch(context.Background(), action); err != ErrStaleObservation {
		t.Fatalf("stale AT-SPI control accepted: %v", err)
	}
	t.Log("real AT-SPI target, approved Unicode edit, independent GTK result and stale-action rejection passed")
}

func fmtPID(pid int) string { return strconv.Itoa(pid) + ":" }
