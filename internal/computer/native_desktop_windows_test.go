//go:build windows && amd64

package computer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNativeWindowsDesktopOwnedApplicationLaunch(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_NATIVE_WINDOWS") != "1" {
		t.Skip("opt-in owned application launch")
	}
	if err := NativeWindowsTestEnvironment(); err != nil {
		t.Skipf("native environment unavailable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "owned-lifecycle.exe")
	build := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", path, "./testdata/nativeapp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver, err := NewNativeWindows()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	c, err := NewDesktopController(driver)
	if err != nil {
		t.Fatal(err)
	}
	c.apps = map[string]installedApplication{"fixture": {NativeApplication: NativeApplication{ID: "fixture", Name: "Owned fixture"}, path: path, identity: info}}
	journal, err := OpenNativeJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	consent, step := desktopConsentFixture()
	consent.Binding.Target = c.Identity()
	consent.Desktop.Binding = consent.Binding
	step.Binding = consent.Binding
	action, err := c.Prepare(Operation{Kind: "open_application", Application: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	step.Actions = []PreparedControlAction{action}
	supervisor := NewNativeSupervisor(journal, consent, c.Dispatch)
	defer supervisor.Stop()
	grant, err := supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	result, err := supervisor.Execute(step, grant.ID)
	if err != nil || len(result) != 1 || result[0].Outcome != "dispatched" {
		t.Fatal(result, err)
	}
	evidence, err := c.VerifyApplication(context.Background(), "fixture")
	if err != nil || !evidence.Verified {
		t.Fatal(evidence, err)
	}
	surface := driver.targets[evidence.Target.ID]
	defer func() {
		// Address only the exact surface and process generation created by our
		// temporary executable; never terminate by name or title.
		generation, _, err := processGeneration(surface.pid)
		if err == nil && generation == surface.generation {
			user32.NewProc("PostMessageW").Call(surface.hwnd, 0x0010, 0, 0)
		}
		for i := 0; i < 50; i++ {
			if _, _, err := processGeneration(surface.pid); err != nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	again, err := c.VerifyApplication(context.Background(), "fixture")
	if err != nil || !again.Verified || again.Target.ProcessGeneration != evidence.Target.ProcessGeneration {
		t.Fatal("existing process not reused", err)
	}
	grant, err = supervisor.Approve(step, func(BoundedStep) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = supervisor.Execute(step, grant.ID); err != nil {
		t.Fatal("recorded result not recovered", err)
	}
	surfaces, err := c.Surfaces()
	if err != nil || len(surfaces) != 1 {
		t.Fatal("launch was duplicated", len(surfaces), err)
	}
	t.Log("real catalog executable launch, UIA readiness, process reuse and journaled replay passed")
}
